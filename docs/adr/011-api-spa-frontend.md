# ADR 011 — REST API + SPA Frontend

**Status:** Accepted
**Date:** 2026-03-07

## Context

RepLog's current architecture renders all UI server-side using Go's `html/template` with htmx for interactivity. While this is operationally simple, it has limitations:

- No API contract — handlers return HTML fragments, not structured data
- No path to mobile clients without a full rethink
- UI interactions are limited to htmx's partial-swap model
- No ability for external tools (n8n, curl, automations) to consume data

The Go backend is well-structured with thin handlers, clean model separation, and comprehensive test coverage (591 tests). The goal is to preserve this foundation while adding a modern frontend layer.

## Decision

### Backend: Add JSON REST API alongside existing SSR

1. **Content negotiation** — handlers serve JSON when `Accept: application/json` is present, HTML otherwise. This enables incremental migration without breaking the existing UI.
2. **API DTO layer** (`internal/api/`) — JSON-serializable response types that convert `sql.Null*` fields to pointer types. Separates serialization concerns from database scanning.
3. **CORS middleware** — chi CORS middleware for local development (Vite dev server on different port). Production uses same-origin via `//go:embed`.
4. **Session auth preserved** — cookie-based sessions (scs) remain the auth mechanism. No JWTs.

### Frontend: React + shadcn/ui + Tailwind CSS via Vite

1. **React + TypeScript** — strongest AI-assisted development support, largest ecosystem
2. **shadcn/ui** — components copied into codebase (not black-box dependency), highly customizable
3. **Tailwind CSS** — utility-first, pairs naturally with shadcn/ui
4. **Vite** — fast builds, excellent developer experience

### Deployment: Single binary preserved via `//go:embed`

The compiled frontend (`web/dist/`) is embedded into the Go binary at build time. The Nix flake runs the frontend build first, then `go build`. Same single binary, same NixOS deployment — no operational changes.

```go
//go:embed all:web/dist
var frontendFS embed.FS
```

### Repository structure

Monorepo — `web/` directory added alongside existing `cmd/` and `internal/`:

```
replog/
├── cmd/replog/          # Go entrypoint (unchanged)
├── internal/            # Go backend (+ internal/api/ for DTOs)
├── web/                 # React/Vite frontend (new)
│   ├── src/
│   ├── dist/            # Built output, embedded into Go binary
│   └── package.json
├── docs/
└── flake.nix            # Builds web/ then api/
```

## Migration Path

1. Add JSON responses alongside existing HTML handlers (content negotiation)
2. Build React frontend consuming JSON responses
3. Remove HTML template rendering once frontend is stable
4. Add mobile client as separate directory consuming same API

## Consequences

### Positive

- Clean REST API consumable by browser, mobile, curl, automation tools
- Modern component-based UI with richer interactions
- Shared TypeScript types enable future mobile clients
- Single binary deployment story preserved

### Negative

- Frontend build step required (Node.js in CI/Nix/Docker)
- `sql.Null*` → DTO conversion layer adds code
- 591 handler tests need updating (HTML assertions → JSON assertions)
- 3,260 lines of app.css design work must be rebuilt in Tailwind/shadcn
- node_modules dependency tree added to dev environment

### Neutral

- Existing Go backend architecture (handlers, models, middleware) unchanged
- SQLite, chi router, session auth all preserved
- Monorepo structure maintained
