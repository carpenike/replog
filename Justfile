# RepLog dev commands. Run `just` (or `just -l`) to see everything.
#
# Requires: just, go, node, npm. All are provided by the Nix devShell
# (`nix develop` or `direnv allow`).

set shell := ["bash", "-cu"]

# Default REPLOG_* env so subcommands behave the same as the VS Code Run Server task.
export REPLOG_ADDR              := env_var_or_default("REPLOG_ADDR", ":8080")
export REPLOG_DB_PATH           := env_var_or_default("REPLOG_DB_PATH", "./dev.db")
export REPLOG_AVATAR_DIR        := env_var_or_default("REPLOG_AVATAR_DIR", "./avatars")
export REPLOG_ADMIN_USER        := env_var_or_default("REPLOG_ADMIN_USER", "admin")
export REPLOG_ADMIN_PASS        := env_var_or_default("REPLOG_ADMIN_PASS", "admin")
export REPLOG_ADMIN_EMAIL       := env_var_or_default("REPLOG_ADMIN_EMAIL", "admin@localhost")
export REPLOG_SECRET_KEY        := env_var_or_default("REPLOG_SECRET_KEY", "dev-only-secret-key-not-for-prod!")
# PocketID OIDC (ADR 019) — leave unset for dev; the admin/admin break-glass
# password login works without it. Set all three to exercise the OIDC RP path.
export REPLOG_OIDC_ISSUER        := env_var_or_default("REPLOG_OIDC_ISSUER", "")
export REPLOG_OIDC_CLIENT_ID     := env_var_or_default("REPLOG_OIDC_CLIENT_ID", "")
export REPLOG_OIDC_CLIENT_SECRET := env_var_or_default("REPLOG_OIDC_CLIENT_SECRET", "")

# List available recipes.
default:
    @just --list

# --- Setup ---

# Install frontend dependencies (idempotent; safe to re-run).
install:
    cd web && npm install

# --- Day-to-day dev ---

# Run backend + frontend together. Open http://localhost:5173 (login: admin/admin).
dev:
    #!/usr/bin/env bash
    set -euo pipefail
    if [[ ! -d web/node_modules ]]; then
        echo "→ web/node_modules missing, running 'just install' first..."
        just install
    fi
    echo "→ Starting backend on :8080 and Vite on :5173"
    echo "→ Open http://localhost:5173 (login: admin / admin)"
    trap 'kill 0' EXIT
    go run ./cmd/replog &
    cd web && npm run dev &
    wait

# Run only the Go backend (no frontend hot-reload). SPA served from web/dist if built.
dev-backend:
    go run ./cmd/replog

# Run only the Vite dev server.
dev-frontend:
    cd web && npm run dev

# --- Quality gates ---

# Run go vet + golangci-lint + frontend lint.
lint:
    go vet ./...
    golangci-lint run
    cd web && npm run lint

# Run all Go tests.
test:
    go test -count=1 -race ./...

# Build everything: frontend bundle + Go binary with embedded SPA.
build:
    cd web && npm run build
    go build -o replog ./cmd/replog
    @echo "→ Built ./replog ($(du -h replog | cut -f1))"

# Quality gates: vet + lint + test + build (matches CI).
qa: openapi-check lint test build

# --- OpenAPI ---

# Regenerate internal/api/openapi/swagger.yaml from handler annotations.
# Run after adding/changing routes or DTOs. The generated spec is committed.
openapi:
    swag init \
        --generalInfo swag.go \
        --dir ./internal/api \
        --output ./internal/api/openapi \
        --outputTypes yaml \
        --parseInternal \
        --parseDependency
    @echo "→ Regenerated internal/api/openapi/swagger.yaml"

# Verify the committed spec matches what swag would generate now (used by CI).
openapi-check:
    @just openapi >/dev/null
    @if ! git diff --exit-code -- internal/api/openapi/swagger.yaml; then \
        echo ""; \
        echo "❌ OpenAPI spec is stale. Run 'just openapi' and commit the result."; \
        exit 1; \
    fi
    @echo "→ OpenAPI spec is up to date"

# --- Database ---

# Wipe the dev database (ADR 002: pre-prod we mutate 0001_*.sql in place).
db-reset:
    rm -f dev.db dev.db-wal dev.db-shm
    @echo "→ dev.db wiped. Next 'just dev' will re-bootstrap admin and seed catalog."

# Open a sqlite3 shell on the dev database.
db-shell:
    sqlite3 dev.db

# Back up the dev database safely (uses sqlite .backup, not cp — WAL-safe).
db-backup OUT="dev.db.bak":
    sqlite3 dev.db ".backup '{{OUT}}'"
    @echo "→ Backup written to {{OUT}}"

# --- Release-ish ---

# Build the binary the way Nix does (frontend first, then go build).
build-release:
    cd web && npm ci && npm run build
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o replog ./cmd/replog
    @echo "→ Built ./replog ($(du -h replog | cut -f1))"

# Build via Nix flake.
build-nix:
    nix build

# Print the npmDepsHash that nix/package.nix should pin for the current
# web/package-lock.json. Run after `npm install` changes the lockfile;
# paste the printed sha256 into nix/package.nix.
nix-npm-hash:
    @nix-shell -p prefetch-npm-deps --run "prefetch-npm-deps ./web/package-lock.json"

# Print the vendorHash nix/package.nix should pin for the current go.sum.
# Forces a rebuild against `lib.fakeHash` and extracts the "got:" line
# from the resulting hash-mismatch error. Run after `go mod tidy`.
nix-vendor-hash:
    @nix build .#default --no-link 2>&1 \
        | awk '/got:/ {print $2; exit}' \
        || true

# --- Maintenance ---

# Update Go dependencies.
go-tidy:
    go mod tidy

# Audit npm packages for vulnerabilities.
npm-audit:
    cd web && npm audit

# Run the same vulnerability scan CI runs (govulncheck on Go,
# npm audit on the frontend). Installs govulncheck on first run.
vulncheck:
    #!/usr/bin/env bash
    set -euo pipefail
    GOBIN="$(go env GOPATH)/bin"
    if [[ ! -x "$GOBIN/govulncheck" ]]; then
        echo "→ Installing govulncheck..."
        go install golang.org/x/vuln/cmd/govulncheck@latest
    fi
    "$GOBIN/govulncheck" ./...
    cd web && npm audit --omit=dev --audit-level=moderate
    npm audit --audit-level=high
    echo "→ Vulnerability scan clean"
