package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/lib/pq"
)

var ErrNoDatabaseURL = errors.New("database url not set")
var ErrEmailExists = errors.New("email already exists")

// Store wraps the database connection and basic CRUD helpers
type Store struct {
	DB *sql.DB
}

var sslmodePreferRe = regexp.MustCompile(`(?i)(\bsslmode\s*=\s*)('?)(prefer|allow)\2`)

func normalizePostgresDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return dsn
	}

	// URL form: postgres://.../db?sslmode=...
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return dsn
		}
		q := u.Query()
		switch strings.ToLower(q.Get("sslmode")) {
		case "prefer", "allow":
			// lib/pq doesn't support sslmode=prefer; require is the closest equivalent for managed DBs.
			q.Set("sslmode", "require")
			u.RawQuery = q.Encode()
			return u.String()
		default:
			return dsn
		}
	}

	// Keyword/value form: "host=... user=... sslmode=prefer ..."
	return sslmodePreferRe.ReplaceAllString(dsn, `${1}${2}require${2}`)
}

// OpenFromEnv opens a Postgres connection using DATABASE_URL or SCALINGO_POSTGRESQL_URL
func OpenFromEnv(ctx context.Context) (*Store, error) {
	dsn := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("SCALINGO_POSTGRESQL_URL"))
	}
	if dsn == "" {
		return nil, ErrNoDatabaseURL
	}

	dsn = normalizePostgresDSN(dsn)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	s := &Store{DB: db}
	if err := s.Migrate(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

// Close closes the underlying database connection
func (s *Store) Close() error {
	if s == nil || s.DB == nil {
		return nil
	}
	return s.DB.Close()
}

// Migrate ensures the minimal schema exists
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.DB == nil {
		return errors.New("store not initialized")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS users (
            id BIGSERIAL PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            password_hash TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
        );`,
		`CREATE TABLE IF NOT EXISTS sessions (
            id BIGSERIAL PRIMARY KEY,
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            token_hash TEXT NOT NULL UNIQUE,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            expires_at TIMESTAMPTZ NOT NULL
        );`,
		`CREATE INDEX IF NOT EXISTS sessions_user_id_idx ON sessions(user_id);`,
		`CREATE TABLE IF NOT EXISTS favorites (
            user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
            source TEXT NOT NULL CHECK (source IN ('groupie','spotify','deezer','apple')),
            artist_id TEXT NOT NULL,
            created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
            PRIMARY KEY (user_id, source, artist_id)
        );`,
		`CREATE INDEX IF NOT EXISTS favorites_source_idx ON favorites(source);`,
	}

	for _, stmt := range statements {
		if _, err := s.DB.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	return nil
}

// User represents an account in the database
type User struct {
	ID           int64
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

// Session represents a persisted login session
type Session struct {
	ID        int64
	UserID    int64
	TokenHash string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Favorite represents a user's saved artist for a given source
type Favorite struct {
	UserID    int64
	Source    string
	ArtistID  string
	CreatedAt time.Time
}

// CreateUser inserts a new user, returning ErrEmailExists on duplicates
func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, errors.New("email required")
	}

	var u User
	err := s.DB.QueryRowContext(ctx, `
        INSERT INTO users (email, password_hash)
        VALUES ($1, $2)
        RETURNING id, email, password_hash, created_at
    `, normalized, passwordHash).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, ErrEmailExists
		}
		return nil, err
	}

	return &u, nil
}

// GetUserByEmail fetches a user by email
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	normalized := strings.ToLower(strings.TrimSpace(email))
	if normalized == "" {
		return nil, sql.ErrNoRows
	}

	var u User
	err := s.DB.QueryRowContext(ctx, `
        SELECT id, email, password_hash, created_at
        FROM users
        WHERE email = $1
    `, normalized).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetUserByID fetches a user by ID
func (s *Store) GetUserByID(ctx context.Context, id int64) (*User, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	var u User
	err := s.DB.QueryRowContext(ctx, `
        SELECT id, email, password_hash, created_at
        FROM users
        WHERE id = $1
    `, id).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// CreateSession inserts a new session record
func (s *Store) CreateSession(ctx context.Context, userID int64, tokenHash string, expiresAt time.Time) (*Session, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	var sess Session
	err := s.DB.QueryRowContext(ctx, `
        INSERT INTO sessions (user_id, token_hash, expires_at)
        VALUES ($1, $2, $3)
        RETURNING id, user_id, token_hash, created_at, expires_at
    `, userID, tokenHash, expiresAt).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &sess, nil
}

// GetSessionByTokenHash fetches a session by token hash
func (s *Store) GetSessionByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	var sess Session
	err := s.DB.QueryRowContext(ctx, `
        SELECT id, user_id, token_hash, created_at, expires_at
        FROM sessions
        WHERE token_hash = $1
    `, tokenHash).Scan(&sess.ID, &sess.UserID, &sess.TokenHash, &sess.CreatedAt, &sess.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return &sess, nil
}

// DeleteSessionByTokenHash deletes a session by token hash
func (s *Store) DeleteSessionByTokenHash(ctx context.Context, tokenHash string) error {
	if s == nil || s.DB == nil {
		return errors.New("store not initialized")
	}

	_, err := s.DB.ExecContext(ctx, `
        DELETE FROM sessions WHERE token_hash = $1
    `, tokenHash)
	return err
}

// ListFavoriteIDsBySource returns artist IDs for a user and source
func (s *Store) ListFavoriteIDsBySource(ctx context.Context, userID int64, source string) ([]string, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	rows, err := s.DB.QueryContext(ctx, `
        SELECT artist_id
        FROM favorites
        WHERE user_id = $1 AND source = $2
        ORDER BY created_at DESC
    `, userID, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return ids, nil
}

// ListFavorites returns all favorites for a user
func (s *Store) ListFavorites(ctx context.Context, userID int64) ([]Favorite, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("store not initialized")
	}

	rows, err := s.DB.QueryContext(ctx, `
        SELECT user_id, source, artist_id, created_at
        FROM favorites
        WHERE user_id = $1
        ORDER BY created_at DESC
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Favorite
	for rows.Next() {
		var fav Favorite
		if err := rows.Scan(&fav.UserID, &fav.Source, &fav.ArtistID, &fav.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, fav)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

// IsFavorite reports whether a user has saved an artist for a source
func (s *Store) IsFavorite(ctx context.Context, userID int64, source, artistID string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("store not initialized")
	}

	var exists bool
	err := s.DB.QueryRowContext(ctx, `
        SELECT EXISTS (
            SELECT 1
            FROM favorites
            WHERE user_id = $1 AND source = $2 AND artist_id = $3
        )
    `, userID, source, artistID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// ToggleFavorite inserts or removes a favorite and returns true if added
func (s *Store) ToggleFavorite(ctx context.Context, userID int64, source, artistID string) (bool, error) {
	if s == nil || s.DB == nil {
		return false, errors.New("store not initialized")
	}

	res, err := s.DB.ExecContext(ctx, `
        DELETE FROM favorites
        WHERE user_id = $1 AND source = $2 AND artist_id = $3
    `, userID, source, artistID)
	if err != nil {
		return false, err
	}

	if rows, _ := res.RowsAffected(); rows > 0 {
		return false, nil
	}

	_, err = s.DB.ExecContext(ctx, `
        INSERT INTO favorites (user_id, source, artist_id)
        VALUES ($1, $2, $3)
    `, userID, source, artistID)
	if err != nil {
		return false, err
	}

	return true, nil
}
