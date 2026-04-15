package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
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
		writeJSON(w, studentdata.DashboardFor(r.URL.Query().Get("account")))
	})
	mux.HandleFunc("/api/student/exams/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/student/exams/"):]
		exam, ok := studentdata.ExamByID(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, exam)
	})
	mux.HandleFunc("/api/student/reviews/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/student/reviews/"):]
		review, ok := studentdata.ReviewByID(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, review)
	})
	mux.HandleFunc("/api/teacher/dashboard", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, teacherdata.DashboardFor(r.URL.Query().Get("account")))
	})
	mux.HandleFunc("/api/teacher/exams/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/api/teacher/exams/"):]
		exam, ok := teacherdata.ExamDetailByID(id)
		if !ok {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, exam)
	})
	mux.HandleFunc("/api/teacher/import/parse", handleTeacherImportParse(db))
	mux.HandleFunc("/api/teacher/import/items/save", handleTeacherImportItemSave(db))

	mux.HandleFunc("/", serveFrontend("frontend/dist"))

	log.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
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
