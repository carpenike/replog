package models

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// DCRClient is an OAuth client registered via Dynamic Client Registration
// (RFC 7591). client_secret is stored only as a SHA-256 hash; the plaintext
// is returned once at registration.
type DCRClient struct {
	ClientID                string
	ClientSecretHash        string
	ClientName              string
	RedirectURIs            []string
	TokenEndpointAuthMethod string
	CreatedAt               time.Time
}

// RegisterDCRClient creates a new DCR client and returns the plaintext client
// secret EXACTLY ONCE (only the hash is stored). redirectURIs must already be
// allowlist-filtered by the caller; authMethod defaults to client_secret_post
// when empty.
func RegisterDCRClient(ctx context.Context, db *sql.DB, clientName string, redirectURIs []string, authMethod string) (client *DCRClient, plaintextSecret string, err error) {
	clientID, err := generateToken(16) // 128-bit public identifier
	if err != nil {
		return nil, "", err
	}
	plaintextSecret, err = generateToken(32) // 256-bit secret
	if err != nil {
		return nil, "", err
	}
	if authMethod == "" {
		authMethod = "client_secret_post"
	}
	if redirectURIs == nil {
		redirectURIs = []string{}
	}

	urisJSON, err := json.Marshal(redirectURIs)
	if err != nil {
		return nil, "", errors.New("models: marshal redirect_uris: " + err.Error())
	}
	hash := hashMCPSecret(plaintextSecret)
	now := time.Now()

	_, err = db.ExecContext(ctx,
		`INSERT INTO dcr_clients (client_id, client_secret_hash, client_name, redirect_uris, token_endpoint_auth_method, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		clientID, hash, clientName, string(urisJSON), authMethod, now,
	)
	if err != nil {
		return nil, "", errors.New("models: register dcr client: " + err.Error())
	}

	return &DCRClient{
		ClientID:                clientID,
		ClientSecretHash:        hash,
		ClientName:              clientName,
		RedirectURIs:            redirectURIs,
		TokenEndpointAuthMethod: authMethod,
		CreatedAt:               now,
	}, plaintextSecret, nil
}

// GetDCRClient looks up a registered client by its public client_id. Returns
// ErrNotFound when the client is unknown.
func GetDCRClient(ctx context.Context, db *sql.DB, clientID string) (*DCRClient, error) {
	var (
		c        DCRClient
		urisJSON string
	)
	err := db.QueryRowContext(ctx,
		`SELECT client_id, client_secret_hash, client_name, redirect_uris, token_endpoint_auth_method, created_at
		 FROM dcr_clients WHERE client_id = ?`,
		clientID,
	).Scan(&c.ClientID, &c.ClientSecretHash, &c.ClientName, &urisJSON, &c.TokenEndpointAuthMethod, &c.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, errors.New("models: get dcr client: " + err.Error())
	}
	if err := json.Unmarshal([]byte(urisJSON), &c.RedirectURIs); err != nil {
		return nil, errors.New("models: unmarshal redirect_uris: " + err.Error())
	}
	return &c, nil
}

// ValidateDCRClientSecret returns the client when the presented secret matches
// the stored hash, using a constant-time comparison to avoid leaking the
// secret via timing. Returns ErrNotFound for an unknown client or a mismatch.
func ValidateDCRClientSecret(ctx context.Context, db *sql.DB, clientID, secret string) (*DCRClient, error) {
	c, err := GetDCRClient(ctx, db, clientID)
	if err != nil {
		return nil, err
	}
	presented := hashMCPSecret(secret)
	if subtle.ConstantTimeCompare([]byte(presented), []byte(c.ClientSecretHash)) != 1 {
		return nil, ErrNotFound
	}
	return c, nil
}

// DeleteOrphanDCRClients hard-deletes DCR clients that were registered before
// the given cutoff AND have never produced a surviving mcp_token. A Dynamic
// Client Registration where the user abandoned the flow (never completed the
// token exchange) leaves a dangling client row forever otherwise. Clients that
// own at least one (non-purged) token are always retained. Returns the number
// of rows removed.
func DeleteOrphanDCRClients(ctx context.Context, db *sql.DB, cutoff time.Time) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM dcr_clients
		 WHERE created_at < ?
		   AND client_id NOT IN (
		       SELECT oauth_client_id FROM mcp_tokens WHERE oauth_client_id IS NOT NULL
		   )`,
		cutoff,
	)
	if err != nil {
		return 0, errors.New("models: delete orphan dcr clients: " + err.Error())
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// HasRedirectURI reports whether the exact redirect URI is registered to the
// client (exact-match per the OAuth spec — no prefix/substring matching).
func (c *DCRClient) HasRedirectURI(uri string) bool {
	for _, u := range c.RedirectURIs {
		if u == uri {
			return true
		}
	}
	return false
}
