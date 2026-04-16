package teacherdata

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Dashboard struct {
	Profile Profile       `json:"profile"`
	Exams   []ExamSummary `json:"exams"`
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
	ID          string                     `json:"id"`
	Title       string                     `json:"title"`
	Status      string                     `json:"status"`
	ExamType    string                     `json:"examType"`
	TargetClass string                     `json:"targetClass"`
	StartTime   string                     `json:"startTime"`
	Average     float64                    `json:"average"`
	Submitted   int                        `json:"submitted"`
	Total       int                        `json:"total"`
	Metrics     []Metric                   `json:"metrics"`
	Tables      map[string]StatisticsTable `json:"tables"`
	Students    []StudentAttemptDetail     `json:"students"`
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

func DashboardFor(ctx context.Context, db *pgxpool.Pool, account string) (Dashboard, error) {
	profile, err := profileFor(ctx, db, account)
	if err != nil {
		return Dashboard{}, err
	}
	exams, err := examSummaries(ctx, db)
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
	tables := buildTables(students)
	detail := ExamDetail{
		ID:          strconv.FormatInt(base.ID, 10),
		Title:       base.Title,
		Status:      statusLabel(base.Status),
		ExamType:    modeLabel(base.Mode),
		TargetClass: emptyDash(base.TargetClass),
		StartTime:   formatStart(base.StartTime),
		Average:     round1(base.Average),
		Submitted:   base.Submitted,
		Total:       base.Total,
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

func examSummaries(ctx context.Context, db *pgxpool.Pool) ([]ExamSummary, error) {
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
		GROUP BY e.id
		ORDER BY e.start_time DESC NULLS LAST, e.id DESC
	`)
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
	return base, true, nil
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
	default:
		return status
	}
}

func modeLabel(mode string) string {
	if mode == "official" {
		return "Chính thức"
	}
	return "Thi thử"
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
	local := value.Local()
	now := time.Now()
	if local.Year() == now.Year() && local.YearDay() == now.YearDay() {
		return "Hôm nay " + local.Format("15:04")
	}
	return local.Format("02/01 - 15:04")
}

func timeText(value *time.Time) string {
	if value == nil {
		return "Chưa nộp"
	}
	return value.Local().Format("02/01/2006 15:04")
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
