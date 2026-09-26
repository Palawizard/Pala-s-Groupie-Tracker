package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"palasgroupietracker/internal/store"
)

const sessionCookieName = "gt_session"
const sessionDuration = 14 * 24 * time.Hour

// LoginHandler starts the Authentik (OIDC) sign-in. Local passwords are no longer supported.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	startAuth(w, r, false)
}

// RegisterHandler opens Authentik's account creation page, then signs the new user in.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	startAuth(w, r, true)
}

func startAuth(w http.ResponseWriter, r *http.Request, signup bool) {
	next := resolveNextURL(r.URL.Query().Get("next"), r)
	if _, authed := getCurrentUser(w, r); authed {
		http.Redirect(w, r, next, http.StatusSeeOther)
		return
	}
	startOIDCLogin(w, r, next, signup)
}

// LogoutHandler clears the session cookie and deletes the server session
func LogoutHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, withBasePath(r, "/"), http.StatusSeeOther)
		return
	}

	if appStore != nil {
		if cookie, err := r.Cookie(sessionCookieName); err == nil {
			tokenHash := hashToken(cookie.Value)
			_ = appStore.DeleteSessionByTokenHash(r.Context(), tokenHash)
		}
	}

	clearSessionCookie(w, r)
	http.Redirect(w, r, withBasePath(r, "/")+"?source="+getSource(r), http.StatusSeeOther)
}

func createSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	if appStore == nil {
		return errors.New("store not configured")
	}

	token, tokenHash, err := newSessionToken()
	if err != nil {
		return err
	}

	expiresAt := time.Now().Add(sessionDuration)
	if _, err := appStore.CreateSession(r.Context(), userID, tokenHash, expiresAt); err != nil {
		return err
	}

	setSessionCookie(w, r, token, expiresAt)
	return nil
}

func newSessionToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}

	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     sessionCookiePath(r),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     sessionCookiePath(r),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

func sessionCookiePath(r *http.Request) string {
	base := getBasePath(r)
	if base == "" {
		return "/"
	}
	return base + "/"
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")))
	return proto == "https"
}

// getCurrentUser resolves the logged-in user from the session cookie
func getCurrentUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	if appStore == nil {
		return nil, false
	}

	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return nil, false
	}

	tokenHash := hashToken(cookie.Value)
	sess, err := appStore.GetSessionByTokenHash(r.Context(), tokenHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			clearSessionCookie(w, r)
		}
		return nil, false
	}

	if time.Now().After(sess.ExpiresAt) {
		_ = appStore.DeleteSessionByTokenHash(r.Context(), tokenHash)
		clearSessionCookie(w, r)
		return nil, false
	}

	user, err := appStore.GetUserByID(r.Context(), sess.UserID)
	if err != nil {
		return nil, false
	}

	return user, true
}

// buildCurrentURL builds a base-path aware URL for the current request
func buildCurrentURL(r *http.Request) string {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	base := getBasePath(r)
	if base != "" && (path == base || strings.HasPrefix(path, base+"/")) {
		base = ""
	}

	full := base + path
	if r.URL.RawQuery != "" {
		full += "?" + r.URL.RawQuery
	}
	return full
}

// buildArtistsListURL builds a return URL to the artists list for ajax contexts
func buildArtistsListURL(r *http.Request) string {
	url := withBasePath(r, "/artists")
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	return url
}

// resolveNextURL sanitizes a return path to prevent open redirects
func resolveNextURL(next string, r *http.Request) string {
	next = strings.TrimSpace(next)
	if next == "" {
		return withBasePath(r, "/")
	}
	if strings.Contains(next, "://") || strings.HasPrefix(next, "//") {
		return withBasePath(r, "/")
	}
	if !strings.HasPrefix(next, "/") {
		next = "/" + next
	}

	base := getBasePath(r)
	if base != "" && !strings.HasPrefix(next, base+"/") && next != base {
		next = base + next
	}

	return next
}
