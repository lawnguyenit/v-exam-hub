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

type ClassSummary struct {
	ID        int64  `json:"id"`
	ClassCode string `json:"classCode"`
	ClassName string `json:"className"`
}

type StudentImportResult struct {
	ClassCode          string                 `json:"classCode"`
	ClassName          string                 `json:"className"`
	Created            int                    `json:"created"`
	Updated            int                    `json:"updated"`
	AddedToClass       int                    `json:"addedToClass"`
	Skipped            int                    `json:"skipped"`
	ImportedStudents   []ImportedStudent      `json:"importedStudents"`
	GeneratedPasswords []GeneratedPasswordRow `json:"generatedPasswords"`
	Errors             []string               `json:"errors"`
}

type ImportedStudent struct {
	Username          string `json:"username"`
	StudentCode       string `json:"studentCode"`
	FullName          string `json:"fullName"`
	TemporaryPassword string `json:"temporaryPassword"`
}

type GeneratedPasswordRow struct {
	Username    string `json:"username"`
	StudentCode string `json:"studentCode"`
	FullName    string `json:"fullName"`
	Password    string `json:"password"`
}

type studentRow struct {
	Code     string
	FullName string
	Email    string
	Phone    string
	Username string
	Password string
}

func Authenticate(ctx context.Context, db *pgxpool.Pool, payload LoginRequest) (LoginResult, error) {
	username := strings.TrimSpace(payload.Username)
	password := payload.Password
	role := strings.TrimSpace(payload.Role)
	if username == "" || password == "" || role == "" {
		return LoginResult{}, fmt.Errorf("thiếu tài khoản, mật khẩu hoặc vai trò")
	}

	var storedHash, dbRole, displayName string
	err := db.QueryRow(ctx, `
		SELECT u.password_hash, r.code,
			COALESCE(sp.full_name, tp.full_name, u.username)
		FROM users u
		JOIN user_roles ur ON ur.user_id = u.id
		JOIN roles r ON r.id = ur.role_id
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN teacher_profiles tp ON tp.user_id = u.id
		WHERE u.username = $1 AND u.account_status = 'active' AND r.code = $2
		LIMIT 1
	`, username, role).Scan(&storedHash, &dbRole, &displayName)
	if err != nil {
		return LoginResult{}, fmt.Errorf("tài khoản không tồn tại hoặc không đúng vai trò")
	}
	if !passwordMatches(storedHash, password) {
		return LoginResult{}, fmt.Errorf("mật khẩu không đúng")
	}
	return LoginResult{Username: username, Role: dbRole, DisplayName: displayName}, nil
}

func ListClasses(ctx context.Context, db *pgxpool.Pool) ([]ClassSummary, error) {
	rows, err := db.Query(ctx, `
		SELECT id, class_code, class_name
		FROM classes
		WHERE class_status = 'active'
		ORDER BY class_code, class_name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	classes := []ClassSummary{}
	for rows.Next() {
		var class ClassSummary
		if err := rows.Scan(&class.ID, &class.ClassCode, &class.ClassName); err != nil {
			return nil, err
		}
		classes = append(classes, class)
	}
	return classes, rows.Err()
}

func ImportStudentsFromXLSX(ctx context.Context, db *pgxpool.Pool, classCode, className, filename string, content []byte) (StudentImportResult, error) {
	rows, err := parseXLSXRows(content)
	if err != nil {
		return StudentImportResult{}, err
	}
	if len(rows) == 0 {
		return StudentImportResult{}, fmt.Errorf("file %s không có dữ liệu sinh viên", filename)
	}
	return importStudentRows(ctx, db, classCode, className, rows)
}

func ImportStudents(ctx context.Context, db *pgxpool.Pool, payload StudentImportRequest) (StudentImportResult, error) {
	rows := parseManualRows(payload.Rows)
	return importStudentRows(ctx, db, payload.ClassCode, payload.ClassName, rows)
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
	`, password, strings.TrimSpace(payload.Username), strings.TrimSpace(payload.StudentCode))
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return fmt.Errorf("không tìm thấy sinh viên cần đổi mật khẩu")
	}
	return nil
}

func importStudentRows(ctx context.Context, db *pgxpool.Pool, classCode, className string, rows []studentRow) (StudentImportResult, error) {
	classCode = strings.TrimSpace(classCode)
	className = strings.TrimSpace(className)
	if classCode == "" || className == "" {
		return StudentImportResult{}, fmt.Errorf("thiếu mã lớp hoặc tên lớp")
	}
	result := StudentImportResult{ClassCode: classCode, ClassName: className}
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var classID int64
	if err := tx.QueryRow(ctx, `
		INSERT INTO classes (class_code, class_name)
		VALUES ($1, $2)
		ON CONFLICT (class_code) DO UPDATE SET class_name = EXCLUDED.class_name, updated_at = NOW()
		RETURNING id
	`, classCode, className).Scan(&classID); err != nil {
		return result, err
	}

	roleID, err := ensureRole(ctx, tx, "student", "Sinh viên")
	if err != nil {
		return result, err
	}

	for index, row := range rows {
		row.normalize()
		if row.Code == "" || row.FullName == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("Dòng %d thiếu mã sinh viên hoặc họ tên", index+1))
			result.Skipped++
			continue
		}
		if row.Username == "" {
			row.Username = row.Code
		}
		if row.Password == "" {
			row.Password = row.Code
		}

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
		`, row.Username, row.Password).Scan(&userID, &inserted); err != nil {
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
		imported := ImportedStudent{Username: row.Username, StudentCode: row.Code, FullName: row.FullName, TemporaryPassword: row.Password}
		result.ImportedStudents = append(result.ImportedStudents, imported)
		result.GeneratedPasswords = append(result.GeneratedPasswords, GeneratedPasswordRow{Username: row.Username, StudentCode: row.Code, FullName: row.FullName, Password: row.Password})
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
		rows = append(rows, studentRow{Code: parts[0], FullName: parts[1], Email: parts[2], Phone: parts[3], Username: parts[4], Password: parts[5]})
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
	header := map[string]int{}
	for index, cell := range values[0] {
		header[normalizeHeader(cell)] = index
	}
	get := func(row []string, names ...string) string {
		for _, name := range names {
			if index, ok := header[name]; ok && index < len(row) {
				return row[index]
			}
		}
		return ""
	}
	rows := []studentRow{}
	for _, row := range values[1:] {
		rows = append(rows, studentRow{
			Code:     get(row, "masv", "ma sv", "mssv", "studentcode"),
			FullName: get(row, "hoten", "ho va ten", "họ và tên", "fullname"),
			Email:    get(row, "email"),
			Phone:    get(row, "sdt", "so dien thoai", "phone"),
			Username: get(row, "taikhoan", "tai khoan", "username"),
			Password: get(row, "matkhau", "mat khau", "password"),
		})
	}
	return rows, nil
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

func normalizeHeader(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("đ", "d", "Đ", "d", " ", "", "_", "", "-", "")
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

func ReadMultipartFile(file multipart.File) ([]byte, error) {
	defer file.Close()
	return io.ReadAll(file)
}
