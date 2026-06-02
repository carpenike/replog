package models

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// MCPTokenPrefix is the human-recognizable prefix on every opaque MCP bearer
// token ("RepLog Personal Access Token"). It lets the bearer middleware
// reject obviously-non-MCP credentials with a cheap string check before
// touching the database, and makes a leaked token greppable in logs.
const MCPTokenPrefix = "rlpat_"

// MCPTokenLifetime is the validity window for a freshly minted MCP token.
// 90 days matches the long-lived-agent-credential posture in ADR 019: the
// Claude/VS Code client holds one token across many sessions and only
// re-runs the OAuth dance when it expires or is revoked.
const MCPTokenLifetime = 90 * 24 * time.Hour

// MCPToken is a stored opaque bearer token. The plaintext secret is never
// persisted — only its SHA-256 hash — so a database leak does not yield
// usable credentials.
type MCPToken struct {
	ID            int64
	UserID        int64
	OAuthClientID sql.NullString
	Label         sql.NullString
	ExpiresAt     time.Time
	RevokedAt     sql.NullTime
	LastUsedAt    sql.NullTime
	CreatedAt     time.Time
}

// hashMCPSecret returns the lowercase-hex SHA-256 of an opaque secret. Used
// for both token storage/lookup and DCR client-secret comparison.
func hashMCPSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// CreateMCPToken mints a new opaque bearer token for a user and returns the
// plaintext secret EXACTLY ONCE — it is not recoverable afterward (only the
// hash is stored). oauthClientID/label are optional audit metadata.
func CreateMCPToken(db *sql.DB, userID int64, oauthClientID, label string) (plaintext string, token *MCPToken, err error) {
	random, err := generateToken(32) // 256-bit secret
	if err != nil {
		return "", nil, err
	}
	plaintext = MCPTokenPrefix + random
	hash := hashMCPSecret(plaintext)

	now := time.Now()
	expires := now.Add(MCPTokenLifetime)

	var clientVal, labelVal sql.NullString
	if oauthClientID != "" {
		clientVal = sql.NullString{String: oauthClientID, Valid: true}
	}
	if label != "" {
		labelVal = sql.NullString{String: label, Valid: true}
	}

	res, err := db.Exec(
		`INSERT INTO mcp_tokens (token_hash, user_id, oauth_client_id, label, expires_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		hash, userID, clientVal, labelVal, expires, now,
	)
	if err != nil {
		return "", nil, errors.New("models: create mcp token: " + err.Error())
	}
	id, _ := res.LastInsertId()

	return plaintext, &MCPToken{
		ID:            id,
		UserID:        userID,
		OAuthClientID: clientVal,
		Label:         labelVal,
		ExpiresAt:     expires,
		CreatedAt:     now,
	}, nil
}

// ValidateMCPToken resolves a presented opaque token to its owning user. It
// returns ErrNotFound when the token is malformed, unknown, revoked, or
// expired — the caller maps that to a 401. On success it bumps last_used_at
// (best-effort) and returns the user.
//
// The presented string must already carry the MCPTokenPrefix; the middleware
// gates on the prefix before calling this, but we re-check defensively so a
// direct model caller cannot bypass it.
func ValidateMCPToken(db *sql.DB, presented string) (*User, error) {
	if !strings.HasPrefix(presented, MCPTokenPrefix) {
		return nil, ErrNotFound
	}
	hash := hashMCPSecret(presented)

	var (
		id        int64
		userID    int64
		expiresAt time.Time
		revokedAt sql.NullTime
	)
	err := db.QueryRow(
		`SELECT id, user_id, expires_at, revoked_at FROM mcp_tokens WHERE token_hash = ?`,
		hash,
	).Scan(&id, &userID, &expiresAt, &revokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, errors.New("models: validate mcp token: " + err.Error())
	}
	if revokedAt.Valid {
		return nil, ErrNotFound
	}
	if time.Now().After(expiresAt) {
		return nil, ErrNotFound
	}

	user, err := GetUserByID(db, userID)
	if err != nil {
		return nil, err
	}

	// Best-effort usage stamp; a failure here must not fail the request.
	_, _ = db.Exec(`UPDATE mcp_tokens SET last_used_at = ? WHERE id = ?`, time.Now(), id)

	return user, nil
}

// RevokeMCPToken soft-deletes a token by id (sets revoked_at). Idempotent.
func RevokeMCPToken(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE mcp_tokens SET revoked_at = ? WHERE id = ? AND revoked_at IS NULL`, time.Now(), id)
	if err != nil {
		return errors.New("models: revoke mcp token: " + err.Error())
	}
	return nil
}
