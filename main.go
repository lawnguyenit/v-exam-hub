package main

import (
	"html/template"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	pages := map[string]*template.Template{
		"login":   template.Must(template.ParseFiles("templates/index.html")),
		"student": template.Must(template.ParseFiles("templates/student.html")),
		"teacher": template.Must(template.ParseFiles("templates/teacher.html")),
	}

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}

		renderPage(w, pages["login"])
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	mux.HandleFunc("/student", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["student"])
	})
	mux.HandleFunc("/teacher", func(w http.ResponseWriter, r *http.Request) {
		renderPage(w, pages["teacher"])
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
