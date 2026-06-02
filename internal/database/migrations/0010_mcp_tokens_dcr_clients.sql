-- +goose Up

-- Native MCP OAuth Authorization Server storage (ADR 019 Phases 2+3 — HOF-013).
--
-- RepLog stops trusting an external homelab-mcp AS (the retired JWKS/RS256
-- path) and becomes its own MCP OAuth 2.1 Authorization Server: it runs
-- Dynamic Client Registration (RFC 7591), federates the human login to
-- PocketID over a server-side PKCE hop, and mints its OWN opaque bearer
-- tokens for the native `/api/mcp` server. Two new tables back that flow.
--
-- mcp_tokens — opaque bearer tokens (the `rlpat_<hex>` access tokens).
--   * token_hash is the SHA-256 (hex) of the full presented secret; the
--     plaintext is returned to the client exactly once at mint time and is
--     NEVER stored. This is the deliberate divergence from login_tokens
--     (short-lived single-use magic links stored plaintext) — a 90-day
--     bearer at rest must be hashed.
--   * user_id is the identity FK. A token cannot exist without a valid
--     user, so validation keys off this FK (no email/"unknown-user" branch);
--     the mcp_enabled gate still applies after the lookup.
--   * NO role column — RepLog has no token-scoped role. Access is the
--     user's existing coach/athlete authz, enforced by the reused handlers.
--   * revoked_at is a soft-delete; expires_at bounds the 90-day lifetime;
--     last_used_at is bumped (best-effort) on each successful validation.
--
-- dcr_clients — DCR-registered OAuth clients (Claude, VS Code, etc.).
--   * client_secret_hash is the SHA-256 (hex) of the issued secret; the
--     plaintext is returned once at registration. Secret comparison at the
--     token endpoint is constant-time (crypto/subtle).
--   * redirect_uris is a JSON array, validated against an allowlist of
--     loopback + known editor/agent origins at register and authorize time.
--
-- New tables, so inline UNIQUE is fine (the SQLite "ALTER TABLE ADD COLUMN
-- cannot add UNIQUE" restriction only applies to altering existing tables).
-- Additive only (ADR 002).

CREATE TABLE mcp_tokens (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash      TEXT NOT NULL UNIQUE,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    oauth_client_id TEXT,
    label           TEXT,
    expires_at      TIMESTAMP NOT NULL,
    revoked_at      TIMESTAMP,
    last_used_at    TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_mcp_tokens_user ON mcp_tokens(user_id);

CREATE TABLE dcr_clients (
    client_id                  TEXT PRIMARY KEY,
    client_secret_hash         TEXT NOT NULL,
    client_name                TEXT NOT NULL DEFAULT '',
    redirect_uris              TEXT NOT NULL DEFAULT '[]',
    token_endpoint_auth_method TEXT NOT NULL DEFAULT 'client_secret_post',
    created_at                 TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down

DROP TABLE IF EXISTS dcr_clients;
DROP INDEX IF EXISTS idx_mcp_tokens_user;
DROP TABLE IF EXISTS mcp_tokens;
