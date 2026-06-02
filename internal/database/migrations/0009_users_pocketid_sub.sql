-- +goose Up

-- PocketID subject binding (ADR 019 Phase 1 — HOF-012).
--
-- RepLog becomes a PocketID OIDC relying party for the webui. `pocketid_sub`
-- is the stable PocketID subject (`sub` claim), set on first OIDC login and
-- thereafter the authoritative identity key — it replaces email as the thing
-- we match a returning user on. Email is retained for display/notify only.
--
-- Linking on first login (see models.UpsertUserFromOIDC):
--   1. match by pocketid_sub (the steady-state path);
--   2. else, if the ID token carries email_verified == true, match by email
--      and bind the sub (the one-time cutover for the existing account);
--   3. else JIT-create a passwordless user with a derived unique username.
-- An empty/absent `sub` is rejected at the handler boundary (mirrors the
-- bearer middleware's empty-claim 401), so an empty value never reaches here.
--
-- UNIQUE is enforced via a PARTIAL unique index (not an inline column
-- constraint): SQLite's ALTER TABLE ADD COLUMN cannot add a UNIQUE column.
-- The `WHERE pocketid_sub IS NOT NULL` clause lets existing rows (which have
-- no sub yet) coexist fine until each is bound on its owner's first OIDC
-- login, while still rejecting duplicate non-null subjects.
--
-- Additive only (ADR 002 — forge runs a populated DB; no in-place edit of
-- 0001). Deliberately does NOT drop password_hash or the webauthn_credentials
-- table: the password path is retained as documented break-glass, and the
-- dead passkey table is left dormant for a later explicit cleanup migration
-- once the OIDC RP is proven in production (ADR 019 "retire once OIDC is live").

ALTER TABLE users ADD COLUMN pocketid_sub TEXT;
CREATE UNIQUE INDEX idx_users_pocketid_sub ON users(pocketid_sub) WHERE pocketid_sub IS NOT NULL;

-- +goose Down

-- SQLite >= 3.35 (modernc.org/sqlite ships a newer engine) supports
-- DROP COLUMN. Down is documented but never run in production per ADR 002.
DROP INDEX IF EXISTS idx_users_pocketid_sub;
ALTER TABLE users DROP COLUMN pocketid_sub;
