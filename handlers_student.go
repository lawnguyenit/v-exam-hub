package main

import (
	"encoding/json"
	"net/http"

	"website-exam/internal/studentdata"

	"github.com/jackc/pgx/v5/pgxpool"
)

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
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u báº¯t Ä‘áº§u bÃ i", http.StatusBadRequest)
			return
		}
		payload.Account = auth.Username
		state, err := studentdata.StartAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng báº¯t Ä‘áº§u Ä‘Æ°á»£c bÃ i lÃ m: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c Ä‘Ã¡p Ã¡n", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SaveAnswer(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c Ä‘Ã¡p Ã¡n: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u Ä‘á»“ng bá»™", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SyncAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»“ng bá»™ Ä‘Æ°á»£c bÃ i lÃ m: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c tiáº¿n trÃ¬nh", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.UpdateProgress(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng lÆ°u Ä‘Æ°á»£c tiáº¿n trÃ¬nh: "+err.Error(), http.StatusBadRequest)
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
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c dá»¯ liá»‡u ná»™p bÃ i", http.StatusBadRequest)
			return
		}
		if err := ensureStudentAttemptOwner(r.Context(), db, payload.AttemptID, auth.UserID); err != nil {
			http.Error(w, "khong co quyen voi bai lam nay", http.StatusForbidden)
			return
		}
		state, err := studentdata.SubmitAttempt(r.Context(), db, payload)
		if err != nil {
			http.Error(w, "KhÃ´ng ná»™p Ä‘Æ°á»£c bÃ i: "+err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, state)
	}
}
