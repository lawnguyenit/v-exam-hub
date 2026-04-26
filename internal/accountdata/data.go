package accountdata

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"mime/multipart"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

type LoginResult struct {
	Username    string `json:"username"`
	Role        string `json:"role"`
	DisplayName string `json:"displayName"`
}

type StudentImportRequest struct {
	ClassCode string `json:"classCode"`
	ClassName string `json:"className"`
	Rows      string `json:"rows"`
}

type StudentPasswordUpdateRequest struct {
	Username    string `json:"username"`
	StudentCode string `json:"studentCode"`
	Password    string `json:"password"`
}

type TeacherCreateRequest struct {
	AdminUsername string `json:"adminUsername"`
	Username      string `json:"username"`
	Password      string `json:"password"`
	TeacherCode   string `json:"teacherCode"`
	FullName      string `json:"fullName"`
	Email         string `json:"email"`
	Phone         string `json:"phone"`
	Department    string `json:"department"`
}

type TeacherCreateResult struct {
	Username          string `json:"username"`
	TeacherCode       string `json:"teacherCode"`
	FullName          string `json:"fullName"`
	Email             string `json:"email"`
	Department        string `json:"department"`
	TemporaryPassword string `json:"temporaryPassword"`
	Created           bool   `json:"created"`
}

type ClassSummary struct {
	ID          int64  `json:"id"`
	ClassCode   string `json:"classCode"`
	ClassName   string `json:"className"`
	MemberCount int    `json:"memberCount"`
	ExamCount   int    `json:"examCount"`
}

type ClassDetail struct {
	ID           int64              `json:"id"`
	ClassCode    string             `json:"classCode"`
	ClassName    string             `json:"className"`
	MemberCount  int                `json:"memberCount"`
	ExamCount    int                `json:"examCount"`
	AverageScore string             `json:"averageScore"`
	Members      []ClassMember      `json:"members"`
	Exams        []ClassExamSummary `json:"exams"`
}

type ClassMember struct {
	UserID       int64  `json:"userId"`
	Username     string `json:"username"`
	StudentCode  string `json:"studentCode"`
	FullName     string `json:"fullName"`
	Email        string `json:"email"`
	Phone        string `json:"phone"`
	AttemptCount int    `json:"attemptCount"`
	BestScore    string `json:"bestScore"`
	LastSeen     string `json:"lastSeen"`
}

type ClassExamSummary struct {
	ID        int64  `json:"id"`
	Title     string `json:"title"`
	Status    string `json:"status"`
	Submitted int    `json:"submitted"`
	Total     int    `json:"total"`
	Average   string `json:"average"`
}

type ClassUpdateRequest struct {
	ClassCode string `json:"classCode"`
	ClassName string `json:"className"`
}

type StudentImportResult struct {
	ClassCode          string                  `json:"classCode"`
	ClassName          string                  `json:"className"`
	Created            int                     `json:"created"`
	Updated            int                     `json:"updated"`
	AddedToClass       int                     `json:"addedToClass"`
	Skipped            int                     `json:"skipped"`
	ImportedStudents   []ImportedStudent       `json:"importedStudents"`
	GeneratedPasswords []GeneratedPasswordRow  `json:"generatedPasswords"`
	Errors             []string                `json:"errors"`
	RowErrors          []StudentImportRowError `json:"rowErrors"`
}

type ImportedStudent struct {
	SourceRow         int    `json:"sourceRow"`
	Username          string `json:"username"`
	StudentCode       string `json:"studentCode"`
	FullName          string `json:"fullName"`
	TemporaryPassword string `json:"temporaryPassword"`
}

type GeneratedPasswordRow struct {
	SourceRow   int    `json:"sourceRow"`
	Username    string `json:"username"`
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
	Password    string `json:"password"`
}

type StudentImportRowError struct {
	SourceRow   int    `json:"sourceRow"`
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Message     string `json:"message"`
}

type studentRow struct {
	SourceRow int
	Code      string
	FullName  string
	Email     string
	Phone     string
	Username  string
	Password  string
}

func Authenticate(ctx context.Context, db *pgxpool.Pool, payload LoginRequest) (LoginResult, error) {
	username := strings.TrimSpace(payload.Username)
	password := payload.Password
	role := strings.TrimSpace(payload.Role)
	if username == "" || password == "" || role == "" {
		return LoginResult{}, fmt.Errorf("thiếu tài khoản, mật khẩu hoặc vai trò")
	}

	var userID int64
	var storedHash, displayName string
	err := db.QueryRow(ctx, `
		SELECT u.id, u.password_hash,
			COALESCE(sp.full_name, tp.full_name, u.username)
		FROM users u
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN teacher_profiles tp ON tp.user_id = u.id
		WHERE u.username = $1 AND u.account_status = 'active'
		LIMIT 1
	`, username).Scan(&userID, &storedHash, &displayName)
	if err != nil {
		return LoginResult{}, fmt.Errorf("tài khoản không tồn tại hoặc không đúng vai trò")
	}
	if !passwordMatches(storedHash, password) {
		return LoginResult{}, fmt.Errorf("mật khẩu không đúng")
	}
	rows, err := db.Query(ctx, `
		SELECT r.code
		FROM user_roles ur
		JOIN roles r ON r.id = ur.role_id
		WHERE ur.user_id = $1
		ORDER BY r.code
	`, userID)
	if err != nil {
		return LoginResult{}, err
	}
	defer rows.Close()

	roles := map[string]bool{}
	for rows.Next() {
		var code string
		if err := rows.Scan(&code); err != nil {
			return LoginResult{}, err
		}
		roles[code] = true
	}
	if err := rows.Err(); err != nil {
		return LoginResult{}, err
	}
	if roles["admin"] {
		return LoginResult{Username: username, Role: "admin", DisplayName: displayName}, nil
	}
	if !roles[role] {
		return LoginResult{}, fmt.Errorf("tài khoản không đúng vai trò")
	}
	return LoginResult{Username: username, Role: role, DisplayName: displayName}, nil
}

func ListClasses(ctx context.Context, db *pgxpool.Pool, teacherUserID int64) ([]ClassSummary, error) {
	rows, err := db.Query(ctx, `
		SELECT c.id, c.class_code, c.class_name,
			COUNT(DISTINCT cm.student_user_id) FILTER (WHERE cm.member_status = 'active')::int AS member_count,
			COUNT(DISTINCT e.id) FILTER (WHERE e.created_by_user_id = $1)::int AS exam_count
		FROM classes c
		LEFT JOIN teacher_class_assignments tca
			ON tca.class_id = c.id
			AND tca.teacher_user_id = $1
			AND tca.assignment_status = 'active'
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN exam_targets et ON et.class_id = c.id
		LEFT JOIN exams e ON e.id = et.exam_id
		WHERE c.class_status = 'active'
			AND (c.created_by_user_id = $1 OR c.homeroom_teacher_user_id = $1 OR tca.teacher_user_id IS NOT NULL)
		GROUP BY c.id, c.class_code, c.class_name
		ORDER BY c.class_code, c.class_name
	`, teacherUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	classes := []ClassSummary{}
	for rows.Next() {
		var class ClassSummary
		if err := rows.Scan(&class.ID, &class.ClassCode, &class.ClassName, &class.MemberCount, &class.ExamCount); err != nil {
			return nil, err
		}
		classes = append(classes, class)
	}
	return classes, rows.Err()
}

func ClassDetailByID(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64) (ClassDetail, error) {
	if teacherUserID <= 0 || classID <= 0 {
		return ClassDetail{}, fmt.Errorf("lớp không hợp lệ")
	}
	var detail ClassDetail
	if err := db.QueryRow(ctx, `
		SELECT c.id, c.class_code, c.class_name,
			COUNT(DISTINCT cm.student_user_id) FILTER (WHERE cm.member_status = 'active')::int,
			COUNT(DISTINCT e.id) FILTER (WHERE e.created_by_user_id = $1)::int,
			COALESCE(ROUND(AVG(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 2)::text, '--')
		FROM classes c
		LEFT JOIN teacher_class_assignments tca
			ON tca.class_id = c.id
			AND tca.teacher_user_id = $1
			AND tca.assignment_status = 'active'
		LEFT JOIN class_members cm ON cm.class_id = c.id
		LEFT JOIN exam_targets et ON et.class_id = c.id
		LEFT JOIN exams e ON e.id = et.exam_id AND e.created_by_user_id = $1
		LEFT JOIN exam_attempts ea ON ea.exam_id = e.id
		WHERE c.id = $2
			AND c.class_status = 'active'
			AND (c.created_by_user_id = $1 OR c.homeroom_teacher_user_id = $1 OR tca.teacher_user_id IS NOT NULL)
		GROUP BY c.id, c.class_code, c.class_name
	`, teacherUserID, classID).Scan(&detail.ID, &detail.ClassCode, &detail.ClassName, &detail.MemberCount, &detail.ExamCount, &detail.AverageScore); err != nil {
		return ClassDetail{}, err
	}
	members, err := classMembers(ctx, db, teacherUserID, classID)
	if err != nil {
		return ClassDetail{}, err
	}
	exams, err := classExams(ctx, db, teacherUserID, classID)
	if err != nil {
		return ClassDetail{}, err
	}
	detail.Members = members
	detail.Exams = exams
	return detail, nil
}

func UpdateClass(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64, payload ClassUpdateRequest) (ClassSummary, error) {
	classCode := strings.TrimSpace(payload.ClassCode)
	className := strings.TrimSpace(payload.ClassName)
	if classCode == "" || className == "" {
		return ClassSummary{}, fmt.Errorf("thiếu mã lớp hoặc tên lớp")
	}
	if ok, err := teacherCanUseClass(ctx, db, teacherUserID, classID); err != nil || !ok {
		if err != nil {
			return ClassSummary{}, err
		}
		return ClassSummary{}, fmt.Errorf("không có quyền sửa lớp này")
	}
	var summary ClassSummary
	if err := db.QueryRow(ctx, `
		UPDATE classes
		SET class_code = $1, class_name = $2, updated_at = NOW()
		WHERE id = $3 AND class_status = 'active'
		RETURNING id, class_code, class_name
	`, classCode, className, classID).Scan(&summary.ID, &summary.ClassCode, &summary.ClassName); err != nil {
		return ClassSummary{}, err
	}
	return summary, nil
}

func ArchiveClass(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64) error {
	if ok, err := teacherCanUseClass(ctx, db, teacherUserID, classID); err != nil || !ok {
		if err != nil {
			return err
		}
		return fmt.Errorf("không có quyền xóa lớp này")
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE class_members SET member_status = 'inactive', left_at = COALESCE(left_at, NOW()) WHERE class_id = $1 AND member_status = 'active'`, classID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE teacher_class_assignments SET assignment_status = 'inactive' WHERE class_id = $1 AND teacher_user_id = $2`, classID, teacherUserID); err != nil {
		return err
	}
	command, err := tx.Exec(ctx, `UPDATE classes SET class_status = 'archived', updated_at = NOW() WHERE id = $1 AND class_status = 'active'`, classID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy lớp cần xóa")
	}
	return tx.Commit(ctx)
}

func RemoveClassMember(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64, studentUserID int64) error {
	if ok, err := teacherCanUseClass(ctx, db, teacherUserID, classID); err != nil || !ok {
		if err != nil {
			return err
		}
		return fmt.Errorf("không có quyền sửa lớp này")
	}
	command, err := db.Exec(ctx, `
		UPDATE class_members
		SET member_status = 'inactive', left_at = COALESCE(left_at, NOW()), updated_at = NOW()
		WHERE class_id = $1 AND student_user_id = $2 AND member_status = 'active'
	`, classID, studentUserID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy sinh viên trong lớp")
	}
	return nil
}

func ImportStudentsFromXLSX(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classCode, className, filename string, content []byte) (StudentImportResult, error) {
	rows, err := parseXLSXRows(content)
	if err != nil {
		return StudentImportResult{}, err
	}
	if len(rows) == 0 {
		return StudentImportResult{}, fmt.Errorf("file %s không có dữ liệu sinh viên", filename)
	}
	return importStudentRows(ctx, db, teacherUserID, classCode, className, rows)
}

func ImportStudents(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, payload StudentImportRequest) (StudentImportResult, error) {
	rows := parseManualRows(payload.Rows)
	return importStudentRows(ctx, db, teacherUserID, payload.ClassCode, payload.ClassName, rows)
}

func classMembers(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64) ([]ClassMember, error) {
	rows, err := db.Query(ctx, `
		WITH teacher_exams AS (
			SELECT e.id
			FROM exams e
			JOIN exam_targets et ON et.exam_id = e.id
			WHERE e.created_by_user_id = $1 AND et.class_id = $2
		),
		attempts AS (
			SELECT ea.student_user_id,
				COUNT(*)::int AS attempt_count,
				COALESCE(MAX(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 0)::float8 AS best_score,
				MAX(COALESCE(ea.submitted_at, ea.client_last_seen_at, ea.started_at)) AS last_seen
			FROM exam_attempts ea
			JOIN teacher_exams te ON te.id = ea.exam_id
			GROUP BY ea.student_user_id
		)
		SELECT u.id, u.username, sp.student_code, sp.full_name,
			COALESCE(sp.email, ''), COALESCE(sp.phone, ''),
			COALESCE(a.attempt_count, 0),
			COALESCE(a.best_score, 0)::float8,
			a.last_seen
		FROM class_members cm
		JOIN users u ON u.id = cm.student_user_id
		JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN attempts a ON a.student_user_id = u.id
		WHERE cm.class_id = $2 AND cm.member_status = 'active'
		ORDER BY sp.student_code, sp.full_name
	`, teacherUserID, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []ClassMember{}
	for rows.Next() {
		var member ClassMember
		var bestScore float64
		var lastSeen *time.Time
		if err := rows.Scan(&member.UserID, &member.Username, &member.StudentCode, &member.FullName, &member.Email, &member.Phone, &member.AttemptCount, &bestScore, &lastSeen); err != nil {
			return nil, err
		}
		member.BestScore = importScoreText(bestScore)
		member.LastSeen = importTimeText(lastSeen)
		members = append(members, member)
	}
	return members, rows.Err()
}

func classExams(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64) ([]ClassExamSummary, error) {
	rows, err := db.Query(ctx, `
		SELECT e.id, e.title, e.exam_status::text,
			COUNT(DISTINCT ea.student_user_id) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired'))::int AS submitted,
			COUNT(DISTINCT cm.student_user_id) FILTER (WHERE cm.member_status = 'active')::int AS total,
			COALESCE(ROUND(AVG(ea.score_final) FILTER (WHERE ea.attempt_status IN ('submitted', 'expired')), 2)::text, '--') AS average
		FROM exams e
		JOIN exam_targets et ON et.exam_id = e.id
		LEFT JOIN class_members cm ON cm.class_id = et.class_id
		LEFT JOIN exam_attempts ea ON ea.exam_id = e.id AND ea.student_user_id = cm.student_user_id
		WHERE e.created_by_user_id = $1 AND et.class_id = $2
		GROUP BY e.id, e.title, e.exam_status
		ORDER BY e.created_at DESC
	`, teacherUserID, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	exams := []ClassExamSummary{}
	for rows.Next() {
		var exam ClassExamSummary
		if err := rows.Scan(&exam.ID, &exam.Title, &exam.Status, &exam.Submitted, &exam.Total, &exam.Average); err != nil {
			return nil, err
		}
		exams = append(exams, exam)
	}
	return exams, rows.Err()
}

func teacherCanUseClass(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classID int64) (bool, error) {
	var ok bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM classes c
			LEFT JOIN teacher_class_assignments tca
				ON tca.class_id = c.id
				AND tca.teacher_user_id = $1
				AND tca.assignment_status = 'active'
			WHERE c.id = $2
				AND c.class_status = 'active'
				AND (c.created_by_user_id = $1 OR c.homeroom_teacher_user_id = $1 OR tca.teacher_user_id IS NOT NULL)
		)
	`, teacherUserID, classID).Scan(&ok)
	return ok, err
}

func importScoreText(value float64) string {
	if value == 0 {
		return "--"
	}
	return fmt.Sprintf("%.2f", value)
}

func importTimeText(value *time.Time) string {
	if value == nil {
		return "--"
	}
	return value.Local().Format("02/01/2006 15:04")
}

func UpdateStudentPassword(ctx context.Context, db *pgxpool.Pool, payload StudentPasswordUpdateRequest) error {
	password := strings.TrimSpace(payload.Password)
	if password == "" {
		return fmt.Errorf("mật khẩu mới không được trống")
	}
	command, err := db.Exec(ctx, `
		UPDATE users u
		SET password_hash = $1, updated_at = NOW()
		FROM student_profiles sp
		WHERE sp.user_id = u.id
			AND ($2 = '' OR u.username = $2)
			AND ($3 = '' OR sp.student_code = $3)
	`, HashPassword(password), strings.TrimSpace(payload.Username), strings.TrimSpace(payload.StudentCode))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy sinh viên cần đổi mật khẩu")
	}
	return nil
}

func CreateTeacherAccount(ctx context.Context, db *pgxpool.Pool, payload TeacherCreateRequest) (TeacherCreateResult, error) {
	adminUsername := strings.TrimSpace(payload.AdminUsername)
	if adminUsername == "" {
		return TeacherCreateResult{}, fmt.Errorf("missing admin account")
	}
	var isAdmin bool
	if err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM users u
			JOIN user_roles ur ON ur.user_id = u.id
			JOIN roles r ON r.id = ur.role_id
			WHERE u.username = $1 AND r.code = 'admin' AND u.account_status = 'active'
		)
	`, adminUsername).Scan(&isAdmin); err != nil {
		return TeacherCreateResult{}, err
	}
	if !isAdmin {
		return TeacherCreateResult{}, fmt.Errorf("account is not allowed to create teacher users")
	}

	fullName := strings.TrimSpace(payload.FullName)
	if fullName == "" {
		return TeacherCreateResult{}, fmt.Errorf("missing teacher full name")
	}
	email := strings.TrimSpace(payload.Email)
	phone := strings.TrimSpace(payload.Phone)
	department := strings.TrimSpace(payload.Department)

	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return TeacherCreateResult{}, err
	}
	defer tx.Rollback(ctx)

	roleID, err := ensureRole(ctx, tx, "teacher", "Teacher")
	if err != nil {
		return TeacherCreateResult{}, err
	}

	teacherCode := strings.TrimSpace(payload.TeacherCode)
	if teacherCode == "" {
		teacherCode = teacherCodeFromName(fullName)
	}
	teacherCode, err = uniqueTeacherCode(ctx, tx, teacherCode)
	if err != nil {
		return TeacherCreateResult{}, err
	}
	username := strings.TrimSpace(payload.Username)
	if username == "" {
		username = teacherCode
	}
	password := strings.TrimSpace(payload.Password)
	if password == "" {
		password = username
	}
	passwordHash := HashPassword(password)

	var userID int64
	var created bool
	if err := tx.QueryRow(ctx, `
		WITH upsert_user AS (
			INSERT INTO users (username, password_hash)
			VALUES ($1, $2)
			ON CONFLICT (username) DO UPDATE
			SET password_hash = EXCLUDED.password_hash,
				account_status = 'active',
				updated_at = NOW()
			RETURNING id, xmax = 0 AS inserted
		)
		SELECT id, inserted FROM upsert_user
	`, username, passwordHash).Scan(&userID, &created); err != nil {
		return TeacherCreateResult{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID); err != nil {
		return TeacherCreateResult{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO teacher_profiles (user_id, teacher_code, full_name, email, phone, department)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''))
		ON CONFLICT (teacher_code) DO UPDATE
		SET user_id = EXCLUDED.user_id,
			full_name = EXCLUDED.full_name,
			email = EXCLUDED.email,
			phone = EXCLUDED.phone,
			department = EXCLUDED.department,
			profile_status = 'active',
			updated_at = NOW()
	`, userID, teacherCode, fullName, email, phone, department); err != nil {
		return TeacherCreateResult{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return TeacherCreateResult{}, err
	}
	return TeacherCreateResult{
		Username:          username,
		TeacherCode:       teacherCode,
		FullName:          fullName,
		Email:             email,
		Department:        department,
		TemporaryPassword: password,
		Created:           created,
	}, nil
}

func uniqueTeacherCode(ctx context.Context, tx pgx.Tx, base string) (string, error) {
	base = cleanIdentifier(base)
	if base == "" {
		base = "gv"
	}
	if len(base) > 32 {
		base = base[:32]
	}
	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s%d", base, i+1)
		}
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM users WHERE username = $1
				UNION ALL
				SELECT 1 FROM teacher_profiles WHERE teacher_code = $1
			)
		`, candidate).Scan(&exists); err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("cannot create unique teacher code")
}

func teacherCodeFromName(fullName string) string {
	words := strings.Fields(toASCIILower(fullName))
	cleaned := make([]string, 0, len(words))
	for _, word := range words {
		word = cleanIdentifier(word)
		if word != "" {
			cleaned = append(cleaned, word)
		}
	}
	if len(cleaned) == 0 {
		return "gv"
	}
	code := cleaned[len(cleaned)-1]
	for _, word := range cleaned[:len(cleaned)-1] {
		code += string(word[0])
	}
	return code
}

func cleanIdentifier(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func toASCIILower(value string) string {
	replacer := strings.NewReplacer(
		"à", "a", "á", "a", "ạ", "a", "ả", "a", "ã", "a", "â", "a", "ầ", "a", "ấ", "a", "ậ", "a", "ẩ", "a", "ẫ", "a", "ă", "a", "ằ", "a", "ắ", "a", "ặ", "a", "ẳ", "a", "ẵ", "a",
		"è", "e", "é", "e", "ẹ", "e", "ẻ", "e", "ẽ", "e", "ê", "e", "ề", "e", "ế", "e", "ệ", "e", "ể", "e", "ễ", "e",
		"ì", "i", "í", "i", "ị", "i", "ỉ", "i", "ĩ", "i",
		"ò", "o", "ó", "o", "ọ", "o", "ỏ", "o", "õ", "o", "ô", "o", "ồ", "o", "ố", "o", "ộ", "o", "ổ", "o", "ỗ", "o", "ơ", "o", "ờ", "o", "ớ", "o", "ợ", "o", "ở", "o", "ỡ", "o",
		"ù", "u", "ú", "u", "ụ", "u", "ủ", "u", "ũ", "u", "ư", "u", "ừ", "u", "ứ", "u", "ự", "u", "ử", "u", "ữ", "u",
		"ỳ", "y", "ý", "y", "ỵ", "y", "ỷ", "y", "ỹ", "y",
		"đ", "d",
	)
	return replacer.Replace(strings.ToLower(value))
}

func importStudentRows(ctx context.Context, db *pgxpool.Pool, teacherUserID int64, classCode, className string, rows []studentRow) (StudentImportResult, error) {
	classCode = strings.TrimSpace(classCode)
	className = strings.TrimSpace(className)
	if classCode == "" || className == "" {
		return StudentImportResult{}, fmt.Errorf("thiếu mã lớp hoặc tên lớp")
	}
	if teacherUserID <= 0 {
		return StudentImportResult{}, fmt.Errorf("thiếu giáo viên sở hữu lớp")
	}
	result := StudentImportResult{
		ClassCode:          classCode,
		ClassName:          className,
		ImportedStudents:   []ImportedStudent{},
		GeneratedPasswords: []GeneratedPasswordRow{},
		Errors:             []string{},
		RowErrors:          []StudentImportRowError{},
	}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var classID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO classes (class_code, class_name, homeroom_teacher_user_id, created_by_user_id)
		VALUES ($1, $2, $3, $3)
		ON CONFLICT (class_code) DO UPDATE SET
			class_name = EXCLUDED.class_name,
			homeroom_teacher_user_id = COALESCE(classes.homeroom_teacher_user_id, EXCLUDED.homeroom_teacher_user_id),
			created_by_user_id = COALESCE(classes.created_by_user_id, EXCLUDED.created_by_user_id),
			class_status = 'active',
			updated_at = NOW()
		RETURNING id
	`, classCode, className, teacherUserID).Scan(&classID); err != nil {
		return result, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO teacher_class_assignments (class_id, teacher_user_id, permission, assignment_status, assigned_by_user_id)
		VALUES ($1, $2, 'owner', 'active', $2)
		ON CONFLICT (class_id, teacher_user_id) DO UPDATE
		SET permission = EXCLUDED.permission,
			assignment_status = 'active',
			updated_at = NOW()
	`, classID, teacherUserID); err != nil {
		return result, err
	}

	roleID, err := ensureRole(ctx, tx, "student", "Sinh viên")
	if err != nil {
		return result, err
	}

	for index, row := range rows {
		row.normalize()
		sourceRow := row.SourceRow
		if sourceRow <= 0 {
			sourceRow = index + 1
		}
		if row.Code == "" || row.FullName == "" {
			message := "thiếu mã sinh viên hoặc họ tên"
			result.Errors = append(result.Errors, fmt.Sprintf("Dòng %d %s", sourceRow, message))
			result.RowErrors = append(result.RowErrors, StudentImportRowError{
				SourceRow:   sourceRow,
				StudentCode: row.Code,
				FullName:    row.FullName,
				Email:       row.Email,
				Phone:       row.Phone,
				Username:    row.Username,
				Password:    row.Password,
				Message:     message,
			})
			result.Skipped++
			continue
		}
		if row.Username == "" {
			row.Username = row.Code
		}
		if row.Password == "" {
			row.Password = row.Code
		}
		passwordHash := HashPassword(row.Password)

		var userID int64
		var inserted bool
		if err := tx.QueryRow(ctx, `
			WITH upsert_user AS (
				INSERT INTO users (username, password_hash)
				VALUES ($1, $2)
				ON CONFLICT (username) DO UPDATE SET updated_at = NOW()
				RETURNING id, xmax = 0 AS inserted
			)
			SELECT id, inserted FROM upsert_user
		`, row.Username, passwordHash).Scan(&userID, &inserted); err != nil {
			return result, err
		}
		if inserted {
			result.Created++
		} else {
			result.Updated++
		}
		if _, err := tx.Exec(ctx, `INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`, userID, roleID); err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO student_profiles (user_id, student_code, full_name, email, phone)
			VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, ''))
			ON CONFLICT (student_code) DO UPDATE
			SET full_name = EXCLUDED.full_name,
				email = EXCLUDED.email,
				phone = EXCLUDED.phone,
				updated_at = NOW()
		`, userID, row.Code, row.FullName, row.Email, row.Phone); err != nil {
			return result, err
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO class_members (class_id, student_user_id)
			VALUES ($1, $2)
			ON CONFLICT (class_id, student_user_id) DO UPDATE SET member_status = 'active', updated_at = NOW()
		`, classID, userID); err != nil {
			return result, err
		}
		result.AddedToClass++
		imported := ImportedStudent{SourceRow: sourceRow, Username: row.Username, StudentCode: row.Code, FullName: row.FullName, TemporaryPassword: row.Password}
		result.ImportedStudents = append(result.ImportedStudents, imported)
		result.GeneratedPasswords = append(result.GeneratedPasswords, GeneratedPasswordRow{SourceRow: sourceRow, Username: row.Username, StudentCode: row.Code, FullName: row.FullName, Password: row.Password})
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func ensureRole(ctx context.Context, tx pgx.Tx, code, name string) (int64, error) {
	var id int64
	err := tx.QueryRow(ctx, `
		INSERT INTO roles (code, name)
		VALUES ($1, $2)
		ON CONFLICT (code) DO UPDATE SET name = EXCLUDED.name
		RETURNING id
	`, code, name).Scan(&id)
	return id, err
}

func parseManualRows(source string) []studentRow {
	rows := []studentRow{}
	for lineIndex, raw := range strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		if lineIndex == 0 && (strings.Contains(lower, "mã") || strings.Contains(lower, "ma sv")) {
			continue
		}
		separator := ","
		if strings.Contains(line, "\t") {
			separator = "\t"
		}
		parts := strings.Split(line, separator)
		for len(parts) < 6 {
			parts = append(parts, "")
		}
		rows = append(rows, studentRow{SourceRow: lineIndex + 1, Code: parts[0], FullName: parts[1], Email: parts[2], Phone: parts[3], Username: parts[4], Password: parts[5]})
	}
	return rows
}

func parseXLSXRows(content []byte) ([]studentRow, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("XLSX không đọc được: %w", err)
	}
	shared, _ := readSharedStrings(reader)
	sheetXML, err := readFirstWorksheet(reader)
	if err != nil {
		return nil, err
	}
	values, err := parseSheet(sheetXML, shared)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, nil
	}
	header, headerRowIndex, hasHeader := findStudentHeader(values)
	if !hasHeader {
		rows := []studentRow{}
		for rowIndex, row := range values {
			if rowIsEmpty(row) {
				continue
			}
			student := studentRow{
				SourceRow: rowIndex + 1,
				Code:      cellAt(row, 0),
				FullName:  cellAt(row, 1),
				Email:     cellAt(row, 2),
				Phone:     cellAt(row, 3),
				Username:  cellAt(row, 4),
				Password:  cellAt(row, 5),
			}
			if student.hasImportData() {
				rows = append(rows, student)
			}
		}
		return rows, nil
	}
	get := func(row []string, names ...string) string {
		for _, name := range names {
			if index, ok := header[normalizeHeader(name)]; ok && index < len(row) {
				return row[index]
			}
		}
		return ""
	}
	rows := []studentRow{}
	for offset, row := range values[headerRowIndex+1:] {
		if rowIsEmpty(row) {
			continue
		}
		student := studentRow{
			SourceRow: headerRowIndex + offset + 2,
			Code:      get(row, "ma sv", "masv", "mssv", "ma sinh vien", "student code", "student id"),
			FullName:  get(row, "ho ten", "ho va ten", "ten sinh vien", "ho ten sinh vien", "fullname", "name"),
			Email:     get(row, "email"),
			Phone:     get(row, "sdt", "so dien thoai", "dien thoai", "phone", "mobile"),
			Username:  get(row, "tai khoan", "ten dang nhap", "username", "account"),
			Password:  get(row, "mat khau", "password"),
		}
		if student.hasImportData() {
			rows = append(rows, student)
		}
	}
	return rows, nil
}

func findStudentHeader(values [][]string) (map[string]int, int, bool) {
	for rowIndex, row := range values {
		header := map[string]int{}
		for index, cell := range row {
			key := normalizeHeader(cell)
			if key != "" {
				header[key] = index
			}
		}
		if hasAnyHeader(header, "ma sv", "masv", "mssv", "ma sinh vien", "student code", "student id") &&
			hasAnyHeader(header, "ho ten", "ho va ten", "ten sinh vien", "ho ten sinh vien", "fullname", "name") {
			return header, rowIndex, true
		}
	}
	return nil, 0, false
}

func hasAnyHeader(header map[string]int, names ...string) bool {
	for _, name := range names {
		if _, ok := header[normalizeHeader(name)]; ok {
			return true
		}
	}
	return false
}

func rowIsEmpty(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}

func cellAt(row []string, index int) string {
	if index < 0 || index >= len(row) {
		return ""
	}
	return row[index]
}

func readSharedStrings(reader *zip.Reader) ([]string, error) {
	for _, file := range reader.File {
		if file.Name != "xl/sharedStrings.xml" {
			continue
		}
		rc, err := file.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, err
		}
		type textNode struct {
			Text string `xml:",chardata"`
		}
		type item struct {
			Texts []textNode `xml:"t"`
		}
		type sst struct {
			Items []item `xml:"si"`
		}
		var parsed sst
		if err := xml.Unmarshal(data, &parsed); err != nil {
			return nil, err
		}
		out := make([]string, 0, len(parsed.Items))
		for _, item := range parsed.Items {
			parts := []string{}
			for _, text := range item.Texts {
				parts = append(parts, text.Text)
			}
			out = append(out, strings.Join(parts, ""))
		}
		return out, nil
	}
	return nil, nil
}

func readFirstWorksheet(reader *zip.Reader) ([]byte, error) {
	for _, file := range reader.File {
		if strings.HasPrefix(file.Name, "xl/worksheets/sheet") && strings.HasSuffix(file.Name, ".xml") {
			rc, err := file.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return io.ReadAll(rc)
		}
	}
	return nil, fmt.Errorf("XLSX không có worksheet")
}

func parseSheet(data []byte, shared []string) ([][]string, error) {
	type cell struct {
		Ref    string `xml:"r,attr"`
		Type   string `xml:"t,attr"`
		Value  string `xml:"v"`
		Inline string `xml:"is>t"`
	}
	type row struct {
		Cells []cell `xml:"c"`
	}
	type worksheet struct {
		Rows []row `xml:"sheetData>row"`
	}
	var parsed worksheet
	if err := xml.Unmarshal(data, &parsed); err != nil {
		return nil, err
	}
	out := [][]string{}
	for _, row := range parsed.Rows {
		cells := []string{}
		for _, cell := range row.Cells {
			column := columnIndex(cell.Ref)
			for len(cells) <= column {
				cells = append(cells, "")
			}
			value := cell.Value
			if cell.Type == "s" {
				index, _ := strconv.Atoi(value)
				if index >= 0 && index < len(shared) {
					value = shared[index]
				}
			} else if cell.Type == "inlineStr" {
				value = cell.Inline
			}
			cells[column] = strings.TrimSpace(value)
		}
		out = append(out, cells)
	}
	return out, nil
}

func columnIndex(ref string) int {
	n := 0
	for _, char := range ref {
		if char < 'A' || char > 'Z' {
			break
		}
		n = n*26 + int(char-'A'+1)
	}
	if n == 0 {
		return 0
	}
	return n - 1
}

func (row *studentRow) normalize() {
	row.Code = strings.TrimSpace(row.Code)
	row.FullName = strings.TrimSpace(row.FullName)
	row.Email = strings.TrimSpace(row.Email)
	row.Phone = strings.TrimSpace(row.Phone)
	row.Username = strings.TrimSpace(row.Username)
	row.Password = strings.TrimSpace(row.Password)
}

func (row studentRow) hasImportData() bool {
	return strings.TrimSpace(row.Code) != "" ||
		strings.TrimSpace(row.FullName) != "" ||
		strings.TrimSpace(row.Email) != "" ||
		strings.TrimSpace(row.Phone) != "" ||
		strings.TrimSpace(row.Username) != "" ||
		strings.TrimSpace(row.Password) != ""
}

func normalizeHeader(value string) string {
	value = toASCIILower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", "/", "", ".", "")
	return replacer.Replace(value)
}

func passwordMatches(stored, password string) bool {
	if stored == password {
		return true
	}
	sum := sha256.Sum256([]byte(password))
	hexPassword := hex.EncodeToString(sum[:])
	return stored == hexPassword || stored == "sha256:"+hexPassword
}

func HashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ReadMultipartFile(file multipart.File) ([]byte, error) {
	defer file.Close()
	return io.ReadAll(file)
}
