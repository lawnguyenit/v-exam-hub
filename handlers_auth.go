package main

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"website-exam/internal/accountdata"
	"website-exam/internal/authsession"
	"website-exam/internal/httpapi"

	"github.com/jackc/pgx/v5/pgxpool"
)

func handleAuthLogin(db *pgxpool.Pool, limiter *loginAttemptLimiter) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload accountdata.LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "KhÃ´ng Ä‘á»c Ä‘Æ°á»£c thÃ´ng tin Ä‘Äƒng nháº­p", http.StatusBadRequest)
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
			http.Error(w, "ÄÄƒng nháº­p tháº¥t báº¡i: "+err.Error(), http.StatusUnauthorized)
			return
		}
		if err := authsession.CreateLoginSession(r.Context(), db, w, r, result.Username, result.Role); err != nil {
			status := http.StatusBadRequest
			if strings.Contains(err.Error(), "dang co phien") {
				status = http.StatusConflict
			}
			http.Error(w, "Khong tao duoc phien dang nhap: "+err.Error(), status)
			return
		}
		limiter.recordSuccess(attemptKey)
		httpapi.WriteJSON(w, result)
	}
}

func handleAuthLogout(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		authsession.Logout(r.Context(), db, w, r)
		httpapi.WriteJSON(w, map[string]any{"ok": true})
	}
}

func handleAuthMe(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		auth, ok := authsession.Require(r.Context(), db, w, r)
		if !ok {
			return
		}
		displayName, err := displayNameForSession(r.Context(), db, auth.UserID)
		if err != nil {
			http.Error(w, "Khong tai duoc phien dang nhap: "+err.Error(), http.StatusBadRequest)
			return
		}
		httpapi.WriteJSON(w, accountdata.LoginResult{
			Username:    auth.Username,
			Role:        auth.Role,
			DisplayName: displayName,
		})
	}
}

func displayNameForSession(ctx context.Context, db *pgxpool.Pool, userID int64) (string, error) {
	var displayName string
	err := db.QueryRow(ctx, `
		SELECT COALESCE(sp.full_name, tp.full_name, u.username)
		FROM users u
		LEFT JOIN student_profiles sp ON sp.user_id = u.id
		LEFT JOIN teacher_profiles tp ON tp.user_id = u.id
		WHERE u.id = $1
	`, userID).Scan(&displayName)
	return displayName, err
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
		auth, ok := authsession.Require(r.Context(), db, w, r, "admin")
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
		httpapi.WriteJSON(w, result)
	}
}
