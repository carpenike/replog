# ADR 013 — OpenAPI Generation via swag Annotations

**Status:** Accepted
**Date:** 2026-05-12

## Context

ADR 011 made the backend a pure JSON REST API consumed by a React SPA and,
eventually, by other clients (mobile, automation, curl). That makes the API
contract a first-class artifact, not an implementation detail.

The first version of the spec was a hand-edited `internal/api/openapi/swagger.yaml`
plus a small Python script that scraped route registrations with regex to
keep the path list in sync. Predictable problems followed:

- Drift. Adding a handler did not require touching the spec, so the spec
  fell behind. By the time the SPA needed an endpoint documented, finding
  what was missing was a manual diff.
- Round-tripping. Multi-line `description:` blocks, `examples`, and
  request-body schemas were tedious to edit by hand.
- No type-level guarantee that the documented request body matched the
  Go struct the handler decoded into.
- The Python sync script was a second toolchain in a Go repo.

The fix needs to put the contract next to the handler, machine-check that
they stay in sync, and stay inside the Go toolchain.

## Decision

Adopt [`swaggo/swag`](https://github.com/swaggo/swag) — a Go AST scanner
that reads structured doc comments on handlers and DTO structs and emits
an OpenAPI / Swagger spec.

### How it is wired up

1. **General info** lives in
   [`internal/api/swag.go`](../../internal/api/swag.go) — `@title`,
   `@version`, `@BasePath /api`, and the `@tag.*` definitions for every
   logical group (Auth, Athletes, Workouts, ...).
2. **Per-handler annotations** sit immediately above each exported handler:

   ```go
   // Me returns the currently authenticated user.
   //
   //  @Summary      Get current user
   //  @Description  Returns the user associated with the current session, ...
   //  @Tags         Auth
   //  @Produce      json
   //  @Success      200  {object}  api.User
   //  @Failure      401  {object}  api.APIError
   //  @Router       /me [get]
   func (h *Handlers) Me(w http.ResponseWriter, r *http.Request) { ... }
   ```

3. **Named request DTOs** live in
   [`internal/api/requests.go`](../../internal/api/requests.go) so swag
   emits referenced schemas, not anonymous inline ones. Each field carries
   `json:"..."` and `example:"..."` tags so the spec includes useful
   examples.
4. **Generation** is one `just openapi` recipe that calls
   `swag init --generalInfo swag.go --dir ./internal/api
   --output ./internal/api/openapi --outputTypes yaml --parseInternal
   --parseDependency`.
5. **The generated spec is committed.** It is `//go:embed`-ed by
   [`internal/api/docs.go`](../../internal/api/docs.go) and served at
   `GET /api/docs/openapi.yaml`. The Swagger UI page is served at
   `GET /api/docs`.
6. **CI gate.** `just openapi-check` regenerates the spec and fails if
   `git diff` reports any change. The `openapi` job in
   `.github/workflows/ci.yaml` runs the same check, so a PR cannot land
   with a stale spec.

### Author workflow

When adding or changing a route or DTO:

1. Edit the handler. Copy the annotation block from a similar handler
   (Login, Me, Dashboard, ListAthletes are good templates).
2. If the request body changed, edit / add the named struct in
   `requests.go`.
3. Run `just openapi`. The diff in `swagger.yaml` is now part of the PR.
4. Run `just qa` locally. CI runs the same gate.

## Rationale

### Why swag

- **Code-first and Go-native.** No second toolchain. `go install` brings
  the generator down once; everything else is a doc comment.
- **AST-based, not regex.** swag reads the actual function signatures and
  struct definitions, so a typo in a route or a missing field on a DTO
  surfaces at generation time, not in the SPA at runtime.
- **Single source of truth lives next to the code.** When a handler
  changes, the doc comment is right there to update — and the CI gate
  forces the issue.
- **Embeds cleanly.** The output is one YAML file, perfect for
  `//go:embed` and same-binary serving.

### Why not alternatives

| Alternative | Why not |
|------------|---------|
| Hand-edited `swagger.yaml` + Python sync script | The thing this ADR replaces. Drift and a second toolchain. |
| `kin-openapi` runtime spec | Generates a spec from registered route metadata, but couples the spec to runtime startup and does not capture request/response *types* without a separate descriptor. swag is more declarative and more idiomatic. |
| `oapi-codegen` spec-first | Inverts the relationship — author the spec, generate handlers. Wrong direction for an existing handler-rich codebase. Worth re-evaluating if we ever want strict request validation generated from the spec. |
| `huma` framework | Would force every handler to adopt huma's signature. Too invasive for an established `http.HandlerFunc` codebase. |

### Why a CI gate, not a generate-on-build hook

A `go generate` invoked at build time would silently regenerate the spec
without anyone reviewing the diff. The committed spec + diff-fail-CI
pattern means every contract change shows up in code review on its own
lines, where it can be discussed and where reviewers can catch
unintentional schema breaks.

### Quirk: swag emits Swagger 2.0

`swaggo/swag` v1 emits Swagger 2.0 (`swagger: "2.0"`), not OpenAPI 3.x.
Swagger UI renders both, and every consumer we have works with 2.0. The
swag v2 line is in active development and adds OpenAPI 3 output; if and
when we need 3.x features (e.g., `oneOf`, richer security schemes, JSON
Schema 2020-12), we re-evaluate. Until then, 2.0 is fine.

## Consequences

### Positive

- 119 operations across 80 paths documented, all generated from
  annotations on the handlers themselves.
- Adding an endpoint without documenting it fails CI. Adding a documented
  endpoint that disagrees with the Go types fails `swag init`.
- The OpenAPI spec is a real artifact: served by the binary, embeddable in
  client SDKs, browseable at `/api/docs`.
- Author cost is one annotation block copied from a neighbor — no new
  files, no new tools at edit time, no out-of-band sync.

### Negative

- Annotation syntax is its own micro-language; typos are caught at
  generation time but not by `gopls`.
- `swag init` needs the `swag` binary in `$PATH`. Captured in
  `just openapi`, the CI workflow installs it explicitly.
- Spec is Swagger 2.0, not OpenAPI 3. Tracked as a future upgrade if we
  ever need 3.x features.

### Neutral

- Generated spec is committed. Adds noise to PR diffs when many handlers
  change at once, but makes contract changes visible in code review,
  which is the point.
