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

// NewConfirmToken returns a new opaque token with prefix and the
// SHA-256 hex of that token. The plaintext is not stored by this function.
func NewConfirmToken(prefix string) (token, hash string, err error) {
	random, err := generateToken(32)
	if err != nil {
		return "", "", err
	}
	token = prefix + random
	return token, hashMCPSecret(token), nil
}

// HashConfirmToken is the SHA-256 hex of a presented confirm token.
func HashConfirmToken(token string) string {
	return hashMCPSecret(token)
}

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
// plaintext exactly once, plus the new row id. Only the SHA-256 hash is inserted.
func CreateServiceToken(ctx context.Context, db *sql.DB, userID int64, label string) (string, int64, error) {
	random, err := generateToken(32)
	if err != nil {
		return "", 0, err
	}
	plaintext := ServiceTokenPrefix + random
	id, err := insertServiceToken(ctx, db, userID, plaintext, label)
	if err != nil {
		return "", 0, err
	}
	return plaintext, id, nil
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

func insertServiceToken(ctx context.Context, db *sql.DB, userID int64, plaintext, label string) (int64, error) {
	if !validPresentedServiceToken(plaintext) {
		return 0, errors.New("models: create service token: secret is not a service token")
	}
	var labelVal sql.NullString
	if label != "" {
		labelVal = sql.NullString{String: label, Valid: true}
	}
	var id int64
	err := db.QueryRowContext(ctx,
		`INSERT INTO service_tokens (token_hash, user_id, label, created_at) VALUES (?, ?, ?, ?) RETURNING id`,
		hashMCPSecret(plaintext), userID, labelVal, time.Now(),
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("models: create service token: %w", err)
	}
	return id, nil
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

// ServiceTokenInfo is a service token without its hash. The plaintext is
// never stored and is not part of this value.
type ServiceTokenInfo struct {
	ID         int64
	UserID     int64
	Label      sql.NullString
	RevokedAt  sql.NullTime
	LastUsedAt sql.NullTime
	CreatedAt  time.Time
}

// ListServiceTokens returns every token row for a user, including revoked
// rows. The hash is not selected.
func ListServiceTokens(ctx context.Context, db *sql.DB, userID int64) ([]ServiceTokenInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, user_id, label, revoked_at, last_used_at, created_at
		  FROM service_tokens
		 WHERE user_id = ?
		 ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("models: list service tokens for user %d: %w", userID, err)
	}
	defer rows.Close()

	var out []ServiceTokenInfo
	for rows.Next() {
		var info ServiceTokenInfo
		if err := rows.Scan(&info.ID, &info.UserID, &info.Label, &info.RevokedAt, &info.LastUsedAt, &info.CreatedAt); err != nil {
			return nil, fmt.Errorf("models: scan service token: %w", err)
		}
		out = append(out, info)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeServiceTokenForUser soft-deletes one token that belongs to userID.
// A token id for a different user is ErrNotFound. Revoking an
// already-revoked token of this user is a no-op.
func RevokeServiceTokenForUser(ctx context.Context, db *sql.DB, id, userID int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE service_tokens SET revoked_at = ? WHERE id = ? AND user_id = ? AND revoked_at IS NULL`,
		time.Now(), id, userID,
	)
	if err != nil {
		return fmt.Errorf("models: revoke service token %d: %w", id, err)
	}
	rows, _ := res.RowsAffected()
	if rows > 0 {
		return nil
	}
	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM service_tokens WHERE id = ? AND user_id = ?`, id, userID,
	).Scan(&n); err != nil {
		return fmt.Errorf("models: revoke service token %d: %w", id, err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
