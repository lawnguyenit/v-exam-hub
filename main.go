package main

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"website-exam/internal/accountdata"
	"website-exam/internal/importdata"
	"website-exam/internal/studentdata"
	"website-exam/internal/teacherdata"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, err := connectDB(ctx)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	defer db.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/student/dashboard", func(w http.ResponseWriter, r *http.Request) {
		dashboard, err := studentdata.DashboardFor(r.Context(), db, r.URL.Query().Get("account"))
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
	mux.HandleFunc("/api/auth/login", handleAuthLogin(db))
	mux.HandleFunc("/api/student/exams/", func(w http.ResponseWriter, r *http.Request) {
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
		dashboard, err := teacherdata.DashboardFor(r.Context(), db, r.URL.Query().Get("account"))
		if err != nil {
			http.Error(w, "Không tải được dashboard giáo viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, dashboard)
	})
	mux.HandleFunc("/api/teacher/exams/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/teacher/exams/"):]
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
	mux.HandleFunc("/api/teacher/import/parse", handleTeacherImportParse(db))
	mux.HandleFunc("/api/teacher/import/items/save", handleTeacherImportItemSave(db))
	mux.HandleFunc("/api/teacher/import/approve-pass", handleTeacherImportApprovePass(db))
	mux.HandleFunc("/api/teacher/classes", handleTeacherClasses(db))
	mux.HandleFunc("/api/teacher/classes/import-students", handleTeacherClassStudentImport(db))
	mux.HandleFunc("/api/teacher/students/password", handleTeacherStudentPasswordUpdate(db))

	mux.HandleFunc("/", serveFrontend("frontend/dist"))

	log.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func handleStudentAttemptStart(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptStartRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu bắt đầu bài", http.StatusBadRequest)
			return
		}
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptAnswerRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được đáp án", http.StatusBadRequest)
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptSyncRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu đồng bộ", http.StatusBadRequest)
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptProgressRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được tiến trình", http.StatusBadRequest)
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload studentdata.AttemptSubmitRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu nộp bài", http.StatusBadRequest)
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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload teacherdata.ProfileUpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được thông tin hồ sơ giáo viên", http.StatusBadRequest)
			return
		}
		profile, err := teacherdata.UpdateProfile(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không lưu được hồ sơ giáo viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, profile)
	}
}

func handleAuthLogin(db *pgxpool.Pool) http.HandlerFunc {
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
		result, err := accountdata.Authenticate(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Đăng nhập thất bại: "+err.Error(), http.StatusUnauthorized)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherImportItemSave(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64                     `json:"importBatchId"`
		Question      importdata.ParsedQuestion `json:"question"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được dữ liệu câu hỏi", http.StatusBadRequest)
			return
		}
		if err := importdata.UpdateImportItem(r.Context(), db, payload.ImportBatchID, payload.Question); err != nil {
			http.Error(w, "Không lưu được câu hỏi: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func handleTeacherImportApprovePass(db *pgxpool.Pool) http.HandlerFunc {
	type requestBody struct {
		ImportBatchID int64 `json:"importBatchId"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload requestBody
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Không đọc được batch import", http.StatusBadRequest)
			return
		}
		result, err := importdata.ApprovePassedImportItems(r.Context(), db, payload.ImportBatchID)
		if err != nil {
			http.Error(w, "Không lưu được câu pass: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherClasses(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		classes, err := accountdata.ListClasses(r.Context(), db)
		if err != nil {
			http.Error(w, "Không tải được danh sách lớp: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, classes)
	}
}

func handleTeacherClassStudentImport(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		result, err := accountdata.ImportStudents(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "Không tạo được tài khoản sinh viên: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, result)
	}
}

func handleTeacherStudentPasswordUpdate(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://admin:123123@localhost:5432/v_exam_hub?sslmode=disable"
	}
	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, err
	}
	log.Println("Database connected")
	return db, nil
}

func handleTeacherImportParse(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if err := importdata.SaveImport(r.Context(), db, &result); err != nil {
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
