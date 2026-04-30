package importdata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"website-exam/internal/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func SaveImport(ctx context.Context, db *pgxpool.Pool, result *ParseUploadResult, uploadedByAccount string) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	parseStatus := parseStatusFor(result)
	rawContent := result.Extract.Text
	if rawContent == "" {
		rawContent = result.Extract.Warning
	}

	var uploadedByUserID any
	uploadedByAccount = strings.TrimSpace(uploadedByAccount)
	if uploadedByAccount == "" {
		return fmt.Errorf("missing teacher account for import")
	}
	var id int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, uploadedByAccount).Scan(&id); err != nil {
		return fmt.Errorf("teacher account not found for import: %s", uploadedByAccount)
	}
	uploadedByUserID = id

	var batchID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO import_batches (
			uploaded_by_user_id,
			source_name,
			original_filename,
			source_type,
			file_size_bytes,
			file_sha256,
			content_fingerprint,
			raw_content,
			parser_model,
			parse_status,
			document_title,
			image_count,
			asset_summary_json,
			parse_error
		)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''), $8, $9, $10, $11, $12, $13::jsonb, NULLIF($14, ''))
		RETURNING id
	`, uploadedByUserID, result.File.Name, result.File.Name, sourceTypeFor(result.File.Kind), result.File.Size, result.FileSHA256, ContentFingerprint(result.Extract.Text), rawContent, "local-rule-parser-v1", parseStatus, result.Extract.DocumentTitle, result.Extract.ImageCount, assetSummaryJSON(result), result.Extract.Warning).Scan(&batchID); err != nil {
		return err
	}
	if len(result.RawFile) > 0 {
		sourcePath, err := saveOriginalUpload(batchID, result.File.Name, result.RawFile)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE import_batches SET source_file_url = $1 WHERE id = $2`, sourcePath, batchID); err != nil {
			return err
		}
	}
	if len(result.Assets) > 0 {
		if err := saveExtractedAssets(ctx, tx, batchID, result.Assets); err != nil {
			return err
		}
	}

	for index, question := range result.Questions {
		payload, err := json.Marshal(question)
		if err != nil {
			return err
		}
		reviewStatus := "pending"
		if question.Status == "fail" {
			reviewStatus = "rejected"
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO import_items (
				batch_id,
				item_order,
				source_order,
				raw_question_text,
				parsed_question_json,
				normalized_question_json,
				ai_confidence,
				review_status,
				review_note
			)
			VALUES ($1, $2, $3, $4, $5::jsonb, $5::jsonb, $6, $7, $8)
			RETURNING id
		`, batchID, index+1, question.SourceOrder, rawQuestionText(question), string(payload), float64(question.Confidence)/100, reviewStatus, strings.Join(question.Warnings, "\n")).Scan(&result.Questions[index].ImportItemID); err != nil {
			return err
		}
	}
	candidates, err := duplicateCandidates(ctx, tx, result, uploadedByUserID)
	if err != nil {
		return err
	}
	result.DuplicateCandidates = candidates

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	result.ImportBatchID = batchID
	return nil
}

type duplicateBatchStats struct {
	batchID       int64
	title         string
	sourceName    string
	createdAt     time.Time
	existingKeys  map[string]bool
	matchingCount int
}

func duplicateCandidates(ctx context.Context, tx pgx.Tx, result *ParseUploadResult, uploadedByUserID any) ([]ImportDuplicateCandidate, error) {
	incomingKeys := map[string]bool{}
	for _, question := range result.Questions {
		if question.Status == "fail" {
			continue
		}
		key := QuestionContentKey(question.Content)
		if key != "" {
			incomingKeys[key] = true
		}
	}
	if len(incomingKeys) == 0 || uploadedByUserID == nil {
		return nil, nil
	}

	rows, err := tx.Query(ctx, `
		SELECT ib.id,
			COALESCE(NULLIF(ib.original_filename, ''), NULLIF(ib.source_name, ''), 'Bo de #' || ib.id::text),
			COALESCE(NULLIF(ib.source_name, ''), NULLIF(ib.original_filename, ''), ''),
			ib.created_at,
			qb.content
		FROM import_batches ib
		JOIN import_items ii ON ii.batch_id = ib.id
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ib.uploaded_by_user_id = $1
		ORDER BY ib.id DESC, ii.item_order, qb.id
	`, uploadedByUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	statsByBatch := map[int64]*duplicateBatchStats{}
	for rows.Next() {
		var batchID int64
		var title, sourceName, content string
		var createdAt time.Time
		if err := rows.Scan(&batchID, &title, &sourceName, &createdAt, &content); err != nil {
			return nil, err
		}
		stats := statsByBatch[batchID]
		if stats == nil {
			stats = &duplicateBatchStats{
				batchID:      batchID,
				title:        title,
				sourceName:   sourceName,
				createdAt:    createdAt,
				existingKeys: map[string]bool{},
			}
			statsByBatch[batchID] = stats
		}
		key := QuestionContentKey(content)
		if key == "" || stats.existingKeys[key] {
			continue
		}
		stats.existingKeys[key] = true
		if incomingKeys[key] {
			stats.matchingCount++
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	candidates := make([]ImportDuplicateCandidate, 0, len(statsByBatch))
	threshold := maxInt(3, len(incomingKeys)/10)
	for _, stats := range statsByBatch {
		if stats.matchingCount < threshold {
			continue
		}
		candidates = append(candidates, ImportDuplicateCandidate{
			BatchID:               stats.batchID,
			Title:                 stats.title,
			SourceName:            stats.sourceName,
			ExistingQuestionCount: len(stats.existingKeys),
			MatchingQuestionCount: stats.matchingCount,
			NewQuestionCount:      maxInt(0, len(incomingKeys)-stats.matchingCount),
			CreatedAt:             stats.createdAt.Format("02/01/2006 15:04"),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].MatchingQuestionCount == candidates[j].MatchingQuestionCount {
			return candidates[i].BatchID > candidates[j].BatchID
		}
		return candidates[i].MatchingQuestionCount > candidates[j].MatchingQuestionCount
	})
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	return candidates, nil
}

func UpdateImportItem(ctx context.Context, db *pgxpool.Pool, batchID int64, question ParsedQuestion) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if batchID <= 0 || question.ImportItemID <= 0 {
		return fmt.Errorf("missing import batch or item id")
	}
	payload, err := json.Marshal(question)
	if err != nil {
		return err
	}
	reviewStatus := "pending"
	if question.Status == "fail" {
		reviewStatus = "rejected"
	}
	commandTag, err := db.Exec(ctx, `
		UPDATE import_items
		SET raw_question_text = $1,
			parsed_question_json = $2::jsonb,
			normalized_question_json = $2::jsonb,
			ai_confidence = $3,
			review_status = $4,
			review_note = $5,
			updated_at = NOW()
		WHERE id = $6 AND batch_id = $7
	`, rawQuestionText(question), string(payload), float64(question.Confidence)/100, reviewStatus, strings.Join(question.Warnings, "\n"), question.ImportItemID, batchID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("import item not found")
	}
	return nil
}

func CreateImportItem(ctx context.Context, db *pgxpool.Pool, batchID int64, question ParsedQuestion) (ParsedQuestion, error) {
	if db == nil {
		return question, fmt.Errorf("database is not configured")
	}
	if batchID <= 0 {
		return question, fmt.Errorf("missing import batch id")
	}
	if question.SourceOrder <= 0 {
		question.SourceOrder = 1
	}
	question.ImportItemID = 0
	payload, err := json.Marshal(question)
	if err != nil {
		return question, err
	}
	reviewStatus := "pending"
	if question.Status == "fail" {
		reviewStatus = "rejected"
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return question, err
	}
	defer tx.Rollback(ctx)

	var itemOrder int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(item_order), 0) + 1 FROM import_items WHERE batch_id = $1`, batchID).Scan(&itemOrder); err != nil {
		return question, err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO import_items (
			batch_id,
			item_order,
			source_order,
			raw_question_text,
			parsed_question_json,
			normalized_question_json,
			ai_confidence,
			review_status,
			review_note
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $5::jsonb, $6, $7, $8)
		RETURNING id
	`, batchID, itemOrder, question.SourceOrder, rawQuestionText(question), string(payload), float64(question.Confidence)/100, reviewStatus, strings.Join(question.Warnings, "\n")).Scan(&question.ImportItemID); err != nil {
		return question, err
	}
	payload, err = json.Marshal(question)
	if err != nil {
		return question, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE import_items
		SET parsed_question_json = $1::jsonb,
			normalized_question_json = $1::jsonb
		WHERE id = $2
	`, string(payload), question.ImportItemID); err != nil {
		return question, err
	}
	if err := tx.Commit(ctx); err != nil {
		return question, err
	}
	return question, nil
}

func RejectImportItem(ctx context.Context, db *pgxpool.Pool, batchID int64, importItemID int64) error {
	if db == nil {
		return fmt.Errorf("database is not configured")
	}
	if batchID <= 0 || importItemID <= 0 {
		return fmt.Errorf("missing import batch or item id")
	}
	commandTag, err := db.Exec(ctx, `
		UPDATE import_items
		SET review_status = 'rejected',
			review_note = 'Giáo viên xoá khỏi batch import.',
			updated_at = NOW()
		WHERE id = $1 AND batch_id = $2
	`, importItemID, batchID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("import item not found")
	}
	return nil
}

type ApprovePassedResult struct {
	ImportBatchID   int64   `json:"importBatchId"`
	TargetBatchID   int64   `json:"targetBatchId,omitempty"`
	Approved        int     `json:"approved"`
	AlreadyApproved int     `json:"alreadyApproved"`
	Skipped         int     `json:"skipped"`
	Rejected        int     `json:"rejected"`
	QuestionCount   int     `json:"questionCount"`
	QuestionIDs     []int64 `json:"questionIds"`
}

type importItemForApproval struct {
	ID              int64
	Payload         []byte
	ReviewStatus    string
	SavedQuestionID sql.NullInt64
}

func ApprovePassedImportItems(ctx context.Context, db *pgxpool.Pool, batchID int64) (ApprovePassedResult, error) {
	return ApprovePassedImportItemsToSource(ctx, db, batchID, 0)
}

func ApprovePassedImportItemsToSource(ctx context.Context, db *pgxpool.Pool, batchID int64, targetBatchID int64) (ApprovePassedResult, error) {
	result := ApprovePassedResult{ImportBatchID: batchID}
	if db == nil {
		return result, fmt.Errorf("database is not configured")
	}
	if targetBatchID <= 0 {
		targetBatchID = batchID
	}
	result.TargetBatchID = targetBatchID
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var sourceOwner, targetOwner sql.NullInt64
	if err := tx.QueryRow(ctx, `SELECT uploaded_by_user_id FROM import_batches WHERE id = $1`, batchID).Scan(&sourceOwner); err != nil {
		return result, fmt.Errorf("source import batch not found")
	}
	if err := tx.QueryRow(ctx, `SELECT uploaded_by_user_id FROM import_batches WHERE id = $1`, targetBatchID).Scan(&targetOwner); err != nil {
		return result, fmt.Errorf("target question bank source not found")
	}
	if targetBatchID != batchID && (!sourceOwner.Valid || !targetOwner.Valid || sourceOwner.Int64 != targetOwner.Int64) {
		return result, fmt.Errorf("target question bank source is not owned by the same teacher")
	}
	if targetBatchID != batchID {
		var imageCount int
		if err := tx.QueryRow(ctx, `SELECT COALESCE(image_count, 0) FROM import_batches WHERE id = $1`, batchID).Scan(&imageCount); err != nil {
			return result, err
		}
		if imageCount > 0 {
			return result, fmt.Errorf("cannot merge imports with extracted images yet; create a new question bank source instead")
		}
	}

	targetKeys, err := activeQuestionKeysByBatch(ctx, tx, targetBatchID)
	if err != nil {
		return result, err
	}
	var nextTargetOrder int
	if targetBatchID != batchID {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(MAX(item_order), 0) FROM import_items WHERE batch_id = $1`, targetBatchID).Scan(&nextTargetOrder); err != nil {
			return result, err
		}
	}

	rows, err := tx.Query(ctx, `
		SELECT ii.id, ii.parsed_question_json, ii.review_status,
			qb.id
		FROM import_items ii
		LEFT JOIN question_bank qb ON qb.import_item_id = ii.id
		WHERE ii.batch_id = $1
		ORDER BY ii.item_order
	`, batchID)
	if err != nil {
		return result, err
	}

	items := []importItemForApproval{}
	for rows.Next() {
		var item importItemForApproval
		if err := rows.Scan(&item.ID, &item.Payload, &item.ReviewStatus, &item.SavedQuestionID); err != nil {
			rows.Close()
			return result, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return result, err
	}
	rows.Close()

	for _, item := range items {
		if item.SavedQuestionID.Valid || item.ReviewStatus == "approved" {
			if item.SavedQuestionID.Valid {
				result.QuestionIDs = append(result.QuestionIDs, item.SavedQuestionID.Int64)
			}
			result.AlreadyApproved++
			continue
		}
		if item.ReviewStatus == "rejected" {
			result.Rejected++
			continue
		}
		var question ParsedQuestion
		if err := json.Unmarshal(item.Payload, &question); err != nil {
			result.Rejected++
			continue
		}
		if question.Status != "pass" {
			result.Skipped++
			continue
		}
		if targetBatchID != batchID {
			key := QuestionContentKey(question.Content)
			if key != "" && targetKeys[key] {
				if _, err := tx.Exec(ctx, `
					UPDATE import_items
					SET review_status = 'rejected',
						review_note = 'Duplicate of selected existing question bank source.',
						updated_at = NOW()
					WHERE id = $1
				`, item.ID); err != nil {
					return result, err
				}
				result.Skipped++
				continue
			}
			nextTargetOrder++
			if _, err := tx.Exec(ctx, `
				UPDATE import_items
				SET batch_id = $1,
					item_order = $2,
					updated_at = NOW()
				WHERE id = $3
			`, targetBatchID, nextTargetOrder, item.ID); err != nil {
				return result, err
			}
			targetKeys[key] = true
		}
		var questionID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO question_bank (import_item_id, question_type, content, question_status)
			VALUES ($1, 'single_choice', $2, 'active')
			RETURNING id
		`, item.ID, question.Content).Scan(&questionID); err != nil {
			return result, err
		}
		for index, option := range question.Options {
			if _, err := tx.Exec(ctx, `
				INSERT INTO question_bank_options (question_id, option_order, content, is_correct)
				VALUES ($1, $2, $3, $4)
			`, questionID, index+1, option.Content, strings.EqualFold(option.Label, question.CorrectLabel)); err != nil {
				return result, err
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE import_items SET review_status = 'approved', updated_at = NOW() WHERE id = $1`, item.ID); err != nil {
			return result, err
		}
		result.QuestionIDs = append(result.QuestionIDs, questionID)
		result.Approved++
	}
	if targetBatchID != batchID {
		if _, err := tx.Exec(ctx, `DELETE FROM import_batches WHERE id = $1`, batchID); err != nil {
			return result, err
		}
		result.ImportBatchID = targetBatchID
	}
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(qb.id)::int
		FROM import_items ii
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ii.batch_id = $1
	`, targetBatchID).Scan(&result.QuestionCount); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	if targetBatchID != batchID {
		_ = storage.RemoveImportBatch(batchID)
	}
	return result, nil
}

func activeQuestionKeysByBatch(ctx context.Context, tx pgx.Tx, batchID int64) (map[string]bool, error) {
	rows, err := tx.Query(ctx, `
		SELECT qb.content
		FROM import_items ii
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ii.batch_id = $1
	`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	keys := map[string]bool{}
	for rows.Next() {
		var content string
		if err := rows.Scan(&content); err != nil {
			return nil, err
		}
		key := QuestionContentKey(content)
		if key != "" {
			keys[key] = true
		}
	}
	return keys, rows.Err()
}

func saveOriginalUpload(batchID int64, filename string, content []byte) (string, error) {
	safeName := unsafeFilenameChars.ReplaceAllString(filename, "_")
	if safeName == "" || safeName == "." {
		safeName = "source-file"
	}
	return storage.SaveImportFile(batchID, safeName, content)
}

func saveExtractedAssets(ctx context.Context, tx pgx.Tx, batchID int64, assets []ExtractedAsset) error {
	for index, asset := range assets {
		safeName := unsafeFilenameChars.ReplaceAllString(asset.FileName, "_")
		if safeName == "" || safeName == "." {
			safeName = fmt.Sprintf("asset-%03d.bin", index+1)
		}
		path, err := storage.SaveImportAsset(batchID, safeName, asset.Data)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO import_item_assets (
				batch_id,
				target,
				file_url,
				mime_type,
				display_order
			)
			VALUES ($1, 'unknown', $2, $3, $4)
		`, batchID, path, asset.MimeType, index+1); err != nil {
			return err
		}
	}
	return nil
}

func parseStatusFor(result *ParseUploadResult) string {
	if result.Extract.NeedsConversion {
		return "needs_conversion"
	}
	if result.Extract.NeedsOCR {
		return "needs_ocr"
	}
	if result.Extract.Status == "unsupported" || result.Extract.Status == "failed" {
		return "failed"
	}
	if len(result.Questions) > 0 {
		return "parsed"
	}
	return "pending"
}

func sourceTypeFor(kind string) string {
	switch strings.ToLower(kind) {
	case "doc":
		return "doc"
	case "docx":
		return "docx"
	case "pdf":
		return "pdf"
	case "txt":
		return "txt"
	case "xlsx":
		return "xlsx"
	case "csv":
		return "csv"
	default:
		return "other"
	}
}

func assetSummaryJSON(result *ParseUploadResult) string {
	payload, err := json.Marshal(map[string]any{
		"imageCount":        result.Extract.ImageCount,
		"pageEstimate":      result.Extract.PageEstimate,
		"documentTitle":     result.Extract.DocumentTitle,
		"headingCandidates": result.Extract.HeadingCandidates,
	})
	if err != nil {
		return "{}"
	}
	return string(payload)
}

func rawQuestionText(question ParsedQuestion) string {
	var builder strings.Builder
	builder.WriteString(question.Content)
	for _, option := range question.Options {
		builder.WriteString("\n")
		builder.WriteString(option.Label)
		builder.WriteString(". ")
		builder.WriteString(option.Content)
	}
	if question.CorrectLabel != "" {
		builder.WriteString("\nĐáp án: ")
		builder.WriteString(question.CorrectLabel)
	}
	return builder.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
