package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"website-exam/internal/studentdata"
	"website-exam/internal/teacherdata"
)

func main() {
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

	mux.HandleFunc("/", serveFrontend("frontend/dist"))

	log.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
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
