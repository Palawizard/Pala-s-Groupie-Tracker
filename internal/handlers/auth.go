package handlers

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"html/template"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"palasgroupietracker/internal/store"
)

const sessionCookieName = "gt_session"
const sessionDuration = 14 * 24 * time.Hour

// AuthPageData powers the login and register pages.
type AuthPageData struct {
	Title      string
	Source     string
	ActiveNav  string
	BasePath   string
	CurrentURL string
	User       *store.User
	IsAuthed   bool

	Email   string
	Error   string
	NextURL string
}

// LoginHandler renders and processes the login form.
func LoginHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)

	if r.Method == http.MethodPost {
		handleLoginPost(w, r)
		return
	}

	user, authed := getCurrentUser(w, r)
	if authed {
		http.Redirect(w, r, resolveNextURL(r.URL.Query().Get("next"), r), http.StatusSeeOther)
		return
	}

	data := AuthPageData{
		Title:      "Login",
		Source:     source,
		ActiveNav:  "",
		BasePath:   basePath,
		CurrentURL: buildCurrentURL(r),
		User:       user,
		IsAuthed:   authed,
		Email:      "",
		Error:      "",
		NextURL:    resolveNextURL(r.URL.Query().Get("next"), r),
	}

	renderAuthTemplate(w, data, "web/templates/login.gohtml")
}

// RegisterHandler renders and processes the registration form.
func RegisterHandler(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)

	if r.Method == http.MethodPost {
		handleRegisterPost(w, r)
		return
	}

	user, authed := getCurrentUser(w, r)
	if authed {
		http.Redirect(w, r, resolveNextURL(r.URL.Query().Get("next"), r), http.StatusSeeOther)
		return
	}

	data := AuthPageData{
		Title:      "Create account",
		Source:     source,
		ActiveNav:  "",
		BasePath:   basePath,
		CurrentURL: buildCurrentURL(r),
		User:       user,
		IsAuthed:   authed,
		Email:      "",
		Error:      "",
		NextURL:    resolveNextURL(r.URL.Query().Get("next"), r),
	}

	renderAuthTemplate(w, data, "web/templates/register.gohtml")
}

// LogoutHandler clears the session cookie and deletes the server session.
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

func handleLoginPost(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)
	currentURL := buildCurrentURL(r)

	if appStore == nil {
		data := AuthPageData{
			Title:      "Login",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			User:       nil,
			IsAuthed:   false,
			Email:      "",
			Error:      "Database is not configured.",
			NextURL:    resolveNextURL(r.FormValue("next"), r),
		}
		renderAuthTemplate(w, data, "web/templates/login.gohtml")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	next := resolveNextURL(r.FormValue("next"), r)

	if email == "" || password == "" {
		data := AuthPageData{
			Title:      "Login",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			Email:      email,
			Error:      "Email and password are required.",
			NextURL:    next,
		}
		renderAuthTemplate(w, data, "web/templates/login.gohtml")
		return
	}

	user, err := appStore.GetUserByEmail(r.Context(), email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			data := AuthPageData{
				Title:      "Login",
				Source:     source,
				ActiveNav:  "",
				BasePath:   basePath,
				CurrentURL: currentURL,
				Email:      email,
				Error:      "Invalid email or password.",
				NextURL:    next,
			}
			renderAuthTemplate(w, data, "web/templates/login.gohtml")
			return
		}
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		data := AuthPageData{
			Title:      "Login",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			Email:      email,
			Error:      "Invalid email or password.",
			NextURL:    next,
		}
		renderAuthTemplate(w, data, "web/templates/login.gohtml")
		return
	}

	if err := createSession(w, r, user.ID); err != nil {
		http.Error(w, "login failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
}

func handleRegisterPost(w http.ResponseWriter, r *http.Request) {
	source := getSource(r)
	basePath := getBasePath(r)
	currentURL := buildCurrentURL(r)

	if appStore == nil {
		data := AuthPageData{
			Title:      "Create account",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			User:       nil,
			IsAuthed:   false,
			Email:      "",
			Error:      "Database is not configured.",
			NextURL:    resolveNextURL(r.FormValue("next"), r),
		}
		renderAuthTemplate(w, data, "web/templates/register.gohtml")
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")
	next := resolveNextURL(r.FormValue("next"), r)

	if email == "" || password == "" {
		data := AuthPageData{
			Title:      "Create account",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			Email:      email,
			Error:      "Email and password are required.",
			NextURL:    next,
		}
		renderAuthTemplate(w, data, "web/templates/register.gohtml")
		return
	}

	if len(password) < 8 {
		data := AuthPageData{
			Title:      "Create account",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			Email:      email,
			Error:      "Password must be at least 8 characters.",
			NextURL:    next,
		}
		renderAuthTemplate(w, data, "web/templates/register.gohtml")
		return
	}

	if confirm != "" && confirm != password {
		data := AuthPageData{
			Title:      "Create account",
			Source:     source,
			ActiveNav:  "",
			BasePath:   basePath,
			CurrentURL: currentURL,
			Email:      email,
			Error:      "Passwords do not match.",
			NextURL:    next,
		}
		renderAuthTemplate(w, data, "web/templates/register.gohtml")
		return
	}

	hashBytes, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	if err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	user, err := appStore.CreateUser(r.Context(), email, string(hashBytes))
	if err != nil {
		if errors.Is(err, store.ErrEmailExists) {
			data := AuthPageData{
				Title:      "Create account",
				Source:     source,
				ActiveNav:  "",
				BasePath:   basePath,
				CurrentURL: currentURL,
				Email:      email,
				Error:      "Email already exists.",
				NextURL:    next,
			}
			renderAuthTemplate(w, data, "web/templates/register.gohtml")
			return
		}
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	if err := createSession(w, r, user.ID); err != nil {
		http.Error(w, "registration failed", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, next, http.StatusSeeOther)
}

func renderAuthTemplate(w http.ResponseWriter, data AuthPageData, pageTemplate string) {
	tmpl, err := templateWithLayout(pageTemplate)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}

	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "render error", http.StatusInternalServerError)
		return
	}
}

// templateWithLayout loads layout + page template.
func templateWithLayout(pageTemplate string) (*template.Template, error) {
	return template.ParseFiles(
		"web/templates/layout.gohtml",
		pageTemplate,
	)
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

// getCurrentUser resolves the logged-in user from the session cookie.
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

// buildCurrentURL builds a base-path aware URL for the current request.
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

// buildArtistsListURL builds a return URL to the artists list for ajax contexts.
func buildArtistsListURL(r *http.Request) string {
	url := withBasePath(r, "/artists")
	if r.URL.RawQuery != "" {
		url += "?" + r.URL.RawQuery
	}
	return url
}

// resolveNextURL sanitizes a return path to prevent open redirects.
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
