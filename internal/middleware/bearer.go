package middleware

import (
	"context"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/carpenike/replog/internal/models"
)

// BearerAuthConfig parameterises the MCP bearer middleware (HOF-004).
//
// The middleware verifies short-TTL RS256 JWTs minted by the homelab-mcp
// OAuth Authorization Server (mcp.holthome.net) against the AS's published
// JWKS. RepLog never holds the private signing key — verification is
// JWKS-only, fetched over HTTPS (or loopback when AS + RS are co-resident
// on the same forge host), cached in process, and refetched lazily on
// `kid` cache miss or TTL expiry.
//
// Identity flow: the JWT IS the caller identity. The middleware reads the
// `email` claim, refuses empty/absent values (401), resolves to a *models.User
// via models.GetUserByEmail, refuses if `user.MCPEnabled == false` (403),
// then attaches user + prefs to context under the SAME keys the scs-cookie
// RequireAuth middleware uses (UserContextKey / PrefsContextKey). Downstream
// handlers that call UserFromContext / CanAccessAthlete / CanManageAthlete
// work without modification.
type BearerAuthConfig struct {
	// Issuer is the expected `iss` claim. e.g. "https://mcp.holthome.net".
	Issuer string

	// Audience is the expected `aud` claim. e.g. "https://replog.holthome.net".
	// The homelab-mcp tool-hop mint helper sets aud=<this resource's URL>
	// so a token addressed to mcp.holthome.net's own tools cannot be
	// replayed against replog.
	Audience string

	// JWKSURL is the absolute URL of the AS's published JWKS document.
	// Typically "<Issuer>/oauth/jwks.json".
	JWKSURL string

	// CacheTTL bounds how long a fetched JWKS stays valid before a refetch
	// is forced on the next request. Default 1h. A `kid` cache miss
	// triggers an immediate refetch regardless of TTL.
	CacheTTL time.Duration

	// HTTPClient is used to fetch the JWKS. Defaults to a client with a
	// 10s timeout if nil.
	HTTPClient *http.Client

	// Clock is the time source. Used by tests to control TTL expiry; nil
	// uses time.Now.
	Clock func() time.Time
}

// BearerAuth is the constructed middleware.
//
// Use:
//
//	ba := middleware.NewBearerAuth(db, BearerAuthConfig{
//	    Issuer:   "https://mcp.holthome.net",
//	    Audience: "https://replog.holthome.net",
//	    JWKSURL:  "https://mcp.holthome.net/oauth/jwks.json",
//	})
//	r.Use(ba.Middleware)
//
// Or wrap a single handler:
//
//	r.Get("/api-mcp/dashboard", ba.Middleware(h.Dashboard).ServeHTTP)
type BearerAuth struct {
	db     *sql.DB
	cfg    BearerAuthConfig
	keys   map[string]*rsa.PublicKey
	keysAt time.Time
	mu     sync.RWMutex
}

// NewBearerAuth constructs a BearerAuth with sensible defaults filled in.
// Panics if cfg.Issuer, cfg.Audience, or cfg.JWKSURL is empty — those are
// load-bearing and a missing value in production would silently weaken
// the verification chain.
func NewBearerAuth(db *sql.DB, cfg BearerAuthConfig) *BearerAuth {
	if cfg.Issuer == "" {
		panic("middleware: BearerAuth requires Issuer")
	}
	if cfg.Audience == "" {
		panic("middleware: BearerAuth requires Audience")
	}
	if cfg.JWKSURL == "" {
		panic("middleware: BearerAuth requires JWKSURL")
	}
	if cfg.CacheTTL <= 0 {
		cfg.CacheTTL = time.Hour
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	return &BearerAuth{
		db:   db,
		cfg:  cfg,
		keys: map[string]*rsa.PublicKey{},
	}
}

// Middleware returns the ASGI-shaped HTTP middleware.
func (b *BearerAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearer(r)
		if token == "" {
			writeBearerError(w, http.StatusUnauthorized, "missing-bearer-token",
				`Bearer realm="replog", error="invalid_request"`)
			return
		}

		claims, err := b.verify(r.Context(), token)
		if err != nil {
			log.Printf("middleware: bearer rejected: %v", err)
			writeBearerError(w, http.StatusUnauthorized, "invalid-token",
				`Bearer realm="replog", error="invalid_token"`)
			return
		}

		email := strings.ToLower(strings.TrimSpace(claimString(claims, "email")))
		if email == "" {
			writeBearerError(w, http.StatusUnauthorized, "missing-email-claim",
				`Bearer realm="replog", error="invalid_token"`)
			return
		}

		user, err := models.GetUserByEmail(b.db, email)
		if errors.Is(err, models.ErrNotFound) {
			// Treat unknown-but-validly-signed identity as forbidden, not
			// 401: the token IS valid and the caller is authenticated as
			// SOMEONE — they just have no replog binding. 403 makes the
			// "your homelab account exists but there's no replog account
			// for that email" failure mode visible without retrying the
			// OAuth dance.
			writeBearerError(w, http.StatusForbidden, "unknown-user", "")
			return
		}
		if err != nil {
			log.Printf("middleware: bearer email lookup: %v", err)
			writeBearerError(w, http.StatusInternalServerError, "lookup-failed", "")
			return
		}

		if !user.MCPEnabled {
			// The toggle is admin-controlled and intentionally surfaced
			// in the error so the operator can act on it. We never reveal
			// existence/non-existence above (unknown-user vs not-enabled
			// are different statuses), but at this point both branches
			// have already been distinguished by reaching this line.
			writeBearerError(w, http.StatusForbidden, "mcp-not-enabled", "")
			return
		}

		ctx := context.WithValue(r.Context(), UserContextKey, user)

		// Same defaults-on-error semantics as RequireAuth.
		prefs, err := models.GetUserPreferences(b.db, user.ID)
		if err != nil {
			log.Printf("middleware: bearer prefs lookup for user %d: %v", user.ID, err)
			prefs = &models.UserPreferences{
				UserID:     user.ID,
				WeightUnit: models.DefaultWeightUnit,
				Timezone:   models.DefaultTimezone,
				DateFormat: models.DefaultDateFormat,
			}
		}
		ctx = context.WithValue(ctx, PrefsContextKey, prefs)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// verify parses + validates the token against the cached JWKS. On `kid`
// miss it refetches the JWKS once; if the kid still doesn't appear the
// token is rejected. exp/iat/iss/aud are validated by jwt.WithValidMethods
// / RegisteredClaims + the explicit issuer/audience checks below.
func (b *BearerAuth) verify(ctx context.Context, raw string) (jwt.MapClaims, error) {
	keyFn := func(tok *jwt.Token) (any, error) {
		if tok.Method.Alg() != "RS256" {
			return nil, fmt.Errorf("unexpected signing alg %q", tok.Method.Alg())
		}
		kid, _ := tok.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token missing kid")
		}
		key, err := b.resolveKey(ctx, kid)
		if err != nil {
			return nil, fmt.Errorf("resolve kid %q: %w", kid, err)
		}
		return key, nil
	}

	claims := jwt.MapClaims{}
	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(b.cfg.Issuer),
		jwt.WithAudience(b.cfg.Audience),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
	)
	if _, err := parser.ParseWithClaims(raw, claims, keyFn); err != nil {
		return nil, err
	}

	// jwt/v5 enforces `aud` containment, but its sub-claim semantics treat
	// `sub` as optional. We require `sub` explicitly so a future token
	// shape change cannot quietly disable identity tracking.
	if claimString(claims, "sub") == "" {
		return nil, errors.New("token missing sub")
	}
	return claims, nil
}

// resolveKey returns the RSA public key for the given kid, refetching the
// JWKS on cache miss or TTL expiry. Concurrent callers wait for a single
// in-flight refetch via the mutex.
func (b *BearerAuth) resolveKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	b.mu.RLock()
	key, ok := b.keys[kid]
	stale := b.cfg.Clock().Sub(b.keysAt) > b.cfg.CacheTTL
	b.mu.RUnlock()
	if ok && !stale {
		return key, nil
	}

	if err := b.refresh(ctx); err != nil {
		// If we had a stale-but-present key, prefer using it over an
		// outright failure — preserves availability across a brief AS
		// outage. The token's own exp claim still bounds replay risk.
		if key != nil {
			log.Printf("middleware: jwks refresh failed, using stale key: %v", err)
			return key, nil
		}
		return nil, err
	}

	b.mu.RLock()
	key, ok = b.keys[kid]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("kid %q not in jwks", kid)
	}
	return key, nil
}

// refresh fetches the JWKS, parses every RSA key, and replaces the cache.
// Non-RSA / malformed entries are skipped (logged) rather than failing
// the whole refresh — a single malformed key shouldn't take down auth.
func (b *BearerAuth) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.cfg.JWKSURL, nil)
	if err != nil {
		return fmt.Errorf("build jwks request: %w", err)
	}
	resp, err := b.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("jwks status %d: %s", resp.StatusCode, body)
	}

	var doc jwksDocument
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&doc); err != nil {
		return fmt.Errorf("decode jwks: %w", err)
	}

	next := map[string]*rsa.PublicKey{}
	for _, k := range doc.Keys {
		if k.Kty != "RSA" {
			continue
		}
		if k.Kid == "" {
			log.Printf("middleware: jwks key with empty kid, skipping")
			continue
		}
		pub, err := jwkToRSA(k)
		if err != nil {
			log.Printf("middleware: jwks kid %q parse failed: %v", k.Kid, err)
			continue
		}
		next[k.Kid] = pub
	}
	if len(next) == 0 {
		return errors.New("jwks contained no usable RSA keys")
	}

	b.mu.Lock()
	b.keys = next
	b.keysAt = b.cfg.Clock()
	b.mu.Unlock()
	return nil
}

// --- helpers ---

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func jwkToRSA(k jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("decode n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("decode e: %w", err)
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, errors.New("empty modulus or exponent")
	}
	n := new(big.Int).SetBytes(nBytes)
	// Exponent is a big-endian unsigned int; jwks practically always uses
	// 65537 (3 bytes "AQAB"), but parse generally for future-proofing.
	e := 0
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	if e <= 0 {
		return nil, errors.New("non-positive exponent")
	}
	return &rsa.PublicKey{N: n, E: e}, nil
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}

func claimString(claims jwt.MapClaims, key string) string {
	v, _ := claims[key].(string)
	return v
}

// writeBearerError emits a JSON 4xx response with a stable `reason` slug
// the operator (and any forge-side monitoring) can pattern-match on. The
// optional wwwAuthenticate header is set per RFC 6750 on 401 responses.
func writeBearerError(w http.ResponseWriter, status int, reason, wwwAuthenticate string) {
	w.Header().Set("Content-Type", "application/json")
	if wwwAuthenticate != "" {
		w.Header().Set("WWW-Authenticate", wwwAuthenticate)
	}
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":  "unauthorized",
		"reason": reason,
		"code":   status,
	})
}
