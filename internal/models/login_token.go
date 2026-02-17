package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// LoginToken represents a passwordless login token for device-based access.
type LoginToken struct {
	ID        int64
	UserID    int64
	Token     string
	Label     sql.NullString
	ExpiresAt sql.NullTime
	CreatedAt time.Time
}

// LoginTokenWithUser extends LoginToken with the associated username.
type LoginTokenWithUser struct {
	LoginToken
	Username string
}

// IsExpired returns true if the token has an expiry date that has passed.
func (t *LoginToken) IsExpired() bool {
	return t.ExpiresAt.Valid && t.ExpiresAt.Time.Before(time.Now())
}

// generateToken creates a cryptographically secure random hex string.
func generateToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("models: generate token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// CreateLoginToken generates a new login token for the given user.
// Label is optional (e.g. "iPad", "iPhone"). ExpiresAt is optional — nil means no expiry.
func CreateLoginToken(db *sql.DB, userID int64, label string, expiresAt *time.Time) (*LoginToken, error) {
	token, err := generateToken(32) // 256-bit token
	if err != nil {
		return nil, err
	}

	var labelVal sql.NullString
	if label != "" {
		labelVal = sql.NullString{String: label, Valid: true}
	}

	var expiresVal sql.NullTime
	if expiresAt != nil {
		expiresVal = sql.NullTime{Time: *expiresAt, Valid: true}
	}

	result, err := db.Exec(
		`INSERT INTO login_tokens (user_id, token, label, expires_at) VALUES (?, ?, ?, ?)`,
		userID, token, labelVal, expiresVal,
	)
	if err != nil {
		return nil, fmt.Errorf("models: create login token for user %d: %w", userID, err)
	}

	id, _ := result.LastInsertId()
	return &LoginToken{
		ID:        id,
		UserID:    userID,
		Token:     token,
		Label:     labelVal,
		ExpiresAt: expiresVal,
		CreatedAt: time.Now(),
	}, nil
}

// ValidateLoginToken looks up a token and returns the associated user if the
// token is valid and not expired. Returns ErrNotFound if invalid or expired.
func ValidateLoginToken(db *sql.DB, token string) (*User, error) {
	lt := &LoginToken{}
	err := db.QueryRow(
		`SELECT id, user_id, token, label, expires_at, created_at
		 FROM login_tokens WHERE token = ?`, token,
	).Scan(&lt.ID, &lt.UserID, &lt.Token, &lt.Label, &lt.ExpiresAt, &lt.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: validate login token: %w", err)
	}

	if lt.IsExpired() {
		return nil, ErrNotFound
	}

	user, err := GetUserByID(db, lt.UserID)
	if err != nil {
		return nil, fmt.Errorf("models: validate login token get user: %w", err)
	}

	return user, nil
}

// ListLoginTokensByUser returns all login tokens for a given user, ordered by creation date.
func ListLoginTokensByUser(db *sql.DB, userID int64) ([]*LoginToken, error) {
	rows, err := db.Query(
		`SELECT id, user_id, token, label, expires_at, created_at
		 FROM login_tokens WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list login tokens for user %d: %w", userID, err)
	}
	defer rows.Close()

	var tokens []*LoginToken
	for rows.Next() {
		t := &LoginToken{}
		if err := rows.Scan(&t.ID, &t.UserID, &t.Token, &t.Label, &t.ExpiresAt, &t.CreatedAt); err != nil {
			return nil, fmt.Errorf("models: list login tokens scan: %w", err)
		}
		tokens = append(tokens, t)
	}
	return tokens, rows.Err()
}

// DeleteLoginToken removes a login token by ID. Returns ErrNotFound if the
// token does not exist.
func DeleteLoginToken(db *sql.DB, id int64) error {
	result, err := db.Exec(`DELETE FROM login_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("models: delete login token %d: %w", id, err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteLoginTokensByUser removes all login tokens for a user.
func DeleteLoginTokensByUser(db *sql.DB, userID int64) error {
	_, err := db.Exec(`DELETE FROM login_tokens WHERE user_id = ?`, userID)
	if err != nil {
		return fmt.Errorf("models: delete login tokens for user %d: %w", userID, err)
	}
	return nil
}
