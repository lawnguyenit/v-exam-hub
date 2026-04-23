package studentdata

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Dashboard struct {
	Profile        Profile         `json:"profile"`
	Summary        Summary         `json:"summary"`
	AvailableExams []ExamSummary   `json:"availableExams"`
	PlannedExams   []PlannedExam   `json:"plannedExams"`
	History        []HistoryRecord `json:"history"`
}

type Profile struct {
	DisplayName string `json:"displayName"`
	ClassName   string `json:"className"`
	Email       string `json:"email"`
	Status      string `json:"status"`
}

type Summary struct {
	AvailableCount int    `json:"availableCount"`
	PlannedCount   int    `json:"plannedCount"`
	LatestScore    string `json:"latestScore"`
}

type ExamSummary struct {
	ID       string `json:"id"`
	Status   string `json:"status"`
	Title    string `json:"title"`
	Meta     string `json:"meta"`
	Duration string `json:"duration"`
}

type PlannedExam struct {
	Time   string `json:"time"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

type HistoryRecord struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Date     string `json:"date"`
	Score    string `json:"score"`
	Duration string `json:"duration"`
}

type Exam struct {
	ID                 string     `json:"id"`
	Title              string     `json:"title"`
	DurationSeconds    int        `json:"durationSeconds"`
	ExamMode           string     `json:"examMode"`
	RequiresAccessCode bool       `json:"requiresAccessCode"`
	Questions          []Question `json:"questions"`
}

type Question struct {
	Title        string   `json:"title"`
	Answers      []string `json:"answers"`
	AssetBatchID int64    `json:"assetBatchId,omitempty"`
}

type Review struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Score     string           `json:"score"`
	Duration  string           `json:"duration"`
	Questions []ReviewQuestion `json:"questions"`
}

type ReviewQuestion struct {
	Title          string   `json:"title"`
	Answers        []string `json:"answers"`
	CorrectAnswer  int      `json:"correctAnswer"`
	SelectedAnswer int      `json:"selectedAnswer"`
	AssetBatchID   int64    `json:"assetBatchId,omitempty"`
}

type AttemptStartRequest struct {
	Account    string `json:"account"`
	ExamID     string `json:"examId"`
	AccessCode string `json:"accessCode"`
}

type AttemptAnswerRequest struct {
	AttemptID     int64 `json:"attemptId"`
	QuestionIndex int   `json:"questionIndex"`
	AnswerIndex   int   `json:"answerIndex"`
}

type AttemptProgressRequest struct {
	AttemptID     int64 `json:"attemptId"`
	QuestionIndex int   `json:"questionIndex"`
}

type AttemptSyncRequest struct {
	AttemptID       int64          `json:"attemptId"`
	CurrentQuestion int            `json:"currentQuestion"`
	Answers         map[string]int `json:"answers"`
}

type AttemptSubmitRequest struct {
	AttemptID int64 `json:"attemptId"`
}

type AttemptState struct {
	AttemptID       int64          `json:"attemptId"`
	ExamID          string         `json:"examId"`
	StartedAt       int64          `json:"startedAt"`
	EndAt           int64          `json:"endAt"`
	CurrentQuestion int            `json:"currentQuestion"`
	Answers         map[string]int `json:"answers"`
	Status          string         `json:"status"`
	Score           string         `json:"score,omitempty"`
	LastSavedAt     int64          `json:"lastSavedAt"`
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
	if account == "" {
		account = "student"
	}
	profile, userID, err := profileFor(ctx, db, account)
	if err != nil {
		return Dashboard{}, err
	}
	available, planned, err := examsForStudent(ctx, db, userID)
	if err != nil {
		return Dashboard{}, err
	}
	history, latest, err := historyForStudent(ctx, db, userID)
	if err != nil {
		return Dashboard{}, err
	}
	return Dashboard{
		Profile:        profile,
		Summary:        Summary{AvailableCount: len(available), PlannedCount: len(planned), LatestScore: latest},
		AvailableExams: available,
		PlannedExams:   planned,
		History:        history,
	}, nil
}

func StartAttempt(ctx context.Context, db *pgxpool.Pool, payload AttemptStartRequest) (AttemptState, error) {
	examID, err := strconv.ParseInt(payload.ExamID, 10, 64)
	if err != nil || examID <= 0 {
		return AttemptState{}, fmt.Errorf("mã bài thi không hợp lệ")
	}
	studentID, err := studentUserID(ctx, db, payload.Account)
	if err != nil {
		return AttemptState{}, err
	}

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AttemptState{}, err
	}
	defer tx.Rollback(ctx)

	var attemptID int64
	err = tx.QueryRow(ctx, `
		SELECT id
		FROM exam_attempts ea
		WHERE exam_id = $1
			AND student_user_id = $2
			AND attempt_status = 'in_progress'
			AND end_at > NOW()
			AND EXISTS (SELECT 1 FROM attempt_questions aq WHERE aq.attempt_id = ea.id)
		ORDER BY attempt_no DESC
		LIMIT 1
	`, examID, studentID).Scan(&attemptID)
	if err == nil {
		if _, err := tx.Exec(ctx, `UPDATE exam_attempts SET client_last_seen_at = NOW() WHERE id = $1`, attemptID); err != nil {
			return AttemptState{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return AttemptState{}, err
		}
		return attemptState(ctx, db, attemptID)
	}
	if !strings.Contains(err.Error(), "no rows") {
		return AttemptState{}, err
	}

	var durationSeconds int
	var maxAttempts int
	var totalPoints float64
	var status string
	var mode string
	var storedAccessCode string
	var start *time.Time
	if err := tx.QueryRow(ctx, `
		SELECT duration_seconds, max_attempts_per_student, total_points::float8,
			exam_status::text, exam_mode::text, COALESCE(access_code, ''), start_time
		FROM exams
		WHERE id = $1
	`, examID).Scan(&durationSeconds, &maxAttempts, &totalPoints, &status, &mode, &storedAccessCode, &start); err != nil {
		return AttemptState{}, err
	}
	status = effectiveExamStatus(status, start, time.Now())
	if status != "open" {
		return AttemptState{}, fmt.Errorf("bài thi chưa mở")
	}
	if requiresAccessCode(mode) {
		ok, err := studentBelongsToExam(ctx, tx, examID, studentID)
		if err != nil {
			return AttemptState{}, err
		}
		if !ok {
			return AttemptState{}, fmt.Errorf("tài khoản này không nằm trong danh sách lớp được phép làm bài")
		}
		if !validExamAccessCode(storedAccessCode, payload.AccessCode, time.Now()) {
			return AttemptState{}, fmt.Errorf("bài này cần mã truy cập hợp lệ")
		}
	}
	var usedAttempts int
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM exam_attempts WHERE exam_id = $1 AND student_user_id = $2 AND attempt_status <> 'cancelled'`, examID, studentID).Scan(&usedAttempts); err != nil {
		return AttemptState{}, err
	}
	if maxAttempts > 0 && usedAttempts >= maxAttempts {
		return AttemptState{}, fmt.Errorf("sinh viên đã dùng hết số lần làm bài")
	}
	versionID, err := ensureExamVersion(ctx, tx, examID)
	if err != nil {
		return AttemptState{}, err
	}
	if totalPoints <= 0 {
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(points_snapshot), 0)::float8 FROM exam_version_questions WHERE exam_version_id = $1`, versionID).Scan(&totalPoints); err != nil {
			return AttemptState{}, err
		}
	}
	if totalPoints <= 0 {
		totalPoints = 10
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO exam_attempts (
			exam_id, exam_version_id, student_user_id, attempt_no,
			end_at, duration_seconds_snapshot, total_points_snapshot,
			current_question_order, client_last_seen_at
		)
		VALUES ($1, $2, $3, $4, NOW() + make_interval(secs => $5), $5, $6, 1, NOW())
		RETURNING id
	`, examID, versionID, studentID, usedAttempts+1, durationSeconds, totalPoints).Scan(&attemptID); err != nil {
		return AttemptState{}, err
	}
	if err := snapshotAttemptQuestions(ctx, tx, attemptID, versionID); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attempt_events (attempt_id, event_type) VALUES ($1, 'start')`, attemptID); err != nil {
		return AttemptState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptState{}, err
	}
	return attemptState(ctx, db, attemptID)
}

func SaveAnswer(ctx context.Context, db *pgxpool.Pool, payload AttemptAnswerRequest) (AttemptState, error) {
	if payload.AttemptID <= 0 || payload.QuestionIndex < 0 || payload.AnswerIndex < 0 {
		return AttemptState{}, fmt.Errorf("dữ liệu đáp án không hợp lệ")
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AttemptState{}, err
	}
	defer tx.Rollback(ctx)

	var attemptStatus string
	var endAt time.Time
	if err := tx.QueryRow(ctx, `SELECT attempt_status::text, end_at FROM exam_attempts WHERE id = $1`, payload.AttemptID).Scan(&attemptStatus, &endAt); err != nil {
		return AttemptState{}, err
	}
	if attemptStatus != "in_progress" || time.Now().After(endAt) {
		if _, err := tx.Exec(ctx, `UPDATE exam_attempts SET attempt_status = 'expired', updated_at = NOW() WHERE id = $1 AND attempt_status = 'in_progress'`, payload.AttemptID); err != nil {
			return AttemptState{}, err
		}
		return AttemptState{}, fmt.Errorf("bài làm đã hết thời gian hoặc đã nộp")
	}

	if err := saveAnswerInTx(ctx, tx, payload.AttemptID, payload.QuestionIndex, payload.AnswerIndex); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE exam_attempts
		SET current_question_order = $2,
			last_saved_at = NOW(),
			client_last_seen_at = NOW()
		WHERE id = $1
	`, payload.AttemptID, payload.QuestionIndex+1); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attempt_events (attempt_id, event_type) VALUES ($1, 'answer_saved')`, payload.AttemptID); err != nil {
		return AttemptState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptState{}, err
	}
	return attemptState(ctx, db, payload.AttemptID)
}

func SyncAttempt(ctx context.Context, db *pgxpool.Pool, payload AttemptSyncRequest) (AttemptState, error) {
	if payload.AttemptID <= 0 || payload.CurrentQuestion < 0 {
		return AttemptState{}, fmt.Errorf("dữ liệu đồng bộ không hợp lệ")
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AttemptState{}, err
	}
	defer tx.Rollback(ctx)

	var attemptStatus string
	var endAt time.Time
	if err := tx.QueryRow(ctx, `SELECT attempt_status::text, end_at FROM exam_attempts WHERE id = $1`, payload.AttemptID).Scan(&attemptStatus, &endAt); err != nil {
		return AttemptState{}, err
	}
	if attemptStatus != "in_progress" || time.Now().After(endAt) {
		if _, err := tx.Exec(ctx, `UPDATE exam_attempts SET attempt_status = 'expired', updated_at = NOW() WHERE id = $1 AND attempt_status = 'in_progress'`, payload.AttemptID); err != nil {
			return AttemptState{}, err
		}
		return AttemptState{}, fmt.Errorf("bài làm đã hết thời gian hoặc đã nộp")
	}

	for questionIndexText, answerIndex := range payload.Answers {
		questionIndex, err := strconv.Atoi(questionIndexText)
		if err != nil || questionIndex < 0 || answerIndex < 0 {
			return AttemptState{}, fmt.Errorf("dữ liệu đáp án không hợp lệ")
		}
		if err := saveAnswerInTx(ctx, tx, payload.AttemptID, questionIndex, answerIndex); err != nil {
			return AttemptState{}, err
		}
	}

	if _, err := tx.Exec(ctx, `
		UPDATE exam_attempts
		SET current_question_order = $2,
			last_saved_at = NOW(),
			client_last_seen_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
	`, payload.AttemptID, payload.CurrentQuestion+1); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attempt_events (attempt_id, event_type) VALUES ($1, 'answer_saved')`, payload.AttemptID); err != nil {
		return AttemptState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptState{}, err
	}
	return attemptState(ctx, db, payload.AttemptID)
}

func UpdateProgress(ctx context.Context, db *pgxpool.Pool, payload AttemptProgressRequest) (AttemptState, error) {
	if payload.AttemptID <= 0 || payload.QuestionIndex < 0 {
		return AttemptState{}, fmt.Errorf("dữ liệu tiến trình không hợp lệ")
	}
	result, err := db.Exec(ctx, `
		UPDATE exam_attempts
		SET current_question_order = $2,
			client_last_seen_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND attempt_status = 'in_progress'
			AND end_at > NOW()
	`, payload.AttemptID, payload.QuestionIndex+1)
	if err != nil {
		return AttemptState{}, err
	}
	if result.RowsAffected() == 0 {
		return AttemptState{}, fmt.Errorf("bài làm đã hết thời gian hoặc đã nộp")
	}
	return attemptState(ctx, db, payload.AttemptID)
}

func saveAnswerInTx(ctx context.Context, tx pgx.Tx, attemptID int64, questionIndex int, answerIndex int) error {
	var attemptQuestionID int64
	if err := tx.QueryRow(ctx, `
		SELECT id FROM attempt_questions WHERE attempt_id = $1 AND question_order = $2
	`, attemptID, questionIndex+1).Scan(&attemptQuestionID); err != nil {
		return err
	}
	var optionID int64
	var isCorrect bool
	if err := tx.QueryRow(ctx, `
		SELECT id, is_correct_snapshot
		FROM attempt_question_options
		WHERE attempt_question_id = $1 AND option_order = $2
	`, attemptQuestionID, answerIndex+1).Scan(&optionID, &isCorrect); err != nil {
		return err
	}
	var answerID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO student_answers (attempt_question_id, is_correct, score_awarded, saved_at)
		VALUES ($1, $2, CASE WHEN $2 THEN (SELECT points_snapshot FROM attempt_questions WHERE id = $1) ELSE 0 END, NOW())
		ON CONFLICT (attempt_question_id) DO UPDATE
		SET is_correct = EXCLUDED.is_correct,
			score_awarded = EXCLUDED.score_awarded,
			saved_at = NOW(),
			updated_at = NOW()
		RETURNING id
	`, attemptQuestionID, isCorrect).Scan(&answerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM student_answer_options WHERE student_answer_id = $1`, answerID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO student_answer_options (student_answer_id, attempt_question_option_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING
	`, answerID, optionID); err != nil {
		return err
	}
	return nil
}

func SubmitAttempt(ctx context.Context, db *pgxpool.Pool, payload AttemptSubmitRequest) (AttemptState, error) {
	if payload.AttemptID <= 0 {
		return AttemptState{}, fmt.Errorf("thiếu mã lần làm")
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return AttemptState{}, err
	}
	defer tx.Rollback(ctx)

	var score float64
	if err := tx.QueryRow(ctx, `
		SELECT COALESCE(SUM(sa.score_awarded), 0)::float8
		FROM attempt_questions aq
		LEFT JOIN student_answers sa ON sa.attempt_question_id = aq.id
		WHERE aq.attempt_id = $1
	`, payload.AttemptID).Scan(&score); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE exam_attempts
		SET attempt_status = 'submitted',
			submitted_at = NOW(),
			score_raw = $2,
			score_final = $2,
			submit_source = 'manual',
			client_last_seen_at = NOW()
		WHERE id = $1 AND attempt_status = 'in_progress'
	`, payload.AttemptID, score); err != nil {
		return AttemptState{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO attempt_events (attempt_id, event_type) VALUES ($1, 'submit')`, payload.AttemptID); err != nil {
		return AttemptState{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return AttemptState{}, err
	}
	return attemptState(ctx, db, payload.AttemptID)
}

func ExamByID(ctx context.Context, db *pgxpool.Pool, id string) (Exam, bool, error) {
	examID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Exam{}, false, nil
	}
	var exam Exam
	err = db.QueryRow(ctx, `SELECT id::text, title, duration_seconds, exam_mode::text FROM exams WHERE id = $1`, examID).Scan(&exam.ID, &exam.Title, &exam.DurationSeconds, &exam.ExamMode)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Exam{}, false, nil
		}
		return Exam{}, false, err
	}
	exam.RequiresAccessCode = requiresAccessCode(exam.ExamMode)
	rows, err := db.Query(ctx, `
		SELECT qb.content,
			array_agg(qbo.content ORDER BY qbo.option_order),
			COALESCE(ii.batch_id, 0)
		FROM exam_questions eq
		JOIN question_bank qb ON qb.id = eq.question_id
		LEFT JOIN import_items ii ON ii.id = qb.import_item_id
		LEFT JOIN question_bank_options qbo ON qbo.question_id = qb.id
		WHERE eq.exam_id = $1
		GROUP BY eq.id, qb.content, ii.batch_id
		ORDER BY eq.question_order
	`, examID)
	if err != nil {
		return Exam{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var question Question
		if err := rows.Scan(&question.Title, &question.Answers, &question.AssetBatchID); err != nil {
			return Exam{}, false, err
		}
		exam.Questions = append(exam.Questions, question)
	}
	return exam, true, rows.Err()
}

func ReviewByID(ctx context.Context, db *pgxpool.Pool, id string) (Review, bool, error) {
	attemptID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return Review{}, false, nil
	}
	var review Review
	var started time.Time
	var submitted *time.Time
	err = db.QueryRow(ctx, `
		SELECT ea.id::text, e.title, ea.score_final::text, ea.started_at, ea.submitted_at
		FROM exam_attempts ea
		JOIN exams e ON e.id = ea.exam_id
		WHERE ea.id = $1
	`, attemptID).Scan(&review.ID, &review.Title, &review.Score, &started, &submitted)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return Review{}, false, nil
		}
		return Review{}, false, err
	}
	review.Duration = durationText(started, submitted)
	rows, err := db.Query(ctx, `
		SELECT aq.content_snapshot,
			array_agg(aqo.content_snapshot ORDER BY aqo.option_order),
			COALESCE(MAX(aqo.option_order) FILTER (WHERE aqo.is_correct_snapshot), 1),
			COALESCE(MAX(aqo.option_order) FILTER (WHERE sao.id IS NOT NULL), 1),
			COALESCE(ii.batch_id, 0)
		FROM attempt_questions aq
		LEFT JOIN exam_version_questions evq ON evq.id = aq.exam_version_question_id
		LEFT JOIN question_bank qb ON qb.id = evq.source_question_id
		LEFT JOIN import_items ii ON ii.id = qb.import_item_id
		LEFT JOIN attempt_question_options aqo ON aqo.attempt_question_id = aq.id
		LEFT JOIN student_answers sa ON sa.attempt_question_id = aq.id
		LEFT JOIN student_answer_options sao ON sao.student_answer_id = sa.id AND sao.attempt_question_option_id = aqo.id
		WHERE aq.attempt_id = $1
		GROUP BY aq.id, ii.batch_id
		ORDER BY aq.question_order
	`, attemptID)
	if err != nil {
		return Review{}, false, err
	}
	defer rows.Close()
	for rows.Next() {
		var question ReviewQuestion
		var correct, selected int
		if err := rows.Scan(&question.Title, &question.Answers, &correct, &selected, &question.AssetBatchID); err != nil {
			return Review{}, false, err
		}
		question.CorrectAnswer = max(0, correct-1)
		question.SelectedAnswer = max(0, selected-1)
		review.Questions = append(review.Questions, question)
	}
	return review, true, rows.Err()
}

func studentUserID(ctx context.Context, db *pgxpool.Pool, account string) (int64, error) {
	var userID int64
	if err := db.QueryRow(ctx, `
		SELECT u.id
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		WHERE u.username = $1 AND r.code = 'student' AND u.account_status = 'active'
	`, account).Scan(&userID); err != nil {
		return 0, fmt.Errorf("không tìm thấy tài khoản sinh viên hợp lệ")
	}
	return userID, nil
}

func ensureExamVersion(ctx context.Context, tx pgx.Tx, examID int64) (int64, error) {
	var versionID int64
	err := tx.QueryRow(ctx, `
		SELECT id FROM exam_versions WHERE exam_id = $1 ORDER BY version_no DESC LIMIT 1
	`, examID).Scan(&versionID)
	if err == nil {
		return versionID, populateExamVersion(ctx, tx, examID, versionID)
	}
	if !strings.Contains(err.Error(), "no rows") {
		return 0, err
	}
	if err := tx.QueryRow(ctx, `
		INSERT INTO exam_versions (
			exam_id, version_no, title_snapshot, description_snapshot,
			exam_mode_snapshot, duration_seconds_snapshot, total_points_snapshot,
			exam_status_snapshot, shuffle_questions_snapshot, shuffle_options_snapshot,
			show_result_immediately_snapshot, allow_review_snapshot,
			start_time_snapshot, end_time_snapshot, published_by_user_id
		)
		SELECT id, 1, title, description, exam_mode, duration_seconds,
			CASE WHEN total_points > 0 THEN total_points ELSE GREATEST((SELECT COUNT(*) FROM exam_questions WHERE exam_id = exams.id), 1) END,
			exam_status, shuffle_questions, shuffle_options, show_result_immediately,
			allow_review, start_time, end_time, created_by_user_id
		FROM exams
		WHERE id = $1
		RETURNING id
	`, examID).Scan(&versionID); err != nil {
		return 0, err
	}
	return versionID, populateExamVersion(ctx, tx, examID, versionID)
}

func populateExamVersion(ctx context.Context, tx pgx.Tx, examID int64, versionID int64) error {
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
		ON CONFLICT (exam_version_id, question_order) DO NOTHING
	`, versionID, examID); err != nil {
		return err
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
		ON CONFLICT (exam_version_question_id, option_order) DO NOTHING
	`, versionID); err != nil {
		return err
	}
	return nil
}

func snapshotAttemptQuestions(ctx context.Context, tx pgx.Tx, attemptID int64, versionID int64) error {
	var shuffleQuestions, shuffleOptions bool
	if err := tx.QueryRow(ctx, `
		SELECT shuffle_questions_snapshot, shuffle_options_snapshot
		FROM exam_versions
		WHERE id = $1
	`, versionID).Scan(&shuffleQuestions, &shuffleOptions); err != nil {
		return err
	}

	questionSelect := `
		INSERT INTO attempt_questions (
			attempt_id, exam_version_question_id, question_order,
			question_type_snapshot, content_snapshot, explanation_snapshot, points_snapshot
		)
		SELECT $1, id, question_order, question_type_snapshot, content_snapshot, explanation_snapshot, points_snapshot
		FROM exam_version_questions
		WHERE exam_version_id = $2
		ORDER BY question_order
		RETURNING id, exam_version_question_id
	`
	if shuffleQuestions {
		questionSelect = `
			INSERT INTO attempt_questions (
				attempt_id, exam_version_question_id, question_order,
				question_type_snapshot, content_snapshot, explanation_snapshot, points_snapshot
			)
			SELECT $1, id, ROW_NUMBER() OVER (ORDER BY random())::int,
				question_type_snapshot, content_snapshot, explanation_snapshot, points_snapshot
			FROM exam_version_questions
			WHERE exam_version_id = $2
			RETURNING id, exam_version_question_id
		`
	}

	rows, err := tx.Query(ctx, questionSelect, attemptID, versionID)
	if err != nil {
		return err
	}
	defer rows.Close()
	questionIDs := map[int64]int64{}
	for rows.Next() {
		var attemptQuestionID, versionQuestionID int64
		if err := rows.Scan(&attemptQuestionID, &versionQuestionID); err != nil {
			return err
		}
		questionIDs[versionQuestionID] = attemptQuestionID
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for versionQuestionID, attemptQuestionID := range questionIDs {
		optionSelect := `
			INSERT INTO attempt_question_options (
				attempt_question_id, option_order, content_snapshot, is_correct_snapshot
			)
			SELECT $1, option_order, content_snapshot, is_correct_snapshot
			FROM exam_version_question_options
			WHERE exam_version_question_id = $2
			ORDER BY option_order
		`
		if shuffleOptions {
			optionSelect = `
				INSERT INTO attempt_question_options (
					attempt_question_id, option_order, content_snapshot, is_correct_snapshot
				)
				SELECT $1, ROW_NUMBER() OVER (ORDER BY random())::int, content_snapshot, is_correct_snapshot
				FROM exam_version_question_options
				WHERE exam_version_question_id = $2
			`
		}
		if _, err := tx.Exec(ctx, optionSelect, attemptQuestionID, versionQuestionID); err != nil {
			return err
		}
	}
	return nil
}

func attemptState(ctx context.Context, db *pgxpool.Pool, attemptID int64) (AttemptState, error) {
	var state AttemptState
	var startedAt, endAt time.Time
	var status string
	var currentQuestion int
	var score string
	var lastSavedAt *time.Time
	if err := db.QueryRow(ctx, `
		SELECT id, exam_id::text, started_at, end_at, COALESCE(current_question_order, 1),
			attempt_status::text, score_final::text, last_saved_at
		FROM exam_attempts
		WHERE id = $1
	`, attemptID).Scan(&state.AttemptID, &state.ExamID, &startedAt, &endAt, &currentQuestion, &status, &score, &lastSavedAt); err != nil {
		return AttemptState{}, err
	}
	state.StartedAt = startedAt.UnixMilli()
	state.EndAt = endAt.UnixMilli()
	state.CurrentQuestion = max(0, currentQuestion-1)
	state.Status = status
	state.Score = score
	if lastSavedAt != nil {
		state.LastSavedAt = lastSavedAt.UnixMilli()
	}
	state.Answers = map[string]int{}
	rows, err := db.Query(ctx, `
		SELECT aq.question_order, aqo.option_order
		FROM attempt_questions aq
		JOIN student_answers sa ON sa.attempt_question_id = aq.id
		JOIN student_answer_options sao ON sao.student_answer_id = sa.id
		JOIN attempt_question_options aqo ON aqo.id = sao.attempt_question_option_id
		WHERE aq.attempt_id = $1
	`, attemptID)
	if err != nil {
		return AttemptState{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var questionOrder, optionOrder int
		if err := rows.Scan(&questionOrder, &optionOrder); err != nil {
			return AttemptState{}, err
		}
		state.Answers[strconv.Itoa(questionOrder-1)] = optionOrder - 1
	}
	return state, rows.Err()
}

func profileFor(ctx context.Context, db *pgxpool.Pool, account string) (Profile, int64, error) {
	var profile Profile
	var userID int64
	err := db.QueryRow(ctx, `
		SELECT u.id, sp.full_name, COALESCE(string_agg(DISTINCT c.class_name, ', '), ''),
			COALESCE(sp.email, ''), sp.profile_status::text
		FROM users u
		JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN class_members cm ON cm.student_user_id = u.id AND cm.member_status = 'active'
		LEFT JOIN classes c ON c.id = cm.class_id
		WHERE u.username = $1
		GROUP BY u.id, sp.full_name, sp.email, sp.profile_status
	`, account).Scan(&userID, &profile.DisplayName, &profile.ClassName, &profile.Email, &profile.Status)
	return profile, userID, err
}

func examsForStudent(ctx context.Context, db *pgxpool.Pool, userID int64) ([]ExamSummary, []PlannedExam, error) {
	rows, err := db.Query(ctx, `
		SELECT e.id::text, e.title, e.exam_status::text, e.exam_mode::text, e.duration_seconds, e.start_time
		FROM exams e
		WHERE EXISTS (
			SELECT 1
			FROM exam_targets et
			JOIN class_members cm ON cm.class_id = et.class_id
			WHERE et.exam_id = e.id
				AND cm.student_user_id = $1
				AND cm.member_status = 'active'
		)
		ORDER BY e.start_time NULLS LAST, e.id
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	available := []ExamSummary{}
	planned := []PlannedExam{}
	for rows.Next() {
		var id, title, status, mode string
		var seconds int
		var start *time.Time
		if err := rows.Scan(&id, &title, &status, &mode, &seconds, &start); err != nil {
			return nil, nil, err
		}
		status = effectiveExamStatus(status, start, time.Now())
		if status == "open" {
			available = append(available, ExamSummary{ID: id, Status: modeLabel(mode), Title: title, Meta: "Đang mở cho lớp của bạn.", Duration: fmt.Sprintf("%d phút", seconds/60)})
		}
		if status == "scheduled" {
			planned = append(planned, PlannedExam{Time: formatStart(start), Title: title, Detail: "Chờ đến giờ mở bài"})
		}
	}
	return available, planned, rows.Err()
}

func historyForStudent(ctx context.Context, db *pgxpool.Pool, userID int64) ([]HistoryRecord, string, error) {
	rows, err := db.Query(ctx, `
		SELECT ea.id::text, e.title, COALESCE(ea.submitted_at, ea.started_at), ea.score_final::text, ea.started_at, ea.submitted_at
		FROM exam_attempts ea
		JOIN exams e ON e.id = ea.exam_id
		WHERE ea.student_user_id = $1 AND ea.attempt_status IN ('submitted', 'expired')
		ORDER BY COALESCE(ea.submitted_at, ea.started_at) DESC
	`, userID)
	if err != nil {
		return nil, "0", err
	}
	defer rows.Close()
	history := []HistoryRecord{}
	latest := "0"
	for rows.Next() {
		var record HistoryRecord
		var when time.Time
		var started time.Time
		var submitted *time.Time
		if err := rows.Scan(&record.ID, &record.Title, &when, &record.Score, &started, &submitted); err != nil {
			return nil, "0", err
		}
		record.Date = when.In(appLocation).Format("02/01/2006")
		record.Duration = durationText(started, submitted)
		if len(history) == 0 {
			latest = record.Score
		}
		history = append(history, record)
	}
	return history, latest, rows.Err()
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

func formatStart(value *time.Time) string {
	if value == nil {
		return "Chưa đặt giờ"
	}
	return value.In(appLocation).Format("02/01 - 15:04")
}

func requiresAccessCode(mode string) bool {
	return mode == "official" || mode == "attendance"
}

func studentBelongsToExam(ctx context.Context, tx pgx.Tx, examID int64, studentID int64) (bool, error) {
	var exists bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM exam_targets et
			JOIN class_members cm ON cm.class_id = et.class_id
			WHERE et.exam_id = $1
				AND cm.student_user_id = $2
				AND cm.member_status = 'active'
		)
	`, examID, studentID).Scan(&exists)
	return exists, err
}

func validExamAccessCode(stored string, input string, now time.Time) bool {
	parts := strings.Split(strings.TrimSpace(stored), ":")
	if len(parts) != 2 {
		return false
	}
	code := strings.ToUpper(strings.TrimSpace(parts[0]))
	expiresUnix, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil || now.Unix() > expiresUnix {
		return false
	}
	return code != "" && code == strings.ToUpper(strings.TrimSpace(input))
}

func effectiveExamStatus(status string, start *time.Time, now time.Time) string {
	if status == "scheduled" && start != nil && !start.After(now) {
		return "open"
	}
	return status
}

func modeLabel(mode string) string {
	if mode == "official" {
		return "Chính thức"
	}
	if mode == "attendance" {
		return "Điểm danh"
	}
	return "Thi thử"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
