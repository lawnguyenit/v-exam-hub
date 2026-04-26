package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"website-exam/internal/accountdata"
	"website-exam/internal/importdata"
	"website-exam/internal/studentdata"
	"website-exam/internal/teacherdata"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	db, err := connectDB(context.Background())
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()
	loginLimiter := newLoginAttemptLimiter()

	mux.HandleFunc("/api/student/dashboard", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		dashboard, err := studentdata.DashboardFor(r.Context(), db, auth.Username)
		if err != nil {
			http.Error(w, "Không tải được dashboard sinh viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, dashboard)
	})
	mux.HandleFunc("/api/student/attempts/start", handleStudentAttemptStart(db))
	mux.HandleFunc("/api/student/attempts/save", handleStudentAttemptSave(db))
	mux.HandleFunc("/api/student/attempts/sync", handleStudentAttemptSync(db))
	mux.HandleFunc("/api/student/attempts/progress", handleStudentAttemptProgress(db))
	mux.HandleFunc("/api/student/attempts/submit", handleStudentAttemptSubmit(db))
	mux.HandleFunc("/api/auth/login", handleAuthLogin(db, loginLimiter))
	mux.HandleFunc("/api/auth/logout", handleAuthLogout(db))
	mux.HandleFunc("/api/admin/teachers", handleAdminTeacherCreate(db))
	mux.HandleFunc("/api/student/exams/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(r.Context(), db, w, r, "student"); !ok {
			return
		}
		id := r.URL.Path[len("/api/student/exams/"):]
		exam, ok, err := studentdata.ExamByID(r.Context(), db, id)
		if err != nil {
			http.Error(w, "Không tải được bài kiểm tra: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, exam)
	})
	mux.HandleFunc("/api/student/reviews/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(r.Context(), db, w, r, "student"); !ok {
			return
		}
		id := r.URL.Path[len("/api/student/reviews/"):]
		review, ok, err := studentdata.ReviewByID(r.Context(), db, id)
		if err != nil {
			http.Error(w, "Không tải được bài xem lại: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, review)
	})
	mux.HandleFunc("/api/teacher/dashboard", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		dashboard, err := teacherdata.DashboardFor(r.Context(), db, auth.Username)
		if err != nil {
			http.Error(w, "Không tải được dashboard giáo viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, dashboard)
	})
	mux.HandleFunc("/api/teacher/question-bank/", handleTeacherQuestionBankSource(db))
	mux.HandleFunc("/api/teacher/exams/", func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		id := r.URL.Path[len("/api/teacher/exams/"):]
		if examID, ok := strings.CutSuffix(id, "/access-code"); ok {
			if r.Method != http.MethodPost {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !teacherOwnsExam(r.Context(), db, examID, auth.UserID) {
				http.Error(w, "khong co quyen voi bai thi nay", http.StatusForbidden)
				return
			}
			result, err := teacherdata.GenerateAccessCode(r.Context(), db, examID)
			if err != nil {
				http.Error(w, "Khong tao duoc ma truy cap: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, result)
			return
		}
		if examID, ok := strings.CutSuffix(id, "/snapshot"); ok {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !teacherOwnsExam(r.Context(), db, examID, auth.UserID) {
				http.Error(w, "khong co quyen voi bai thi nay", http.StatusForbidden)
				return
			}
			snapshot, err := teacherdata.LiveSnapshot(r.Context(), db, examID)
			if err != nil {
				http.Error(w, "Khong tai duoc snapshot phong thi: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, snapshot)
			return
		}
		if examID, ok := strings.CutSuffix(id, "/export"); ok {
			if r.Method != http.MethodGet {
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
				return
			}
			if !teacherOwnsExam(r.Context(), db, examID, auth.UserID) {
				http.Error(w, "khong co quyen voi bai thi nay", http.StatusForbidden)
				return
			}
			content, filename, err := teacherdata.ExportExamScoresXLSX(r.Context(), db, examID)
			if err != nil {
				http.Error(w, "Khong export duoc bang diem: "+err.Error(), http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
			w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			w.Header().Set("Cache-Control", "no-store")
			_, _ = w.Write(content)
			return
		}
		if r.Method == http.MethodDelete {
			if !teacherOwnsExam(r.Context(), db, id, auth.UserID) {
				http.Error(w, "khong co quyen voi bai thi nay", http.StatusForbidden)
				return
			}
			if err := teacherdata.DeleteExam(r.Context(), db, id); err != nil {
				http.Error(w, "Không xoá được bài thi: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !teacherOwnsExam(r.Context(), db, id, auth.UserID) {
			http.Error(w, "khong co quyen voi bai thi nay", http.StatusForbidden)
			return
		}
		exam, ok, err := teacherdata.ExamDetailByID(r.Context(), db, id)
		if err != nil {
			http.Error(w, "Không tải được chi tiết bài thi: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, exam)
	})
	mux.HandleFunc("/api/teacher/profile", handleTeacherProfileUpdate(db))
	mux.HandleFunc("/api/teacher/question-bank", handleTeacherQuestionBank(db))
	mux.HandleFunc("/api/teacher/exams/create", handleTeacherExamCreate(db))
	mux.HandleFunc("/api/teacher/import/parse", handleTeacherImportParse(db))
	mux.HandleFunc("/api/teacher/import/items/create", handleTeacherImportItemCreate(db))
	mux.HandleFunc("/api/teacher/import/items/save", handleTeacherImportItemSave(db))
	mux.HandleFunc("/api/teacher/import/items/delete", handleTeacherImportItemDelete(db))
	mux.HandleFunc("/api/teacher/import/approve-pass", handleTeacherImportApprovePass(db))
	mux.HandleFunc("/api/teacher/import/assets/", handleTeacherImportAsset(db))
	mux.HandleFunc("/api/teacher/classes", handleTeacherClasses(db))
	mux.HandleFunc("/api/teacher/classes/", handleTeacherClassDetail(db))
	mux.HandleFunc("/api/teacher/classes/import-students", handleTeacherClassStudentImport(db))
	mux.HandleFunc("/api/teacher/students/password", handleTeacherStudentPasswordUpdate(db))

	// mux.HandleFunc("/", serveFrontend("frontend/dist"))

	// log.Println("Server running at http://localhost:8080")
	// if err := http.ListenAndServe(":8080", mux); err != nil {
	// 	log.Fatal(err)
	// }
	// 2. Trong hàm main(), sửa đoạn ListenAndServe:
	addr := listenAddress()
	log.Printf("Server running at %s", addr)
	// Bọc mux bằng hàm enableCORS vừa tạo
	if err := http.ListenAndServe(addr, enableRuntimeCORS(mux)); err != nil {
		log.Fatal(err)
	}
}

func listenAddress() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	return ":" + port
}

func handleStudentAttemptStart(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptStartRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu bắt đầu bài", http.StatusBadRequest)
			return
		}
		payload.Account = auth.Username
		state, err := studentdata.StartAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không bắt đầu được bài làm: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}

func handleStudentAttemptSave(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptAnswerRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được đáp án", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SaveAnswer(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không lưu được đáp án: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}

func handleStudentAttemptSync(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu đồng bộ", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SyncAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không đồng bộ được bài làm: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}

func handleStudentAttemptProgress(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptProgressRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được tiến trình", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.UpdateProgress(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không lưu được tiến trình: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}

func handleStudentAttemptSubmit(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "student")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu nộp bài", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SubmitAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không nộp được bài: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}

func handleTeacherProfileUpdate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload teacherdata.ProfileUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được thông tin hồ sơ giáo viên", http.StatusBadRequest)
			return
		}
		payload.Username = auth.Username
		profile, err := teacherdata.UpdateProfile(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không lưu được hồ sơ giáo viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, profile)
	}
}

func handleTeacherQuestionBank(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		items, err := teacherdata.QuestionBank(r.Context(), db, auth.Username)
		if err != nil {
			http.Error(w, "Không tải được ngân hàng câu hỏi: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, items)
	}
}

func handleTeacherQuestionBankSource(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodDelete {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		idText := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teacher/question-bank/"), "/")
		batchID, err := strconv.ParseInt(idText, 10, 64)
		if err != nil || batchID <= 0 {
			http.NotFound(w, r)
			return
		}
		result, err := teacherdata.DeleteQuestionBankSource(r.Context(), db, batchID, auth.Username)
		if err != nil {
			http.Error(w, "Khong xoa duoc bo de cuong: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherExamCreate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload teacherdata.ExamCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được cấu hình bài kiểm tra", http.StatusBadRequest)
			return
		}
		payload.CreatedBy = auth.Username
		result, err := teacherdata.CreateExam(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không tạo được bài kiểm tra: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleAuthLogin(db *pgxpool.Pool, limiter *loginAttemptLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload accountdata.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được thông tin đăng nhập", http.StatusBadRequest)
			return
		}
		attemptKey := loginAttemptKey(payload)
		if wait := limiter.retryAfter(attemptKey, time.Now()); wait > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
			http.Error(w, "Dang nhap sai qua nhieu lan. Vui long doi 1 phut roi thu lai.", http.StatusTooManyRequests)
			return
		}
		result, err := accountdata.Authenticate(r.Context(), db, payload)
		if err != nil {
			if wait := limiter.recordFailure(attemptKey, time.Now()); wait > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())))
				http.Error(w, "Dang nhap sai qua nhieu lan. Vui long doi 1 phut roi thu lai.", http.StatusTooManyRequests)
				return
			}
			http.Error(w, "Đăng nhập thất bại: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if err := createLoginSession(r.Context(), db, w, r, result.Username, result.Role); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "dang co phien") {
				status = http.StatusConflict
			}
			http.Error(w, "Khong tao duoc phien dang nhap: "+err.Error(), status)
			return
		}
		limiter.recordSuccess(attemptKey)
		writeJSON(w, result)
	}
}

func handleAuthLogout(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		logoutSession(r.Context(), db, w, r)
		writeJSON(w, map[string]any{"ok": true})
	}
}

const (
	failedLoginLimit   = 5
	failedLoginWindow  = 10 * time.Minute
	failedLoginLockout = 1 * time.Minute
)

type loginAttemptState struct {
	count       int
	lastFailure time.Time
	lockedUntil time.Time
}

type loginAttemptLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttemptState
}

func newLoginAttemptLimiter() *loginAttemptLimiter {
	return &loginAttemptLimiter{attempts: map[string]loginAttemptState{}}
}

func loginAttemptKey(payload accountdata.LoginRequest) string {
	username := strings.ToLower(strings.TrimSpace(payload.Username))
	role := strings.ToLower(strings.TrimSpace(payload.Role))
	return username + "|" + role
}

func (l *loginAttemptLimiter) retryAfter(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	state, ok := l.attempts[key]
	if !ok || !now.Before(state.lockedUntil) {
		return 0
	}
	return time.Until(state.lockedUntil).Round(time.Second)
}

func (l *loginAttemptLimiter) recordFailure(key string, now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	state := l.attempts[key]
	if !state.lastFailure.IsZero() && now.Sub(state.lastFailure) > failedLoginWindow {
		state.count = 0
	}
	state.count++
	state.lastFailure = now
	if state.count >= failedLoginLimit {
		state.count = 0
		state.lockedUntil = now.Add(failedLoginLockout)
	}
	l.attempts[key] = state
	if now.Before(state.lockedUntil) {
		return time.Until(state.lockedUntil).Round(time.Second)
	}
	return 0
}

func (l *loginAttemptLimiter) recordSuccess(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.attempts, key)
}

func handleAdminTeacherCreate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "admin")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload accountdata.TeacherCreateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Khong doc duoc thong tin giao vien", http.StatusBadRequest)
			return
		}
		payload.AdminUsername = auth.Username
		result, err := accountdata.CreateTeacherAccount(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Khong tao duoc tai khoan giao vien: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherImportItemCreate(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64                     `json:"importBatchId"`
		Question      importdata.ParsedQuestion `json:"question"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Khong doc duoc du lieu cau hoi", http.StatusBadRequest)
			return
		}
		if !teacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		question, err := importdata.CreateImportItem(r.Context(), db, payload.ImportBatchID, payload.Question)
		if err != nil {
			http.Error(w, "Khong tao duoc cau hoi: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, question)
	}
}

func handleTeacherImportItemSave(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64                     `json:"importBatchId"`
		Question      importdata.ParsedQuestion `json:"question"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu câu hỏi", http.StatusBadRequest)
			return
		}
		if !teacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		if err := importdata.UpdateImportItem(r.Context(), db, payload.ImportBatchID, payload.Question); err != nil {
			http.Error(w, "Không lưu được câu hỏi: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportItemDelete(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64 `json:"importBatchId"`
		ImportItemID  int64 `json:"importItemId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được câu cần xoá", http.StatusBadRequest)
			return
		}
		if !teacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		if err := importdata.RejectImportItem(r.Context(), db, payload.ImportBatchID, payload.ImportItemID); err != nil {
			http.Error(w, "Không xoá được câu khỏi batch: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportApprovePass(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64 `json:"importBatchId"`
		TargetBatchID int64 `json:"targetBatchId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được batch import", http.StatusBadRequest)
			return
		}
		if !teacherOwnsBatch(r.Context(), db, payload.ImportBatchID, auth.UserID) || (payload.TargetBatchID > 0 && !teacherOwnsBatch(r.Context(), db, payload.TargetBatchID, auth.UserID)) {
			http.Error(w, "khong co quyen voi batch import nay", http.StatusForbidden)
			return
		}
		result, err := importdata.ApprovePassedImportItemsToSource(r.Context(), db, payload.ImportBatchID, payload.TargetBatchID)
		if err != nil {
			http.Error(w, "Không lưu được câu pass: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherClasses(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		classes, err := accountdata.ListClasses(r.Context(), db, auth.UserID)
		if err != nil {
			http.Error(w, "Không tải được danh sách lớp: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, classes)
	}
}

func handleTeacherClassDetail(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		trimmed := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/teacher/classes/"), "/")
		parts := strings.Split(trimmed, "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == "import-students" {
			http.NotFound(w, r)
			return
		}
		classID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || classID <= 0 {
			http.Error(w, "mã lớp không hợp lệ", http.StatusBadRequest)
			return
		}
		if len(parts) == 1 {
			switch r.Method {
			case http.MethodGet:
				detail, err := accountdata.ClassDetailByID(r.Context(), db, auth.UserID, classID)
				if err != nil {
					http.Error(w, "Không tải được chi tiết lớp: "+err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, detail)
			case http.MethodPatch:
				var payload accountdata.ClassUpdateRequest
				if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
					http.Error(w, "Không đọc được dữ liệu lớp", http.StatusBadRequest)
					return
				}
				updated, err := accountdata.UpdateClass(r.Context(), db, auth.UserID, classID, payload)
				if err != nil {
					http.Error(w, "Không sửa được lớp: "+err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, updated)
			case http.MethodDelete:
				if err := accountdata.ArchiveClass(r.Context(), db, auth.UserID, classID); err != nil {
					http.Error(w, "Không xóa được lớp: "+err.Error(), http.StatusBadRequest)
					return
				}
				writeJSON(w, map[string]any{"ok": true})
			default:
				http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			}
			return
		}
		if len(parts) == 3 && parts[1] == "members" && r.Method == http.MethodDelete {
			studentUserID, err := strconv.ParseInt(parts[2], 10, 64)
			if err != nil || studentUserID <= 0 {
				http.Error(w, "mã sinh viên không hợp lệ", http.StatusBadRequest)
				return
			}
			if err := accountdata.RemoveClassMember(r.Context(), db, auth.UserID, classID, studentUserID); err != nil {
				http.Error(w, "Không xóa được sinh viên khỏi lớp: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
			return
		}
		http.NotFound(w, r)
	}
}

func handleTeacherClassStudentImport(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
			r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
			if err := r.ParseMultipartForm(32 << 20); err != nil {
				http.Error(w, "Không đọc được file danh sách sinh viên", http.StatusBadRequest)
				return
			}
			file, header, err := r.FormFile("file")
			if err != nil {
				http.Error(w, "Thiếu file danh sách sinh viên", http.StatusBadRequest)
				return
			}
			defer file.Close()
			content, err := io.ReadAll(file)
			if err != nil {
				http.Error(w, "Không đọc được nội dung file", http.StatusBadRequest)
				return
			}
			result, err := accountdata.ImportStudentsFromXLSX(
				r.Context(),
				db,
				auth.UserID,
				r.FormValue("classCode"),
				r.FormValue("className"),
				header.Filename,
				content,
			)
			if err != nil {
				http.Error(w, "Không tạo được tài khoản sinh viên: "+err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, result)
			return
		}
		var payload accountdata.StudentImportRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được danh sách sinh viên", http.StatusBadRequest)
			return
		}
		result, err := accountdata.ImportStudents(r.Context(), db, auth.UserID, payload)
		if err != nil {
			http.Error(w, "Không tạo được tài khoản sinh viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherStudentPasswordUpdate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(r.Context(), db, w, r, "teacher"); !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload accountdata.StudentPasswordUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu đổi mật khẩu", http.StatusBadRequest)
			return
		}
		if err := accountdata.UpdateStudentPassword(r.Context(), db, payload); err != nil {
			http.Error(w, "Không đổi được mật khẩu: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func connectDB(ctx context.Context) (*pgxpool.Pool, error) {
	databaseURL := strings.TrimSpace(os.Getenv("DB_URL"))
	if databaseURL == "" {
		if strings.EqualFold(strings.TrimSpace(os.Getenv("APP_ENV")), "production") {
			return nil, fmt.Errorf("DB_URL is required when APP_ENV=production")
		}
		databaseURL = "postgres://admin:123@localhost:5432/v_exam_hub?sslmode=disable"
	}
	startupTimeout := 90 * time.Second
	if raw := strings.TrimSpace(os.Getenv("DB_STARTUP_TIMEOUT")); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
			startupTimeout = parsed
		}
	}

	deadlineCtx, cancel := context.WithTimeout(ctx, startupTimeout)
	defer cancel()

	var lastErr error
	for attempt := 1; ; attempt++ {
		attemptCtx, attemptCancel := context.WithTimeout(deadlineCtx, 5*time.Second)
		db, err := pgxpool.New(attemptCtx, databaseURL)
		if err == nil {
			err = db.Ping(attemptCtx)
		}
		attemptCancel()

		if err == nil {
			if err := ensureCoreSchema(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			if err := ensureDatabaseCompatibility(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			if err := ensureSessionSchema(deadlineCtx, db); err != nil {
				db.Close()
				return nil, err
			}
			log.Println("Database connected")
			return db, nil
		}

		lastErr = err
		if deadlineCtx.Err() != nil {
			break
		}
		log.Printf("database not ready (attempt %d): %v", attempt, err)
		select {
		case <-time.After(2 * time.Second):
		case <-deadlineCtx.Done():
		}
	}

	if deadlineCtx.Err() != nil && lastErr != nil {
		return nil, fmt.Errorf("timed out waiting for database after %s: %w", startupTimeout, lastErr)
	}
	return nil, lastErr
}

func ensureCoreSchema(ctx context.Context, db *pgxpool.Pool) error {
	requiredTables := []string{
		"roles",
		"users",
		"user_roles",
		"student_profiles",
		"teacher_profiles",
		"classes",
		"class_members",
		"import_batches",
		"import_items",
		"question_bank",
		"question_bank_options",
		"exams",
		"exam_questions",
		"exam_targets",
		"exam_attempts",
	}

	missing := make([]string, 0, len(requiredTables))
	for _, tableName := range requiredTables {
		exists, err := tableExists(ctx, db, tableName)
		if err != nil {
			return err
		}
		if !exists {
			missing = append(missing, tableName)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf(
			"missing core schema tables: %s. Bootstrap PostgreSQL bằng file D:\\v-exam-hub\\v-exam-hub-local-db.session.sql hoặc mount file này vào /docker-entrypoint-initdb.d trước khi chạy backend",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

func tableExists(ctx context.Context, db *pgxpool.Pool, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)
	`, tableName).Scan(&exists)
	return exists, err
}

func ensureDatabaseCompatibility(ctx context.Context, db *pgxpool.Pool) error {
	// 1. Khởi tạo các Type (ENUM) nếu chưa có
	// Chúng ta bọc trong khối DO để tránh lỗi "already exists"
	setupEnumsSQL := `
        DO $$ BEGIN
            IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'attachment_target_enum') THEN
                CREATE TYPE attachment_target_enum AS ENUM ('question_body', 'option', 'explanation', 'unknown');
            END IF;
        END $$;
    `
	if _, err := db.Exec(ctx, setupEnumsSQL); err != nil {
		return err
	}

	// 2. Cập nhật ENUM hiện có (Thêm giá trị mới nếu cần)
	// Lệnh ADD VALUE IF NOT EXISTS chỉ chạy được từ Postgres 13+
	if _, err := db.Exec(ctx, `ALTER TYPE exam_mode_enum ADD VALUE IF NOT EXISTS 'official'`); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, `ALTER TYPE exam_mode_enum ADD VALUE IF NOT EXISTS 'attendance'`); err != nil {
		return err
	}

	// 3. Thực thi các lệnh tạo bảng và chỉnh sửa ràng buộc (Constraint)
	// Lưu ý: Các bảng như 'exams' phải tồn tại trước khi chạy ALTER TABLE
	_, err := db.Exec(ctx, `
        -- Chỉ chạy ALTER TABLE nếu bảng 'exams' đã tồn tại
        DO $$ BEGIN
            IF EXISTS (SELECT FROM information_schema.tables WHERE table_name = 'exams') THEN
                ALTER TABLE exams DROP CONSTRAINT IF EXISTS ck_exam_attempts_positive;
                ALTER TABLE exams DROP CONSTRAINT IF EXISTS ck_exam_attempts_non_negative;
                ALTER TABLE exams ADD CONSTRAINT ck_exam_attempts_non_negative CHECK (max_attempts_per_student >= 0);
            END IF;
        END $$;

        -- Tạo bảng mới nếu chưa có
        CREATE TABLE IF NOT EXISTS import_item_assets (
            id BIGSERIAL PRIMARY KEY,
            batch_id BIGINT NOT NULL REFERENCES import_batches(id) ON DELETE CASCADE,
            import_item_id BIGINT REFERENCES import_items(id) ON DELETE SET NULL,
            target attachment_target_enum NOT NULL DEFAULT 'unknown',
            option_label VARCHAR(10),
            file_url TEXT NOT NULL,
            mime_type VARCHAR(100),
            page_number INT,
            display_order INT NOT NULL DEFAULT 0,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );

        CREATE INDEX IF NOT EXISTS idx_import_item_assets_batch_order ON import_item_assets(batch_id, import_item_id, display_order);
    `)
	return err
}

func handleTeacherImportAsset(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(r.Context(), db, w, r); !ok {
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		trimmed := strings.TrimPrefix(r.URL.Path, "/api/teacher/import/assets/")
		parts := strings.Split(strings.Trim(trimmed, "/"), "/")
		if len(parts) != 2 {
			http.NotFound(w, r)
			return
		}
		batchID, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil || batchID <= 0 {
			http.NotFound(w, r)
			return
		}
		displayOrder, err := strconv.Atoi(parts[1])
		if err != nil || displayOrder <= 0 {
			http.NotFound(w, r)
			return
		}

		var fileURL, mimeType string
		err = db.QueryRow(r.Context(), `
			SELECT file_url, COALESCE(mime_type, 'application/octet-stream')
			FROM import_item_assets
			WHERE batch_id = $1 AND display_order = $2
			ORDER BY id
			LIMIT 1
		`, batchID, displayOrder).Scan(&fileURL, &mimeType)
		if err != nil {
			http.NotFound(w, r)
			return
		}

		cleanPath := filepath.Clean(filepath.FromSlash(fileURL))
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		importRoot, err := filepath.Abs(filepath.Join("data", "imports"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		if absPath != importRoot && !strings.HasPrefix(absPath, importRoot+string(os.PathSeparator)) {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", mimeType)
		w.Header().Set("Cache-Control", "private, max-age=3600")
		http.ServeFile(w, r, absPath)
	}
}

func handleTeacherImportParse(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		auth, ok := requireAuth(r.Context(), db, w, r, "teacher")
		if !ok {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 64<<20)
		if err := r.ParseMultipartForm(64 << 20); err != nil {
			http.Error(w, "Không đọc được file upload", http.StatusBadRequest)
			return
		}

		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Thiếu file đề thi", http.StatusBadRequest)
			return
		}

		result, err := importdata.ParseUpload(file, header)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := importdata.SaveImport(r.Context(), db, &result, auth.Username); err != nil {
			http.Error(w, "Không lưu được import vào database: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
}

func serveFrontend(distDir string) http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(distDir))
	indexPath := filepath.Join(distDir, "index.html")

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.NotFound(w, r)
			return
		}

		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if cleanPath != "." {
			filePath := filepath.Join(distDir, cleanPath)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
			if path.Ext(cleanPath) != "" {
				http.NotFound(w, r)
				return
			}
		}

		w.Header().Set("Cache-Control", "no-store")
		http.ServeFile(w, r, indexPath)
	}
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, "Không thể tải dữ liệu", http.StatusInternalServerError)
		log.Println(err)
	}
}
func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Trong thực tế nên để domain thật
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if r.Method == "OPTIONS" {
			return
		}
		next.ServeHTTP(w, r)
	})
}
