package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	sessionCookieName = "examhub_session"
	sessionTTL        = 8 * time.Hour
)

type authSession struct {
	ID       int64
	UserID   int64
	Username string
	Role     string
}

func ensureSessionSchema(ctx context.Context, db *pgxpool.Pool) error {
	_, err := db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_sessions (
			id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			role_code VARCHAR(32) NOT NULL,
			token_hash CHAR(64) NOT NULL UNIQUE,
			user_agent TEXT,
			ip_address INET,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			expires_at TIMESTAMPTZ NOT NULL,
			revoked_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_user_sessions_user_active ON user_sessions(user_id, expires_at) WHERE revoked_at IS NULL;
		CREATE INDEX IF NOT EXISTS idx_user_sessions_token_active ON user_sessions(token_hash, expires_at) WHERE revoked_at IS NULL;
	`)
	return err
}

func createLoginSession(ctx context.Context, db *pgxpool.Pool, w http.ResponseWriter, r *http.Request, username, role string) error {
	var userID int64
	if err := db.QueryRow(ctx, `SELECT id FROM users WHERE username = $1 AND account_status = 'active'`, username).Scan(&userID); err != nil {
		return fmt.Errorf("khong tim thay tai khoan dang hoat dong")
	}

	current, _ := sessionFromRequest(ctx, db, r)
	if current != nil {
		_ = revokeSession(ctx, db, current.ID)
	}

	if _, err := db.Exec(ctx, `
		UPDATE user_sessions
		SET revoked_at = NOW()
		WHERE user_id = $1
			AND revoked_at IS NULL
			AND expires_at > NOW()
	`, userID); err != nil {
		return err
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(sessionTTL)
	if _, err := db.Exec(ctx, `
		INSERT INTO user_sessions (user_id, role_code, token_hash, user_agent, ip_address, expires_at)
		VALUES ($1, $2, $3, NULLIF($4, ''), NULLIF($5, '')::inet, $6)
	`, userID, role, tokenHash, r.UserAgent(), clientIP(r), expiresAt); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

func requireAuth(ctx context.Context, db *pgxpool.Pool, w http.ResponseWriter, r *http.Request, roles ...string) (*authSession, bool) {
	session, err := sessionFromRequest(ctx, db, r)
	if err != nil {
		http.Error(w, "Phien dang nhap khong hop le hoac da het han", http.StatusUnauthorized)
		return nil, false
	}
	if len(roles) == 0 {
		return session, true
	}
	for _, role := range roles {
		if session.Role == role {
			return session, true
		}
	}
	http.Error(w, "Tai khoan khong co quyen truy cap chuc nang nay", http.StatusForbidden)
	return nil, false
}

func sessionFromRequest(ctx context.Context, db *pgxpool.Pool, r *http.Request) (*authSession, error) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, fmt.Errorf("missing session cookie")
	}
	tokenHash := hashSessionToken(cookie.Value)
	var session authSession
	err = db.QueryRow(ctx, `
		SELECT us.id, us.user_id, u.username, us.role_code
		FROM user_sessions us
		JOIN users u ON u.id = us.user_id
		WHERE us.token_hash = $1
			AND us.revoked_at IS NULL
			AND us.expires_at > NOW()
			AND u.account_status = 'active'
	`, tokenHash).Scan(&session.ID, &session.UserID, &session.Username, &session.Role)
	if err != nil {
		return nil, err
	}
	_, _ = db.Exec(ctx, `UPDATE user_sessions SET last_seen_at = NOW() WHERE id = $1`, session.ID)
	return &session, nil
}

func logoutSession(ctx context.Context, db *pgxpool.Pool, w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		_, _ = db.Exec(ctx, `UPDATE user_sessions SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`, hashSessionToken(cookie.Value))
	}
	clearSessionCookie(w, r)
}

func revokeSession(ctx context.Context, db *pgxpool.Pool, sessionID int64) error {
	_, err := db.Exec(ctx, `UPDATE user_sessions SET revoked_at = NOW() WHERE id = $1`, sessionID)
	return err
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	})
}

func newSessionToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(buf)
	return token, hashSessionToken(token), nil
}

func hashSessionToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func requestIsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func clientIP(r *http.Request) string {
	forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0])
	if forwarded != "" {
		if ip := net.ParseIP(forwarded); ip != nil {
			return ip.String()
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return ""
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return ""
}

func ensureStudentAttemptOwner(ctx context.Context, db *pgxpool.Pool, attemptID int64, userID int64) error {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exam_attempts WHERE id = $1 AND student_user_id = $2)`, attemptID, userID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	return nil
}

func teacherOwnsExam(ctx context.Context, db *pgxpool.Pool, examIDText string, userID int64) bool {
	examID, err := strconvParseInt64(examIDText)
	if err != nil {
		return false
	}
	var exists bool
	err = db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM exams WHERE id = $1 AND created_by_user_id = $2)`, examID, userID).Scan(&exists)
	return err == nil && exists
}

func teacherOwnsBatch(ctx context.Context, db *pgxpool.Pool, batchID int64, userID int64) bool {
	var exists bool
	err := db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM import_batches WHERE id = $1 AND uploaded_by_user_id = $2)`, batchID, userID).Scan(&exists)
	return err == nil && exists
}

func strconvParseInt64(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty id")
	}
	var out int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, fmt.Errorf("invalid id")
		}
		out = out*10 + int64(ch-'0')
	}
	return out, nil
}
