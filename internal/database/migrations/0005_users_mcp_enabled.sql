-- +goose Up

-- Per-user MCP-access gate (HOF-004 — RepLog MCP layer, Option α′).
--
-- Identity for MCP requests flows as a short-TTL RS256 JWT minted by the
-- homelab-mcp OAuth AS (mcp.holthome.net) and verified by replog's new
-- bearer middleware against the AS's published JWKS. The JWT IS the
-- identity — no per-user bearer token is stored anywhere in replog.
--
-- This column is the operator's gate on top of that: even with a valid
-- JWT and a real email->user resolution, the request is refused with
-- 403 mcp-not-enabled if `mcp_enabled = 0`. Default-deny so existing
-- users do not gain MCP access by accident on rollout; the admin UI
-- (PUT /api/users/{userID}/mcp) flips it per user.
--
-- Graduates additively to a `mcp_grants` table later (granted_at /
-- granted_by / last_used_at) when the triggers in HOF-004's
-- [graduation-trigger] fire — at that point this column becomes
-- a denormalized view of the latest grant.
--
-- Additive only (ADR 002 — the binary is already deployed to forge with
-- a populated DB; no in-place edit of 0001).

ALTER TABLE users ADD COLUMN mcp_enabled INTEGER NOT NULL DEFAULT 0
    CHECK(mcp_enabled IN (0, 1));

-- +goose Down

-- SQLite >= 3.35 (modernc.org/sqlite ships a newer engine) supports
-- DROP COLUMN. Down is documented but never run in production per ADR 002.
ALTER TABLE users DROP COLUMN mcp_enabled;
