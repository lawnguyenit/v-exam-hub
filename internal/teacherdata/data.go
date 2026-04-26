package teacherdata

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

type Dashboard struct {
	Profile Profile       `json:"profile"`
	Exams   []ExamSummary `json:"exams"`
}

func effectiveExamStatus(status string, start *time.Time, now time.Time) string {
	if status == "scheduled" && start != nil && !start.After(now) {
		return "open"
	}
	return status
}

type Profile struct {
	DisplayName string `json:"displayName"`
	TeacherCode string `json:"teacherCode"`
	Department  string `json:"department"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

type ProfileUpdateRequest struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Department  string `json:"department"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
}

type ExamSummary struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Status      string  `json:"status"`
	ExamType    string  `json:"examType"`
	TargetClass string  `json:"targetClass"`
	StartTime   string  `json:"startTime"`
	Average     float64 `json:"average"`
	Submitted   int     `json:"submitted"`
	Total       int     `json:"total"`
}

type ExamDetail struct {
	ID                    string                     `json:"id"`
	Title                 string                     `json:"title"`
	Description           string                     `json:"description"`
	Status                string                     `json:"status"`
	StatusCode            string                     `json:"statusCode"`
	ExamType              string                     `json:"examType"`
	ExamMode              string                     `json:"examMode"`
	TargetClass           string                     `json:"targetClass"`
	ClassID               int64                      `json:"classId"`
	StartTime             string                     `json:"startTime"`
	StartValue            string                     `json:"startValue"`
	DurationMinutes       int                        `json:"durationMinutes"`
	MaxAttemptsPerStudent int                        `json:"maxAttemptsPerStudent"`
	ShuffleQuestions      bool                       `json:"shuffleQuestions"`
	ShuffleOptions        bool                       `json:"shuffleOptions"`
	ShowResultImmediately bool                       `json:"showResultImmediately"`
	AllowReview           bool                       `json:"allowReview"`
	QuestionSourceID      int64                      `json:"questionSourceId"`
	QuestionCount         int                        `json:"questionCount"`
	CanEdit               bool                       `json:"canEdit"`
	Average               float64                    `json:"average"`
	Submitted             int                        `json:"submitted"`
	Total                 int                        `json:"total"`
	Metrics               []Metric                   `json:"metrics"`
	Tables                map[string]StatisticsTable `json:"tables"`
	Students              []StudentAttemptDetail     `json:"students"`
}

type Metric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

type StatisticsTable struct {
	Title   string     `json:"title"`
	Columns []string   `json:"columns"`
	Rows    [][]string `json:"rows"`
}

type StudentAttemptDetail struct {
	Name         string          `json:"name"`
	StudentCode  string          `json:"studentCode"`
	Progress     string          `json:"progress"`
	Warning      string          `json:"warning"`
	Score        string          `json:"score"`
	Duration     string          `json:"duration"`
	AttemptCount int             `json:"attemptCount"`
	Attempts     []AttemptDetail `json:"attempts"`
	WrongItems   []WrongItem     `json:"wrongItems"`
}

type AttemptDetail struct {
	AttemptNo   int         `json:"attemptNo"`
	Score       string      `json:"score"`
	Duration    string      `json:"duration"`
	Status      string      `json:"status"`
	SubmittedAt string      `json:"submittedAt"`
	WrongItems  []WrongItem `json:"wrongItems"`
}

type WrongItem struct {
	Question string `json:"question"`
	Selected string `json:"selected"`
	Correct  string `json:"correct"`
	Note     string `json:"note"`
}

type QuestionBankItem struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	SourceName    string `json:"sourceName"`
	QuestionCount int    `json:"questionCount"`
	CreatedAt     string `json:"createdAt"`
}

type QuestionBankDeleteResult struct {
	ID                int64 `json:"id"`
	ArchivedQuestions int   `json:"archivedQuestions"`
	DeletedQuestions  int   `json:"deletedQuestions"`
	RemovedBatch      bool  `json:"removedBatch"`
}

type ExamCreateRequest struct {
	ExamID                string  `json:"examId"`
	CreatedBy             string  `json:"createdBy"`
	Title                 string  `json:"title"`
	Description           string  `json:"description"`
	ExamMode              string  `json:"examMode"`
	ClassID               int64   `json:"classId"`
	StartTime             string  `json:"startTime"`
	DurationMinutes       int     `json:"durationMinutes"`
	MaxAttemptsPerStudent int     `json:"maxAttemptsPerStudent"`
	ShuffleQuestions      bool    `json:"shuffleQuestions"`
	ShuffleOptions        bool    `json:"shuffleOptions"`
	ShowResultImmediately bool    `json:"showResultImmediately"`
	AllowReview           bool    `json:"allowReview"`
	QuestionIDs           []int64 `json:"questionIds"`
	QuestionSourceID      int64   `json:"questionSourceId"`
	QuestionCount         int     `json:"questionCount"`
}

type ExamCreateResult struct {
	ID            string `json:"id"`
	QuestionCount int    `json:"questionCount"`
	Status        string `json:"status"`
}

type AccessCodeResult struct {
	ExamID         string `json:"examId"`
	Code           string `json:"code"`
	ExpiresAt      string `json:"expiresAt"`
	ExpiresAtUnix  int64  `json:"expiresAtUnix"`
	DurationMinute int    `json:"durationMinute"`
}

type ExamLiveSnapshot struct {
	ExamID      string            `json:"examId"`
	GeneratedAt string            `json:"generatedAt"`
	Total       int               `json:"total"`
	InProgress  int               `json:"inProgress"`
	Submitted   int               `json:"submitted"`
	NotStarted  int               `json:"notStarted"`
	Rows        []LiveSnapshotRow `json:"rows"`
}

type LiveSnapshotRow struct {
	StudentCode  string `json:"studentCode"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	AttemptCount int    `json:"attemptCount"`
	BestScore    string `json:"bestScore"`
	LastSeen     string `json:"lastSeen"`
}

type examBase struct {
	ID          int64
	Title       string
	Status      string
	Mode        string
	TargetClass string
	StartTime   *time.Time
	QuestionCnt int
	Submitted   int
	Total       int
	Average     float64
	Highest     float64
	AttemptCnt  int
}

type examConfig struct {
	Description           string
	ClassID               int64
	DurationMinutes       int
	MaxAttemptsPerStudent int
	ShuffleQuestions      bool
	ShuffleOptions        bool
	ShowResultImmediately bool
	AllowReview           bool
	QuestionSourceID      int64
}

var appLocation = loadAppLocation()

func loadAppLocation() *time.Location {
	location, err := time.LoadLocation("Asia/Ho_Chi_Minh")
	if err != nil {
		return time.FixedZone("Asia/Ho_Chi_Minh", 7*60*60)
	}
	return location
}

func DashboardFor(ctx context.Context, db *pgxpool.Pool, account string) (Dashboard, error) {
	profile, err := profileFor(ctx, db, account)
	if err != nil {
		return Dashboard{}, err
	}
	exams, err := examSummaries(ctx, db, account)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{Profile: profile, Exams: exams}, nil
}

func UpdateProfile(ctx context.Context, db *pgxpool.Pool, payload ProfileUpdateRequest) (Profile, error) {
	if strings.TrimSpace(payload.Username) == "" {
		return Profile{}, fmt.Errorf("missing username")
	}
	var userID int64
	if err := db.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, payload.Username).Scan(&userID); err != nil {
		return Profile{}, err
	}
	if _, err := db.Exec(ctx, `
		UPDATE teacher_profiles
		SET full_name = NULLIF($1, ''),
			department = NULLIF($2, ''),
			email = NULLIF($3, ''),
			phone = NULLIF($4, ''),
			updated_at = NOW()
		WHERE user_id = $5
	`, strings.TrimSpace(payload.DisplayName), strings.TrimSpace(payload.Department), strings.TrimSpace(payload.Email), strings.TrimSpace(payload.Phone), userID); err != nil {
		return Profile{}, err
	}
	return profileFor(ctx, db, payload.Username)
}

func QuestionBank(ctx context.Context, db *pgxpool.Pool, account string) ([]QuestionBankItem, error) {
	rows, err := db.Query(ctx, `
		SELECT ib.id,
			COALESCE(NULLIF(ib.original_filename, ''), NULLIF(ib.source_name, ''), 'Bộ đề #' || ib.id::text),
			COALESCE(NULLIF(ib.source_name, ''), NULLIF(ib.original_filename, ''), ''),
			COUNT(qb.id)::int,
			MAX(qb.created_at)
		FROM import_batches ib
		JOIN import_items ii ON ii.batch_id = ib.id
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ib.uploaded_by_user_id = (SELECT id FROM users WHERE username = $1)
		GROUP BY ib.id
		ORDER BY MAX(qb.created_at) DESC, ib.id DESC
		LIMIT 200
	`, strings.TrimSpace(account))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []QuestionBankItem{}
	for rows.Next() {
		var item QuestionBankItem
		var createdAt time.Time
		if err := rows.Scan(&item.ID, &item.Title, &item.SourceName, &item.QuestionCount, &createdAt); err != nil {
			return nil, err
		}
		item.CreatedAt = createdAt.In(appLocation).Format("02/01/2006 15:04")
		items = append(items, item)
	}
	return items, rows.Err()
}

func DeleteQuestionBankSource(ctx context.Context, db *pgxpool.Pool, batchID int64, account string) (QuestionBankDeleteResult, error) {
	result := QuestionBankDeleteResult{ID: batchID}
	if batchID <= 0 {
		return result, fmt.Errorf("missing question bank source id")
	}
	account = strings.TrimSpace(account)
	if account == "" {
		return result, fmt.Errorf("missing teacher account")
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var ownerID int64
	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, account).Scan(&ownerID); err != nil {
		return result, fmt.Errorf("teacher account not found")
	}

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM import_batches
			WHERE id = $1 AND uploaded_by_user_id = $2
		)
	`, batchID, ownerID).Scan(&exists); err != nil {
		return result, err
	}
	if !exists {
		return result, fmt.Errorf("question bank source not found or not owned by this account")
	}

	archiveTag, err := tx.Exec(ctx, `
		UPDATE question_bank qb
		SET question_status = 'archived',
			updated_at = NOW()
		FROM import_items ii
		WHERE qb.import_item_id = ii.id
			AND ii.batch_id = $1
			AND qb.question_status <> 'archived'
			AND EXISTS (
				SELECT 1
				FROM exam_questions eq
				WHERE eq.question_id = qb.id
			)
	`, batchID)
	if err != nil {
		return result, err
	}
	result.ArchivedQuestions = int(archiveTag.RowsAffected())

	deleteTag, err := tx.Exec(ctx, `
		DELETE FROM question_bank qb
		USING import_items ii
		WHERE qb.import_item_id = ii.id
			AND ii.batch_id = $1
			AND NOT EXISTS (
				SELECT 1
				FROM exam_questions eq
				WHERE eq.question_id = qb.id
			)
	`, batchID)
	if err != nil {
		return result, err
	}
	result.DeletedQuestions = int(deleteTag.RowsAffected())

	var remainingReferenced int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(qb.id)::int
		FROM import_items ii
		JOIN question_bank qb ON qb.import_item_id = ii.id
		WHERE ii.batch_id = $1
			AND EXISTS (
				SELECT 1
				FROM exam_questions eq
				WHERE eq.question_id = qb.id
			)
	`, batchID).Scan(&remainingReferenced); err != nil {
		return result, err
	}
	if remainingReferenced == 0 {
		deleteBatchTag, err := tx.Exec(ctx, `DELETE FROM import_batches WHERE id = $1 AND uploaded_by_user_id = $2`, batchID, ownerID)
		if err != nil {
			return result, err
		}
		result.RemovedBatch = deleteBatchTag.RowsAffected() > 0
	}

	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	if result.RemovedBatch {
		_ = os.RemoveAll(filepath.Join("data", "imports", fmt.Sprintf("%d", batchID)))
	}
	return result, nil
}

func CreateExam(ctx context.Context, db *pgxpool.Pool, payload ExamCreateRequest) (ExamCreateResult, error) {
	if strings.TrimSpace(payload.ExamID) != "" {
		return updateExam(ctx, db, payload)
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return ExamCreateResult{}, fmt.Errorf("thiếu tên bài kiểm tra")
	}
	questionIDs := uniquePositiveIDs(payload.QuestionIDs)
	if len(questionIDs) == 0 && payload.QuestionSourceID <= 0 {
		return ExamCreateResult{}, fmt.Errorf("cần chọn một bộ đề cương")
	}
	if payload.ClassID <= 0 {
		return ExamCreateResult{}, fmt.Errorf("cần chọn lớp áp dụng")
	}
	durationMinutes := payload.DurationMinutes
	if durationMinutes <= 0 {
		durationMinutes = 45
	}
	maxAttempts := payload.MaxAttemptsPerStudent
	if maxAttempts < 0 {
		maxAttempts = 1
	}
	mode := examModeValue(payload.ExamMode)
	start, err := parseCreateStart(payload.StartTime)
	if err != nil {
		return ExamCreateResult{}, err
	}
	status := "open"
	if start == nil {
		status = "draft"
	} else if start.After(time.Now()) {
		status = "scheduled"
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ExamCreateResult{}, err
	}
	defer tx.Rollback(ctx)

	var creatorID any
	if strings.TrimSpace(payload.CreatedBy) != "" {
		var id int64
		if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE username = $1`, strings.TrimSpace(payload.CreatedBy)).Scan(&id); err == nil {
			creatorID = id
		}
	}

	if len(questionIDs) == 0 {
		questionIDs, err = questionIDsFromSource(ctx, tx, payload.QuestionSourceID, payload.QuestionCount, payload.ShuffleQuestions, creatorID)
		if err != nil {
			return ExamCreateResult{}, err
		}
	}
	if len(questionIDs) == 0 {
		return ExamCreateResult{}, fmt.Errorf("bộ đề cương chưa có câu hỏi active")
	}

	var availableCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM question_bank WHERE id = ANY($1) AND question_status = 'active'`, questionIDs).Scan(&availableCount); err != nil {
		return ExamCreateResult{}, err
	}
	if availableCount != len(questionIDs) {
		return ExamCreateResult{}, fmt.Errorf("một số câu hỏi đã chọn không còn hợp lệ trong ngân hàng")
	}

	var examID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO exams (
			created_by_user_id, title, description, exam_mode, duration_seconds,
			total_points, exam_status, max_attempts_per_student,
			shuffle_questions, shuffle_options, show_result_immediately, allow_review,
			start_time, published_at
		)
		VALUES ($1, $2, NULLIF($3, ''), $4::exam_mode_enum, $5, $6, $7::exam_status_enum, $8, $9, $10, $11, $12, $13, CASE WHEN $7 IN ('open', 'scheduled') THEN NOW() ELSE NULL END)
		RETURNING id
	`, creatorID, title, strings.TrimSpace(payload.Description), mode, durationMinutes*60, float64(len(questionIDs)), status, maxAttempts, payload.ShuffleQuestions, payload.ShuffleOptions, payload.ShowResultImmediately, payload.AllowReview, start).Scan(&examID); err != nil {
		return ExamCreateResult{}, err
	}

	for index, questionID := range questionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO exam_questions (exam_id, question_id, question_order, points_override)
			VALUES ($1, $2, $3, 1)
		`, examID, questionID, index+1); err != nil {
			return ExamCreateResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO exam_targets (exam_id, class_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, examID, payload.ClassID); err != nil {
		return ExamCreateResult{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return ExamCreateResult{}, err
	}
	return ExamCreateResult{ID: strconv.FormatInt(examID, 10), QuestionCount: len(questionIDs), Status: statusLabel(status)}, nil
}

func updateExam(ctx context.Context, db *pgxpool.Pool, payload ExamCreateRequest) (ExamCreateResult, error) {
	examID, err := strconv.ParseInt(strings.TrimSpace(payload.ExamID), 10, 64)
	if err != nil || examID <= 0 {
		return ExamCreateResult{}, fmt.Errorf("mã bài kiểm tra không hợp lệ")
	}
	title := strings.TrimSpace(payload.Title)
	if title == "" {
		return ExamCreateResult{}, fmt.Errorf("thiếu tên bài kiểm tra")
	}
	if payload.ClassID <= 0 {
		return ExamCreateResult{}, fmt.Errorf("cần chọn lớp áp dụng")
	}
	durationMinutes := payload.DurationMinutes
	if durationMinutes <= 0 {
		durationMinutes = 45
	}
	maxAttempts := payload.MaxAttemptsPerStudent
	if maxAttempts < 0 {
		maxAttempts = 1
	}
	start, err := parseCreateStart(payload.StartTime)
	if err != nil {
		return ExamCreateResult{}, err
	}
	status := "open"
	if start == nil {
		status = "draft"
	} else if start.After(time.Now()) {
		status = "scheduled"
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ExamCreateResult{}, err
	}
	defer tx.Rollback(ctx)

	var attemptCount int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM exam_attempts WHERE exam_id = $1`, examID).Scan(&attemptCount); err != nil {
		return ExamCreateResult{}, err
	}
	var lockedSourceID sql.NullInt64
	if attemptCount > 0 {
		if err := tx.QueryRow(ctx, `
			SELECT MIN(ii.batch_id)
			FROM exam_questions eq
			JOIN question_bank qb ON qb.id = eq.question_id
			JOIN import_items ii ON ii.id = qb.import_item_id
			WHERE eq.exam_id = $1
		`, examID).Scan(&lockedSourceID); err != nil {
			return ExamCreateResult{}, err
		}
		if payload.QuestionSourceID <= 0 && lockedSourceID.Valid {
			payload.QuestionSourceID = lockedSourceID.Int64
		}
		if lockedSourceID.Valid && payload.QuestionSourceID > 0 && payload.QuestionSourceID != lockedSourceID.Int64 {
			return ExamCreateResult{}, fmt.Errorf("bai da co luot lam nen khong the doi nguon de cuong; chi duoc doi so cau va cau hinh phat bai")
		}
	}

	questionIDs := uniquePositiveIDs(payload.QuestionIDs)
	if len(questionIDs) == 0 {
		questionIDs, err = questionIDsFromSource(ctx, tx, payload.QuestionSourceID, payload.QuestionCount, payload.ShuffleQuestions, nil)
		if err != nil {
			return ExamCreateResult{}, err
		}
	}
	if len(questionIDs) == 0 {
		return ExamCreateResult{}, fmt.Errorf("bộ đề cương chưa có câu hỏi active")
	}

	commandTag, err := tx.Exec(ctx, `
		UPDATE exams
		SET title = $2,
			description = NULLIF($3, ''),
			exam_mode = $4::exam_mode_enum,
			duration_seconds = $5,
			total_points = $6,
			exam_status = $7::exam_status_enum,
			max_attempts_per_student = $8,
			shuffle_questions = $9,
			shuffle_options = $10,
			show_result_immediately = $11,
			allow_review = $12,
			start_time = $13,
			published_at = CASE WHEN $7 IN ('open', 'scheduled') THEN COALESCE(published_at, NOW()) ELSE published_at END,
			updated_at = NOW()
		WHERE id = $1
	`, examID, title, strings.TrimSpace(payload.Description), examModeValue(payload.ExamMode), durationMinutes*60, float64(len(questionIDs)), status, maxAttempts, payload.ShuffleQuestions, payload.ShuffleOptions, payload.ShowResultImmediately, payload.AllowReview, start)
	if err != nil {
		return ExamCreateResult{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return ExamCreateResult{}, fmt.Errorf("không tìm thấy bài kiểm tra")
	}

	if attemptCount == 0 {
		if _, err := tx.Exec(ctx, `DELETE FROM exam_versions WHERE exam_id = $1`, examID); err != nil {
			return ExamCreateResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM exam_questions WHERE exam_id = $1`, examID); err != nil {
		return ExamCreateResult{}, err
	}
	for index, questionID := range questionIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO exam_questions (exam_id, question_id, question_order, points_override)
			VALUES ($1, $2, $3, 1)
		`, examID, questionID, index+1); err != nil {
			return ExamCreateResult{}, err
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM exam_targets WHERE exam_id = $1`, examID); err != nil {
		return ExamCreateResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO exam_targets (exam_id, class_id)
		VALUES ($1, $2)
	`, examID, payload.ClassID); err != nil {
		return ExamCreateResult{}, err
	}
	if attemptCount > 0 {
		if _, err := createExamVersionSnapshot(ctx, tx, examID); err != nil {
			return ExamCreateResult{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return ExamCreateResult{}, err
	}
	return ExamCreateResult{ID: strconv.FormatInt(examID, 10), QuestionCount: len(questionIDs), Status: statusLabel(status)}, nil
}

func createExamVersionSnapshot(ctx context.Context, tx pgx.Tx, examID int64) (int64, error) {
	var versionID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO exam_versions (
			exam_id, version_no, title_snapshot, description_snapshot,
			exam_mode_snapshot, duration_seconds_snapshot, total_points_snapshot,
			exam_status_snapshot, shuffle_questions_snapshot, shuffle_options_snapshot,
			show_result_immediately_snapshot, allow_review_snapshot,
			start_time_snapshot, end_time_snapshot, published_by_user_id
		)
		SELECT id,
			COALESCE((SELECT MAX(version_no) FROM exam_versions WHERE exam_id = exams.id), 0) + 1,
			title, description, exam_mode, duration_seconds,
			CASE WHEN total_points > 0 THEN total_points ELSE GREATEST((SELECT COUNT(*) FROM exam_questions WHERE exam_id = exams.id), 1) END,
			exam_status, shuffle_questions, shuffle_options,
			show_result_immediately, allow_review, start_time, end_time, created_by_user_id
		FROM exams
		WHERE id = $1
		RETURNING id
	`, examID).Scan(&versionID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO exam_version_questions (
			exam_version_id, source_question_id, question_order,
			question_type_snapshot, content_snapshot, explanation_snapshot, points_snapshot
		)
		SELECT $1, qb.id, eq.question_order, qb.question_type, qb.content, qb.explanation,
			COALESCE(eq.points_override, 1)
		FROM exam_questions eq
		JOIN question_bank qb ON qb.id = eq.question_id
		WHERE eq.exam_id = $2
		ORDER BY eq.question_order
	`, versionID, examID); err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO exam_version_question_options (
			exam_version_question_id, option_order, content_snapshot, is_correct_snapshot
		)
		SELECT evq.id, qbo.option_order, qbo.content, qbo.is_correct
		FROM exam_version_questions evq
		JOIN question_bank_options qbo ON qbo.question_id = evq.source_question_id
		WHERE evq.exam_version_id = $1
		ORDER BY evq.question_order, qbo.option_order
	`, versionID); err != nil {
		return 0, err
	}
	return versionID, nil
}

func DeleteExam(ctx context.Context, db *pgxpool.Pool, id string) error {
	examID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil || examID <= 0 {
		return fmt.Errorf("mã bài kiểm tra không hợp lệ")
	}
	var attemptCount int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM exam_attempts WHERE exam_id = $1`, examID).Scan(&attemptCount); err != nil {
		return err
	}
	if attemptCount > 0 {
		commandTag, err := db.Exec(ctx, `
			UPDATE exams
			SET exam_status = 'archived', updated_at = NOW()
			WHERE id = $1
		`, examID)
		if err != nil {
			return err
		}
		if commandTag.RowsAffected() == 0 {
			return fmt.Errorf("không tìm thấy bài kiểm tra")
		}
		return nil
	}
	commandTag, err := db.Exec(ctx, `DELETE FROM exams WHERE id = $1`, examID)
	if err != nil {
		return err
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy bài kiểm tra")
	}
	return nil
}

func GenerateAccessCode(ctx context.Context, db *pgxpool.Pool, id string) (AccessCodeResult, error) {
	examID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil || examID <= 0 {
		return AccessCodeResult{}, fmt.Errorf("ma bai kiem tra khong hop le")
	}
	code, err := randomAccessCode(6)
	if err != nil {
		return AccessCodeResult{}, err
	}
	expiresAt := time.Now().Add(5 * time.Minute)
	commandTag, err := db.Exec(ctx, `
		UPDATE exams
		SET access_code = $2, updated_at = NOW()
		WHERE id = $1 AND exam_mode IN ('official', 'attendance')
	`, examID, fmt.Sprintf("%s:%d", code, expiresAt.Unix()))
	if err != nil {
		return AccessCodeResult{}, err
	}
	if commandTag.RowsAffected() == 0 {
		return AccessCodeResult{}, fmt.Errorf("chi bai chinh thuc hoac diem danh moi can ma truy cap")
	}
	return AccessCodeResult{
		ExamID:         strconv.FormatInt(examID, 10),
		Code:           code,
		ExpiresAt:      expiresAt.Local().Format("02/01/2006 15:04:05"),
		ExpiresAtUnix:  expiresAt.Unix(),
		DurationMinute: 5,
	}, nil
}

func ExportExamScoresXLSX(ctx context.Context, db *pgxpool.Pool, id string) ([]byte, string, error) {
	examID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil || examID <= 0 {
		return nil, "", fmt.Errorf("ma bai kiem tra khong hop le")
	}
	var title string
	if err := db.QueryRow(ctx, `SELECT title FROM exams WHERE id = $1`, examID).Scan(&title); err != nil {
		return nil, "", err
	}

	rows, err := db.Query(ctx, `
		WITH attempts AS (
			SELECT ea.exam_id, ea.student_user_id,
				COUNT(*)::int AS attempt_count,
				COALESCE(MAX(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8 AS best_score,
				MAX(COALESCE(ea.submitted_at, ea.client_last_seen_at, ea.started_at)) AS last_seen,
				BOOL_OR(ea.attempt_status = 'in_progress' AND ea.end_at > NOW()) AS has_in_progress,
				BOOL_OR(ea.attempt_status IN ('submitted', 'expired')) AS has_submitted
			FROM exam_attempts ea
			WHERE ea.exam_id = $1
			GROUP BY ea.exam_id, ea.student_user_id
		)
		SELECT sp.student_code,
			sp.full_name,
			u.username,
			COALESCE(c.class_code, ''),
			COALESCE(a.attempt_count, 0),
			COALESCE(a.best_score, 0)::float8,
			CASE
				WHEN COALESCE(a.has_in_progress, FALSE) THEN 'Đang làm'
				WHEN COALESCE(a.has_submitted, FALSE) THEN 'Đã làm'
				ELSE 'Chưa làm'
			END,
			a.last_seen
		FROM exams e
		JOIN exam_targets et ON et.exam_id = e.id
		JOIN classes c ON c.id = et.class_id
		JOIN class_members cm ON cm.class_id = c.id AND cm.member_status = 'active'
		JOIN users u ON u.id = cm.student_user_id
		JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN attempts a ON a.student_user_id = u.id AND a.exam_id = e.id
		WHERE e.id = $1
		ORDER BY c.class_code, sp.student_code, sp.full_name
	`, examID)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	file := excelize.NewFile()
	defer file.Close()
	sheet := "Danh sach"
	file.SetSheetName("Sheet1", sheet)
	headers := []string{"Mã SV", "Họ tên", "Tài khoản", "Lớp", "Trạng thái", "Số lần làm", "Điểm cao nhất", "Cập nhật gần nhất"}
	for index, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(index+1, 1)
		_ = file.SetCellValue(sheet, cell, header)
	}
	rowIndex := 2
	for rows.Next() {
		var studentCode, name, username, classCode, status string
		var attemptCount int
		var bestScore float64
		var lastSeen *time.Time
		if err := rows.Scan(&studentCode, &name, &username, &classCode, &attemptCount, &bestScore, &status, &lastSeen); err != nil {
			return nil, "", err
		}
		values := []any{studentCode, name, username, classCode, status, attemptCount, scoreText(bestScore), timeText(lastSeen)}
		for col, value := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, rowIndex)
			_ = file.SetCellValue(sheet, cell, value)
		}
		rowIndex++
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}
	for col := 1; col <= len(headers); col++ {
		name, _ := excelize.ColumnNumberToName(col)
		_ = file.SetColWidth(sheet, name, name, 16)
	}
	var buffer bytes.Buffer
	if err := file.Write(&buffer); err != nil {
		return nil, "", err
	}
	return buffer.Bytes(), safeFilename(fmt.Sprintf("bang-diem-%s-%d.xlsx", title, examID)), nil
}

func LiveSnapshot(ctx context.Context, db *pgxpool.Pool, id string) (ExamLiveSnapshot, error) {
	examID, err := strconv.ParseInt(strings.TrimSpace(id), 10, 64)
	if err != nil || examID <= 0 {
		return ExamLiveSnapshot{}, fmt.Errorf("ma bai kiem tra khong hop le")
	}
	rows, err := db.Query(ctx, `
		WITH attempts AS (
			SELECT ea.exam_id, ea.student_user_id,
				COUNT(*)::int AS attempt_count,
				COALESCE(MAX(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8 AS best_score,
				MAX(COALESCE(ea.client_last_seen_at, ea.submitted_at, ea.started_at)) AS last_seen,
				BOOL_OR(ea.attempt_status = 'in_progress' AND ea.end_at > NOW()) AS has_in_progress,
				BOOL_OR(ea.attempt_status IN ('submitted', 'expired')) AS has_submitted
			FROM exam_attempts ea
			WHERE ea.exam_id = $1
			GROUP BY ea.exam_id, ea.student_user_id
		)
		SELECT sp.student_code,
			sp.full_name,
			COALESCE(a.attempt_count, 0),
			COALESCE(a.best_score, 0)::float8,
			CASE
				WHEN COALESCE(a.has_in_progress, FALSE) THEN 'Đang làm'
				WHEN COALESCE(a.has_submitted, FALSE) THEN 'Đã làm'
				ELSE 'Chưa làm'
			END,
			a.last_seen
		FROM exams e
		JOIN exam_targets et ON et.exam_id = e.id
		JOIN class_members cm ON cm.class_id = et.class_id AND cm.member_status = 'active'
		JOIN student_profiles sp ON sp.user_id = cm.student_user_id
		LEFT JOIN attempts a ON a.student_user_id = cm.student_user_id AND a.exam_id = e.id
		WHERE e.id = $1
		ORDER BY
			CASE
				WHEN COALESCE(a.has_in_progress, FALSE) THEN 1
				WHEN COALESCE(a.has_submitted, FALSE) THEN 2
				ELSE 3
			END,
			sp.student_code,
			sp.full_name
	`, examID)
	if err != nil {
		return ExamLiveSnapshot{}, err
	}
	defer rows.Close()

	snapshot := ExamLiveSnapshot{
		ExamID:      strconv.FormatInt(examID, 10),
		GeneratedAt: time.Now().Local().Format("02/01/2006 15:04:05"),
		Rows:        []LiveSnapshotRow{},
	}
	for rows.Next() {
		var item LiveSnapshotRow
		var bestScore float64
		var lastSeen *time.Time
		if err := rows.Scan(&item.StudentCode, &item.Name, &item.AttemptCount, &bestScore, &item.Status, &lastSeen); err != nil {
			return ExamLiveSnapshot{}, err
		}
		item.BestScore = scoreText(bestScore)
		item.LastSeen = timeText(lastSeen)
		snapshot.Rows = append(snapshot.Rows, item)
		snapshot.Total++
		switch item.Status {
		case "Đang làm":
			snapshot.InProgress++
		case "Đã làm":
			snapshot.Submitted++
		default:
			snapshot.NotStarted++
		}
	}
	return snapshot, rows.Err()
}

func ExamDetailByID(ctx context.Context, db *pgxpool.Pool, id string) (ExamDetail, bool, error) {
	examID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return ExamDetail{}, false, nil
	}
	base, ok, err := examBaseByID(ctx, db, examID)
	if err != nil || !ok {
		return ExamDetail{}, ok, err
	}
	students, err := studentAttempts(ctx, db, examID, base.QuestionCnt)
	if err != nil {
		return ExamDetail{}, false, err
	}
	config, err := examConfigByID(ctx, db, examID)
	if err != nil {
		return ExamDetail{}, false, err
	}
	tables := buildTables(students)
	detail := ExamDetail{
		ID:                    strconv.FormatInt(base.ID, 10),
		Title:                 base.Title,
		Description:           config.Description,
		Status:                statusLabel(base.Status),
		StatusCode:            base.Status,
		ExamType:              modeLabel(base.Mode),
		ExamMode:              base.Mode,
		TargetClass:           emptyDash(base.TargetClass),
		ClassID:               config.ClassID,
		StartTime:             formatStart(base.StartTime),
		StartValue:            datetimeLocal(base.StartTime),
		DurationMinutes:       config.DurationMinutes,
		MaxAttemptsPerStudent: config.MaxAttemptsPerStudent,
		ShuffleQuestions:      config.ShuffleQuestions,
		ShuffleOptions:        config.ShuffleOptions,
		ShowResultImmediately: config.ShowResultImmediately,
		AllowReview:           config.AllowReview,
		QuestionSourceID:      config.QuestionSourceID,
		QuestionCount:         base.QuestionCnt,
		CanEdit:               base.AttemptCnt == 0,
		Average:               round1(base.Average),
		Submitted:             base.Submitted,
		Total:                 base.Total,
		Metrics: []Metric{
			{Label: "Đã nộp", Value: fmt.Sprintf("%d/%d", base.Submitted, base.Total)},
			{Label: "Điểm cao nhất", Value: scoreText(base.Highest)},
			{Label: "Điểm trung bình", Value: scoreText(base.Average)},
			{Label: "Số lượt làm", Value: strconv.Itoa(base.AttemptCnt)},
		},
		Tables:   tables,
		Students: students,
	}
	return detail, true, nil
}

func profileFor(ctx context.Context, db *pgxpool.Pool, account string) (Profile, error) {
	if account == "" {
		account = "gv-cntt-01"
	}
	var profile Profile
	err := db.QueryRow(ctx, `
		SELECT COALESCE(tp.full_name, u.username),
			COALESCE(tp.teacher_code, u.username),
			COALESCE(tp.department, ''),
			COALESCE(tp.email, ''),
			COALESCE(tp.phone, '')
		FROM users u
		JOIN teacher_profiles tp ON tp.user_id = u.id
		WHERE u.username = $1
	`, account).Scan(&profile.DisplayName, &profile.TeacherCode, &profile.Department, &profile.Email, &profile.Phone)
	return profile, err
}

func examSummaries(ctx context.Context, db *pgxpool.Pool, account string) ([]ExamSummary, error) {
	rows, err := db.Query(ctx, `
		SELECT e.id, e.title, e.exam_status::text, e.exam_mode::text,
			COALESCE(string_agg(DISTINCT c.class_code, ', '), ''),
			e.start_time,
			COUNT(DISTINCT cm.student_user_id)::int,
			COUNT(DISTINCT ea.student_user_id) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired'))::int,
			COALESCE(AVG(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8
		FROM exams e
		LEFT JOIN exam_targets et ON et.exam_id = e.id
		LEFT JOIN classes c ON c.id = et.class_id
		LEFT JOIN class_members cm ON cm.class_id = c.id AND cm.member_status = 'active'
		LEFT JOIN exam_attempts ea ON ea.exam_id = e.id
		WHERE e.exam_status <> 'archived'
			AND e.created_by_user_id = (SELECT id FROM users WHERE username = $1)
		GROUP BY e.id
		ORDER BY e.start_time DESC NULLS LAST, e.id DESC
	`, strings.TrimSpace(account))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exams := []ExamSummary{}
	for rows.Next() {
		var id int64
		var title, status, mode, target string
		var start *time.Time
		var total, submitted int
		var avg float64
		if err := rows.Scan(&id, &title, &status, &mode, &target, &start, &total, &submitted, &avg); err != nil {
			return nil, err
		}
		status = effectiveExamStatus(status, start, time.Now())
		exams = append(exams, ExamSummary{
			ID:          strconv.FormatInt(id, 10),
			Title:       title,
			Status:      statusLabel(status),
			ExamType:    modeLabel(mode),
			TargetClass: emptyDash(target),
			StartTime:   formatStart(start),
			Average:     round1(avg),
			Submitted:   submitted,
			Total:       total,
		})
	}
	return exams, rows.Err()
}

func examBaseByID(ctx context.Context, db *pgxpool.Pool, examID int64) (examBase, bool, error) {
	var base examBase
	err := db.QueryRow(ctx, `
		SELECT e.id, e.title, e.exam_status::text, e.exam_mode::text,
			COALESCE(string_agg(DISTINCT c.class_code, ', '), ''),
			e.start_time,
			COUNT(DISTINCT eq.id)::int,
			COUNT(DISTINCT cm.student_user_id)::int,
			COUNT(DISTINCT ea.student_user_id) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired'))::int,
			COALESCE(AVG(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8,
			COALESCE(MAX(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8,
			COUNT(DISTINCT ea.id)::int
		FROM exams e
		LEFT JOIN exam_questions eq ON eq.exam_id = e.id
		LEFT JOIN exam_targets et ON et.exam_id = e.id
		LEFT JOIN classes c ON c.id = et.class_id
		LEFT JOIN class_members cm ON cm.class_id = c.id AND cm.member_status = 'active'
		LEFT JOIN exam_attempts ea ON ea.exam_id = e.id
		WHERE e.id = $1
		GROUP BY e.id
	`, examID).Scan(&base.ID, &base.Title, &base.Status, &base.Mode, &base.TargetClass, &base.StartTime, &base.QuestionCnt, &base.Total, &base.Submitted, &base.Average, &base.Highest, &base.AttemptCnt)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return examBase{}, false, nil
		}
		return examBase{}, false, err
	}
	base.Status = effectiveExamStatus(base.Status, base.StartTime, time.Now())
	return base, true, nil
}

func examConfigByID(ctx context.Context, db *pgxpool.Pool, examID int64) (examConfig, error) {
	var config examConfig
	var sourceID sql.NullInt64
	err := db.QueryRow(ctx, `
		SELECT COALESCE(e.description, ''),
			COALESCE(MIN(et.class_id), 0),
			(e.duration_seconds / 60)::int,
			e.max_attempts_per_student,
			e.shuffle_questions,
			e.shuffle_options,
			e.show_result_immediately,
			e.allow_review,
			MIN(ii.batch_id)
		FROM exams e
		LEFT JOIN exam_targets et ON et.exam_id = e.id
		LEFT JOIN exam_questions eq ON eq.exam_id = e.id
		LEFT JOIN question_bank qb ON qb.id = eq.question_id
		LEFT JOIN import_items ii ON ii.id = qb.import_item_id
		WHERE e.id = $1
		GROUP BY e.id
	`, examID).Scan(
		&config.Description,
		&config.ClassID,
		&config.DurationMinutes,
		&config.MaxAttemptsPerStudent,
		&config.ShuffleQuestions,
		&config.ShuffleOptions,
		&config.ShowResultImmediately,
		&config.AllowReview,
		&sourceID,
	)
	if err != nil {
		return examConfig{}, err
	}
	if sourceID.Valid {
		config.QuestionSourceID = sourceID.Int64
	}
	return config, nil
}

func studentAttempts(ctx context.Context, db *pgxpool.Pool, examID int64, questionCount int) ([]StudentAttemptDetail, error) {
	rows, err := db.Query(ctx, `
		SELECT sp.full_name, sp.student_code, ea.id, ea.attempt_no, ea.attempt_status::text,
			ea.score_final::float8, ea.started_at, ea.submitted_at
		FROM exam_attempts ea
		JOIN student_profiles sp ON sp.user_id = ea.student_user_id
		WHERE ea.exam_id = $1
		ORDER BY sp.full_name, ea.attempt_no
	`, examID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byCode := map[string]*StudentAttemptDetail{}
	order := []string{}
	for rows.Next() {
		var name, code, status string
		var attemptID int64
		var attemptNo int
		var score float64
		var started time.Time
		var submitted *time.Time
		if err := rows.Scan(&name, &code, &attemptID, &attemptNo, &status, &score, &started, &submitted); err != nil {
			return nil, err
		}
		student := byCode[code]
		if student == nil {
			student = &StudentAttemptDetail{Name: name, StudentCode: code, Score: scoreText(score), Warning: attemptStatusLabel(status)}
			byCode[code] = student
			order = append(order, code)
		}
		duration := durationText(started, submitted)
		wrongItems, err := wrongItemsForAttempt(ctx, db, attemptID)
		if err != nil {
			return nil, err
		}
		student.Attempts = append(student.Attempts, AttemptDetail{
			AttemptNo:   attemptNo,
			Score:       scoreText(score),
			Duration:    duration,
			Status:      attemptStatusLabel(status),
			SubmittedAt: timeText(submitted),
			WrongItems:  wrongItems,
		})
		if score > scoreNumber(student.Score) {
			student.Score = scoreText(score)
			student.Duration = duration
			student.Warning = attemptStatusLabel(status)
		}
		student.WrongItems = wrongItems
		student.AttemptCount = len(student.Attempts)
		if questionCount > 0 {
			student.Progress = fmt.Sprintf("%d/%d", questionCount, questionCount)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	students := make([]StudentAttemptDetail, 0, len(order))
	for _, code := range order {
		students = append(students, *byCode[code])
	}
	return students, nil
}

func wrongItemsForAttempt(ctx context.Context, db *pgxpool.Pool, attemptID int64) ([]WrongItem, error) {
	rows, err := db.Query(ctx, `
		SELECT aq.question_order,
			aq.content_snapshot,
			COALESCE(NULLIF(string_agg(DISTINCT selected.content_snapshot, ', '), ''), NULLIF(sa.answer_text, ''), 'Chưa chọn'),
			COALESCE(NULLIF(string_agg(DISTINCT correct.content_snapshot, ', '), ''), 'Chưa có đáp án snapshot'),
			COALESCE(NULLIF(aq.explanation_snapshot, ''), 'Câu này được ghi nhận là sai trong lần làm này.')
		FROM attempt_questions aq
		LEFT JOIN student_answers sa ON sa.attempt_question_id = aq.id
		LEFT JOIN student_answer_options sao ON sao.student_answer_id = sa.id
		LEFT JOIN attempt_question_options selected ON selected.id = sao.attempt_question_option_id
		LEFT JOIN attempt_question_options correct ON correct.attempt_question_id = aq.id AND correct.is_correct_snapshot = TRUE
		WHERE aq.attempt_id = $1 AND COALESCE(sa.is_correct, FALSE) = FALSE
		GROUP BY aq.id, sa.id
		ORDER BY aq.question_order
		LIMIT 20
	`, attemptID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []WrongItem{}
	for rows.Next() {
		var order int
		var question, selected, correct, note string
		if err := rows.Scan(&order, &question, &selected, &correct, &note); err != nil {
			return nil, err
		}
		items = append(items, WrongItem{
			Question: fmt.Sprintf("Câu %d: %s", order, question),
			Selected: selected,
			Correct:  correct,
			Note:     note,
		})
	}
	return items, rows.Err()
}

func buildTables(students []StudentAttemptDetail) map[string]StatisticsTable {
	top := append([]StudentAttemptDetail(nil), students...)
	sort.SliceStable(top, func(i, j int) bool { return scoreNumber(top[i].Score) > scoreNumber(top[j].Score) })
	topRows := [][]string{}
	for _, student := range top {
		if math.IsNaN(scoreNumber(student.Score)) {
			continue
		}
		topRows = append(topRows, []string{student.Name, student.StudentCode, strconv.Itoa(student.AttemptCount), student.Score, student.Warning})
	}
	return map[string]StatisticsTable{
		"top_students": {
			Title:   "Sinh viên làm tốt nhất",
			Columns: []string{"Sinh viên", "Mã SV", "Số lần", "Điểm cao nhất", "Trạng thái"},
			Rows:    topRows,
		},
		"score_distribution": scoreDistribution(students),
		"question_difficulty": {
			Title:   "Câu dễ sai nhất",
			Columns: []string{"Câu", "Tỷ lệ sai", "Ghi chú"},
			Rows:    [][]string{},
		},
		"live_status": {
			Title:   "Trạng thái phòng thi",
			Columns: []string{"Sinh viên", "Mã SV", "Số lần", "Tiến trình", "Cảnh báo"},
			Rows:    liveRows(students),
		},
	}
}

func scoreDistribution(students []StudentAttemptDetail) StatisticsTable {
	type bucket struct {
		label string
		min   float64
		max   float64
	}
	buckets := []bucket{{"9 - 10", 9, 10.01}, {"7 - 8.9", 7, 9}, {"5 - 6.9", 5, 7}, {"Dưới 5", 0, 5}}
	total := 0
	scores := []float64{}
	for _, student := range students {
		score := scoreNumber(student.Score)
		if math.IsNaN(score) {
			continue
		}
		total++
		scores = append(scores, score)
	}
	rows := [][]string{}
	for _, bucket := range buckets {
		count := 0
		for _, score := range scores {
			if score >= bucket.min && score < bucket.max {
				count++
			}
		}
		rate := "0.0%"
		if total > 0 {
			rate = fmt.Sprintf("%.1f%%", float64(count)*100/float64(total))
		}
		rows = append(rows, []string{bucket.label, strconv.Itoa(count), rate})
	}
	return StatisticsTable{Title: "Phân bố điểm", Columns: []string{"Khoảng điểm", "Số sinh viên", "Tỷ lệ"}, Rows: rows}
}

func liveRows(students []StudentAttemptDetail) [][]string {
	rows := make([][]string, 0, len(students))
	for _, student := range students {
		rows = append(rows, []string{student.Name, student.StudentCode, strconv.Itoa(student.AttemptCount), student.Progress, student.Warning})
	}
	return rows
}

func statusLabel(status string) string {
	switch status {
	case "open":
		return "Đang mở"
	case "scheduled":
		return "Lịch dự kiến"
	case "closed":
		return "Đã đóng"
	case "draft":
		return "Bản nháp"
	case "archived":
		return "Đã xoá"
	default:
		return status
	}
}

func modeLabel(mode string) string {
	switch mode {
	case "official":
		return "Chính thức"
	case "attendance":
		return "Điểm danh"
	default:
		return "Thi thử"
	}
}

func attemptStatusLabel(status string) string {
	switch status {
	case "submitted", "expired":
		return "Đã nộp"
	case "in_progress":
		return "Đang làm"
	default:
		return status
	}
}

func formatStart(value *time.Time) string {
	if value == nil {
		return "Chưa đặt giờ"
	}
	local := value.In(appLocation)
	now := time.Now().In(appLocation)
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return "Hôm nay " + local.Format("15:04")
	}
	return local.Format("02/01 - 15:04")
}

func datetimeLocal(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.In(appLocation).Format("2006-01-02T15:04")
}

func timeText(value *time.Time) string {
	if value == nil {
		return "Chưa nộp"
	}
	return value.In(appLocation).Format("02/01/2006 15:04")
}

func durationText(start time.Time, submitted *time.Time) string {
	end := time.Now()
	if submitted != nil {
		end = *submitted
	}
	minutes := int(end.Sub(start).Minutes())
	if minutes < 0 {
		minutes = 0
	}
	return fmt.Sprintf("%d phút", minutes)
}

func scoreText(score float64) string {
	return strconv.FormatFloat(round1(score), 'f', -1, 64)
}

func scoreNumber(value string) float64 {
	score, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
	if err != nil {
		return math.NaN()
	}
	return score
}

func round1(value float64) float64 {
	return math.Round(value*10) / 10
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func uniquePositiveIDs(values []int64) []int64 {
	seen := map[int64]bool{}
	out := []int64{}
	for _, value := range values {
		if value <= 0 || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func questionIDsFromSource(ctx context.Context, tx pgx.Tx, sourceID int64, requestedCount int, randomize bool, creatorID any) ([]int64, error) {
	if sourceID <= 0 {
		return nil, fmt.Errorf("cần chọn bộ đề cương")
	}
	var available int
	if err := tx.QueryRow(ctx, `
		SELECT COUNT(qb.id)::int
		FROM import_batches ib
		JOIN import_items ii ON ii.batch_id = ib.id
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ib.id = $1
			AND ($2::bigint IS NULL OR ib.uploaded_by_user_id = $2 OR ib.uploaded_by_user_id IS NULL)
	`, sourceID, creatorID).Scan(&available); err != nil {
		return nil, err
	}
	if available == 0 {
		return nil, fmt.Errorf("bộ đề cương chưa có câu hỏi active hoặc không thuộc tài khoản này")
	}
	if requestedCount <= 0 {
		requestedCount = available
	}
	if requestedCount > available {
		return nil, fmt.Errorf("bộ đề cương chỉ có %d câu active", available)
	}

	orderClause := "ii.item_order, qb.id"
	if randomize {
		orderClause = "random()"
	}
	rows, err := tx.Query(ctx, fmt.Sprintf(`
		SELECT qb.id
		FROM import_batches ib
		JOIN import_items ii ON ii.batch_id = ib.id
		JOIN question_bank qb ON qb.import_item_id = ii.id AND qb.question_status = 'active'
		WHERE ib.id = $1
			AND ($2::bigint IS NULL OR ib.uploaded_by_user_id = $2 OR ib.uploaded_by_user_id IS NULL)
		ORDER BY %s
		LIMIT $3
	`, orderClause), sourceID, creatorID, requestedCount)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func examModeValue(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "attendance" || strings.Contains(normalized, "điểm") || strings.Contains(normalized, "diem") {
		return "attendance"
	}
	if normalized == "official" || strings.Contains(normalized, "chính") || strings.Contains(normalized, "chinh") {
		return "official"
	}
	return "practice"
}

func randomAccessCode(length int) (string, error) {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		n, err := crand.Int(crand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}

func safeFilename(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "export.xlsx"
	}
	replacer := strings.NewReplacer(
		"\\", "-", "/", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", " ", "-",
	)
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	if len(value) > 120 {
		value = value[:120]
	}
	if !strings.HasSuffix(strings.ToLower(value), ".xlsx") {
		value += ".xlsx"
	}
	return value
}

func parseCreateStart(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return &parsed, nil
	}
	layouts := []string{
		"2006-01-02T15:04",
		"2006-01-02 15:04",
	}
	var lastErr error
	for _, layout := range layouts {
		parsed, err := time.ParseInLocation(layout, value, appLocation)
		if err == nil {
			return &parsed, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("thời gian mở bài không hợp lệ: %v", lastErr)
}
