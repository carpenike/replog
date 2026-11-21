package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
)

// ServiceTokenPrefix is the human-recognizable prefix on every coach
// service token. The bearer middleware rejects any other prefix before
// it touches the database, so an MCP token (rlpat_) cannot be replayed
// against the REST API as this credential.
const ServiceTokenPrefix = "rlcst_"

// coachServiceTokenLabel marks the single env-installed credential.
// Rotating REPLOG_SERVICE_TOKEN replaces the previous row with this label
// and leaves any other labels (tests, a future minted token) alone.
const coachServiceTokenLabel = "coach-service"

// validPresentedServiceToken reports whether presented is a token we
// are willing to hash and look up: the prefix, a long enough secret, and
// no whitespace. A failure here is not a server error.
func validPresentedServiceToken(presented string) bool {
	if !strings.HasPrefix(presented, ServiceTokenPrefix) {
		return false
	}
	rest := presented[len(ServiceTokenPrefix):]
	if len(rest) < 16 {
		return false
	}
	for _, r := range rest {
		if r > unicode.MaxASCII || unicode.IsSpace(r) || !unicode.IsPrint(r) {
			return false
		}
	}
	return true
}

// CreateServiceToken mints a new opaque bearer for userID and returns the
// plaintext exactly once. Only the SHA-256 hash is inserted.
func CreateServiceToken(ctx context.Context, db *sql.DB, userID int64, label string) (string, error) {
	random, err := generateToken(32)
	if err != nil {
		return "", err
	}
	plaintext := ServiceTokenPrefix + random
	if err := insertServiceToken(ctx, db, userID, plaintext, label); err != nil {
		return "", err
	}
	return plaintext, nil
}

// InstallCoachServiceToken stores the hash of a caller-supplied secret
// (REPLOG_SERVICE_TOKEN) for userID. The plaintext is not written. Calling
// it again with the same secret is a no-op; a different secret replaces
// the previous coach-service row for that user so the old bearer stops
// working. The hash is never logged by this function.
func InstallCoachServiceToken(ctx context.Context, db *sql.DB, userID int64, plaintext string) error {
	if !validPresentedServiceToken(plaintext) {
		return fmt.Errorf("models: install service token: secret must be a %s token", ServiceTokenPrefix)
	}
	hash := hashMCPSecret(plaintext)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("models: install service token: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var owner int64
	err = tx.QueryRowContext(ctx, `SELECT user_id FROM service_tokens WHERE token_hash = ?`, hash).Scan(&owner)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO service_tokens (token_hash, user_id, label, created_at) VALUES (?, ?, ?, ?)`,
			hash, userID, coachServiceTokenLabel, time.Now(),
		); err != nil {
			return fmt.Errorf("models: install service token: %w", err)
		}
	case err != nil:
		return fmt.Errorf("models: install service token: %w", err)
	case owner != userID:
		return errors.New("models: install service token: hash is already bound to a different user")
	default:
		if _, err = tx.ExecContext(ctx,
			`UPDATE service_tokens SET revoked_at = NULL, label = ? WHERE token_hash = ?`,
			coachServiceTokenLabel, hash,
		); err != nil {
			return fmt.Errorf("models: install service token: %w", err)
		}
	}

	if _, err = tx.ExecContext(ctx,
		`DELETE FROM service_tokens WHERE user_id = ? AND label = ? AND token_hash != ?`,
		userID, coachServiceTokenLabel, hash,
	); err != nil {
		return fmt.Errorf("models: install service token: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("models: install service token: %w", err)
	}
	return nil
}

func insertServiceToken(ctx context.Context, db *sql.DB, userID int64, plaintext, label string) error {
	if !validPresentedServiceToken(plaintext) {
		return errors.New("models: create service token: secret is not a service token")
	}
	var labelVal sql.NullString
	if label != "" {
		labelVal = sql.NullString{String: label, Valid: true}
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO service_tokens (token_hash, user_id, label, created_at) VALUES (?, ?, ?, ?)`,
		hashMCPSecret(plaintext), userID, labelVal, time.Now(),
	)
	if err != nil {
		return fmt.Errorf("models: create service token: %w", err)
	}
	return nil
}

// ValidateServiceToken resolves a presented bearer to its user. It returns
// ErrNotFound when the token is malformed, unknown, or revoked. On success
// it bumps last_used_at (best-effort). The plaintext is not retained.
func ValidateServiceToken(ctx context.Context, db *sql.DB, presented string) (*User, error) {
	if !validPresentedServiceToken(presented) {
		return nil, ErrNotFound
	}
	var (
		id        int64
		userID    int64
		revokedAt sql.NullTime
	)
	err := db.QueryRowContext(ctx,
		`SELECT id, user_id, revoked_at FROM service_tokens WHERE token_hash = ?`,
		hashMCPSecret(presented),
	).Scan(&id, &userID, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: validate service token: %w", err)
	}
	if revokedAt.Valid {
		return nil, ErrNotFound
	}

	user, err := GetUserByID(ctx, db, userID)
	if err != nil {
		return nil, err
	}

	_, _ = db.ExecContext(ctx, `UPDATE service_tokens SET last_used_at = ? WHERE id = ?`, time.Now(), id)
	return user, nil
}

// RevokeServiceToken soft-deletes a token by id. Idempotent.
func RevokeServiceToken(ctx context.Context, db *sql.DB, id int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE service_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`,
		time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("models: revoke service token: %w", err)
	}
	return nil
}
