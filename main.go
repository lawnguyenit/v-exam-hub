package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"

	"website-exam/internal/studentdata"
	"website-exam/internal/teacherdata"
)

func main() {
	mux := http.NewServeMux()
	pages := map[string]*template.Template{
		"roleSelect":    template.Must(template.ParseFiles("templates/index.html")),
		"studentLogin":  template.Must(template.ParseFiles("templates/student_login.html")),
		"teacherLogin":  template.Must(template.ParseFiles("templates/teacher_login.html")),
		"student":       template.Must(template.ParseFiles("templates/student.html")),
		"studentExam":   template.Must(template.ParseFiles("templates/student_exam.html")),
		"studentReview": template.Must(template.ParseFiles("templates/student_review.html")),
		"teacher":       template.Must(template.ParseFiles("templates/teacher.html")),
		"teacherCreate": template.Must(template.ParseFiles("templates/teacher_create.html")),
	}

	staticFiles := http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	mux.Handle("/static/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		staticFiles.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		renderPage(w, pages["roleSelect"])
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("/login/student", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["studentLogin"])
	})
	mux.HandleFunc("/login/teacher", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["teacherLogin"])
	})
	mux.HandleFunc("/student", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["student"])
	})
	mux.HandleFunc("/student/exam", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["studentExam"])
	})
	mux.HandleFunc("/student/review", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["studentReview"])
	})
	mux.HandleFunc("/teacher", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["teacher"])
	})
	mux.HandleFunc("/teacher/create", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["teacherCreate"])
	})
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

	log.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func renderPage(w http.ResponseWriter, tmpl *template.Template) {
	if err := tmpl.Execute(w, nil); err != nil {
		http.Error(w, "Không thể tải giao diện", http.StatusInternalServerError)
		log.Println(err)
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
