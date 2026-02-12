package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// ErrNotFound is returned when a query finds no matching row.
var ErrNotFound = errors.New("not found")

// ErrDuplicateUsername is returned when a username already exists.
var ErrDuplicateUsername = errors.New("duplicate username")

// User represents a login account in the system.
type User struct {
	ID           int64
	Username     string
	Email        sql.NullString
	PasswordHash string
	AthleteID    sql.NullInt64
	IsCoach      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HashPassword generates a bcrypt hash of the given plaintext password.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("models: hash password: %w", err)
	}
	return string(hash), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// CreateUser inserts a new user. Returns ErrDuplicateUsername if the username
// is already taken.
func CreateUser(db *sql.DB, username, password, email string, isCoach bool) (*User, error) {
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	coachInt := 0
	if isCoach {
		coachInt = 1
	}

	result, err := db.Exec(
		`INSERT INTO users (username, email, password_hash, is_coach) VALUES (?, ?, ?, ?)`,
		username, emailVal, hash, coachInt,
	)
	if err != nil {
		// SQLite unique constraint error contains "UNIQUE constraint failed".
		if isUniqueViolation(err) {
			return nil, ErrDuplicateUsername
		}
		return nil, fmt.Errorf("models: create user %q: %w", username, err)
	}

	id, _ := result.LastInsertId()
	return GetUserByID(db, id)
}

// GetUserByID retrieves a user by primary key.
func GetUserByID(db *sql.DB, id int64) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, email, password_hash, athlete_id, is_coach, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user %d: %w", id, err)
	}
	return u, nil
}

// GetUserByUsername retrieves a user by username (case-insensitive).
func GetUserByUsername(db *sql.DB, username string) (*User, error) {
	u := &User{}
	err := db.QueryRow(
		`SELECT id, username, email, password_hash, athlete_id, is_coach, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by username %q: %w", username, err)
	}
	return u, nil
}

// Authenticate verifies a username/password combination and returns the user
// if valid, or ErrNotFound if the credentials are wrong.
func Authenticate(db *sql.DB, username, password string) (*User, error) {
	u, err := GetUserByUsername(db, username)
	if err != nil {
		return nil, err
	}
	if !CheckPassword(u.PasswordHash, password) {
		return nil, ErrNotFound
	}
	return u, nil
}

// CountUsers returns the total number of users in the database.
func CountUsers(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("models: count users: %w", err)
	}
	return count, nil
}

// isUniqueViolation checks if a SQLite error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && (errContains(err, "UNIQUE constraint failed") || errContains(err, "constraint failed: UNIQUE"))
}

func errContains(err error, substr string) bool {
	return err != nil && contains(err.Error(), substr)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
