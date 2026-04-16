package importdata

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func SaveImport(ctx context.Context, db *pgxpool.Pool, result *ParseUploadResult) error {
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

	var batchID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO import_batches (
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
		VALUES ($1, $2, $3, $4, NULLIF($5, ''), NULLIF($6, ''), $7, $8, $9, $10, $11, $12::jsonb, NULLIF($13, ''))
		RETURNING id
	`, result.File.Name, result.File.Name, sourceTypeFor(result.File.Kind), result.File.Size, result.FileSHA256, ContentFingerprint(result.Extract.Text), rawContent, "local-rule-parser-v1", parseStatus, result.Extract.DocumentTitle, result.Extract.ImageCount, assetSummaryJSON(result), result.Extract.Warning).Scan(&batchID); err != nil {
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

	if err := tx.Commit(ctx); err != nil {
		return err
	}
	result.ImportBatchID = batchID
	return nil
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

type ApprovePassedResult struct {
	ImportBatchID   int64 `json:"importBatchId"`
	Approved        int   `json:"approved"`
	AlreadyApproved int   `json:"alreadyApproved"`
	Skipped         int   `json:"skipped"`
	Rejected        int   `json:"rejected"`
}

func ApprovePassedImportItems(ctx context.Context, db *pgxpool.Pool, batchID int64) (ApprovePassedResult, error) {
	result := ApprovePassedResult{ImportBatchID: batchID}
	if db == nil {
		return result, fmt.Errorf("database is not configured")
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT ii.id, ii.parsed_question_json, ii.review_status,
			EXISTS (SELECT 1 FROM question_bank qb WHERE qb.import_item_id = ii.id) AS already_saved
		FROM import_items ii
		WHERE ii.batch_id = $1
		ORDER BY ii.item_order
	`, batchID)
	if err != nil {
		return result, err
	}
	defer rows.Close()

	for rows.Next() {
		var itemID int64
		var payload []byte
		var reviewStatus string
		var alreadySaved bool
		if err := rows.Scan(&itemID, &payload, &reviewStatus, &alreadySaved); err != nil {
			return result, err
		}
		if alreadySaved || reviewStatus == "approved" {
			result.AlreadyApproved++
			continue
		}
		var question ParsedQuestion
		if err := json.Unmarshal(payload, &question); err != nil {
			result.Rejected++
			continue
		}
		if question.Status != "pass" {
			result.Skipped++
			continue
		}
		var questionID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO question_bank (import_item_id, question_type, content, question_status)
			VALUES ($1, 'single_choice', $2, 'active')
			RETURNING id
		`, itemID, question.Content).Scan(&questionID); err != nil {
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
		if _, err := tx.Exec(ctx, `UPDATE import_items SET review_status = 'approved', updated_at = NOW() WHERE id = $1`, itemID); err != nil {
			return result, err
		}
		result.Approved++
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func saveOriginalUpload(batchID int64, filename string, content []byte) (string, error) {
	dir := filepath.Join("data", "imports", fmt.Sprintf("%d", batchID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	safeName := unsafeFilenameChars.ReplaceAllString(filename, "_")
	if safeName == "" || safeName == "." {
		safeName = "source-file"
	}
	path := filepath.Join(dir, safeName)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	return filepath.ToSlash(path), nil
}

func saveExtractedAssets(ctx context.Context, tx pgx.Tx, batchID int64, assets []ExtractedAsset) error {
	dir := filepath.Join("data", "imports", fmt.Sprintf("%d", batchID), "assets")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for index, asset := range assets {
		safeName := unsafeFilenameChars.ReplaceAllString(asset.FileName, "_")
		if safeName == "" || safeName == "." {
			safeName = fmt.Sprintf("asset-%03d.bin", index+1)
		}
		path := filepath.Join(dir, safeName)
		if err := os.WriteFile(path, asset.Data, 0o644); err != nil {
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
		`, batchID, filepath.ToSlash(path), asset.MimeType, index+1); err != nil {
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
