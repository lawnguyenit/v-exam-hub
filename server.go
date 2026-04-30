package main

import (
	"context"
	"log"
	"net/http"
	"strings"

	"website-exam/internal/config"
	"website-exam/internal/studentdata"
	"website-exam/internal/teacherdata"
)

func runServer(ctx context.Context) {
	cfg := config.Load()

	db, err := connectDB(ctx, cfg)
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
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c dashboard sinh viÃªn: "+err.Error(), http.StatusBadRequest)
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
	mux.HandleFunc("/api/auth/me", handleAuthMe(db))
	mux.HandleFunc("/api/auth/logout", handleAuthLogout(db))
	mux.HandleFunc("/api/admin/teachers", handleAdminTeacherCreate(db))
	mux.HandleFunc("/api/student/exams/", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireAuth(r.Context(), db, w, r, "student"); !ok {
			return
		}
		id := r.URL.Path[len("/api/student/exams/"):]
		exam, ok, err := studentdata.ExamByID(r.Context(), db, id)
		if err != nil {
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c bÃ i kiá»ƒm tra: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c bÃ i xem láº¡i: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c dashboard giÃ¡o viÃªn: "+err.Error(), http.StatusBadRequest)
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
				http.Error(w, "KhÃ´ng xoÃ¡ Ä‘Æ°á»£c bÃ i thi: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng táº£i Ä‘Æ°á»£c chi tiáº¿t bÃ i thi: "+err.Error(), http.StatusBadRequest)
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
	// 2. Trong hÃ m main(), sá»­a Ä‘oáº¡n ListenAndServe:
	log.Printf("Server running at %s", cfg.Address)
	// Bá»c mux báº±ng hÃ m enableCORS vá»«a táº¡o
	if err := http.ListenAndServe(cfg.Address, enableRuntimeCORS(mux, cfg.CORSAllowedOrigins)); err != nil {
		log.Fatal(err)
	}
}
