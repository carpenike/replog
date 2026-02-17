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
var ErrInvalidInput = errors.New("invalid input")

// ErrDuplicateUsername is returned when a username already exists.
var ErrDuplicateUsername = errors.New("duplicate username")

// ErrAthleteAlreadyLinked is returned when an athlete is already linked to another user.
var ErrAthleteAlreadyLinked = errors.New("athlete already linked to another user")

// ErrNoPassword is returned when authenticating a user that has no password set.
var ErrNoPassword = errors.New("account has no password")

// User represents a login account in the system.
type User struct {
	ID           int64
	Username     string
	Name         sql.NullString
	Email        sql.NullString
	PasswordHash string
	AthleteID    sql.NullInt64
	IsCoach      bool
	IsAdmin      bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasPassword reports whether the user has a password set.
// Passwordless users authenticate via magic links or passkeys.
func (u *User) HasPassword() bool {
	return u.PasswordHash != ""
}

// UserWithAthlete extends User with the linked athlete's name.
type UserWithAthlete struct {
	User
	AthleteName sql.NullString
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
// is already taken. When athleteID is valid the user is linked atomically.
// If password is empty the user is created without a password (passwordless).
func CreateUser(db *sql.DB, username, name, password, email string, isCoach bool, isAdmin bool, athleteID sql.NullInt64) (*User, error) {
	var hashVal sql.NullString
	if password != "" {
		hash, err := HashPassword(password)
		if err != nil {
			return nil, err
		}
		hashVal = sql.NullString{String: hash, Valid: true}
	}

	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	var nameVal sql.NullString
	if name != "" {
		nameVal = sql.NullString{String: name, Valid: true}
	}

	coachInt := 0
	if isCoach {
		coachInt = 1
	}

	adminInt := 0
	if isAdmin {
		adminInt = 1
	}

	result, err := db.Exec(
		`INSERT INTO users (username, name, email, password_hash, is_coach, is_admin, athlete_id) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		username, nameVal, emailVal, hashVal, coachInt, adminInt, athleteID,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if errContains(err, "athlete_id") {
				return nil, ErrAthleteAlreadyLinked
			}
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
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, created_at, updated_at
		 FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
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
		`SELECT id, username, name, email, COALESCE(password_hash, ''), athlete_id, is_coach, is_admin, created_at, updated_at
		 FROM users WHERE username = ?`, username,
	).Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get user by username %q: %w", username, err)
	}
	return u, nil
}

// Authenticate verifies a username/password combination and returns the user
// if valid. Returns ErrNotFound if credentials are wrong, or ErrNoPassword
// if the account has no password set (passwordless-only).
func Authenticate(db *sql.DB, username, password string) (*User, error) {
	u, err := GetUserByUsername(db, username)
	if err != nil {
		return nil, err
	}
	if !u.HasPassword() {
		return nil, ErrNoPassword
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

// ListUsers returns all users with linked athlete names, ordered by username.
func ListUsers(db *sql.DB) ([]*UserWithAthlete, error) {
	rows, err := db.Query(`
		SELECT u.id, u.username, u.name, u.email, COALESCE(u.password_hash, ''), u.athlete_id, u.is_coach, u.is_admin, u.created_at, u.updated_at,
		       a.name
		FROM users u
		LEFT JOIN athletes a ON u.athlete_id = a.id
		ORDER BY u.username COLLATE NOCASE
		LIMIT 100
	`)
	if err != nil {
		return nil, fmt.Errorf("models: list users: %w", err)
	}
	defer rows.Close()

	var users []*UserWithAthlete
	for rows.Next() {
		u := &UserWithAthlete{}
		if err := rows.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &u.AthleteID, &u.IsCoach, &u.IsAdmin, &u.CreatedAt, &u.UpdatedAt, &u.AthleteName); err != nil {
			return nil, fmt.Errorf("models: list users scan: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// UpdateUser updates a user's profile fields (not password).
// Returns ErrDuplicateUsername if the new username conflicts.
func UpdateUser(db *sql.DB, id int64, username, name, email string, athleteID sql.NullInt64, isCoach bool, isAdmin bool) (*User, error) {
	var emailVal sql.NullString
	if email != "" {
		emailVal = sql.NullString{String: email, Valid: true}
	}

	var nameVal sql.NullString
	if name != "" {
		nameVal = sql.NullString{String: name, Valid: true}
	}

	coachInt := 0
	if isCoach {
		coachInt = 1
	}

	adminInt := 0
	if isAdmin {
		adminInt = 1
	}

	_, err := db.Exec(
		`UPDATE users SET username = ?, name = ?, email = ?, athlete_id = ?, is_coach = ?, is_admin = ? WHERE id = ?`,
		username, nameVal, emailVal, athleteID, coachInt, adminInt, id,
	)
	if err != nil {
		if isUniqueViolation(err) {
			if errContains(err, "athlete_id") {
				return nil, ErrAthleteAlreadyLinked
			}
			return nil, ErrDuplicateUsername
		}
		return nil, fmt.Errorf("models: update user %d: %w", id, err)
	}
	return GetUserByID(db, id)
}

// UpdatePassword changes a user's password hash.
func UpdatePassword(db *sql.DB, id int64, newPassword string) error {
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	result, err := db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	if err != nil {
		return fmt.Errorf("models: update password for user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes a user by ID.
func DeleteUser(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete user %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
