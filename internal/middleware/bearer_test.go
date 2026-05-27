package middleware

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/golang-jwt/jwt/v5"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

// --- bearer middleware fixtures ---------------------------------------------

// bearerFixture stages a fresh in-memory DB + a fake JWKS-serving HTTP
// server + a signing keypair so the test can mint synthetic JWTs and
// verify the middleware's reactions end-to-end (no real network, no real
// homelab-mcp dependency).
type bearerFixture struct {
	db       *sql.DB
	signer   *rsa.PrivateKey
	kid      string
	jwks     *httptest.Server
	fetchN   *int32 // atomic counter so tests can assert caching
	issuer   string
	audience string
}

func newBearerFixture(t *testing.T) *bearerFixture {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	signer, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa keygen: %v", err)
	}
	kid := "test-kid-1"
	var fetchN int32

	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&fetchN, 1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwksDocument{
			Keys: []jwk{{
				Kty: "RSA",
				Kid: kid,
				Use: "sig",
				Alg: "RS256",
				N:   base64.RawURLEncoding.EncodeToString(signer.N.Bytes()),
				E:   base64.RawURLEncoding.EncodeToString(bigEndianExp(signer.E)),
			}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return &bearerFixture{
		db:       db,
		signer:   signer,
		kid:      kid,
		jwks:     srv,
		fetchN:   &fetchN,
		issuer:   srv.URL,
		audience: "https://replog.test",
	}
}

// mintToken builds + signs a JWT with the fixture's RSA key. Pass nil
// overrides to use sensible defaults; non-nil values are spliced in.
func (f *bearerFixture) mintToken(t *testing.T, overrides jwt.MapClaims) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":   f.issuer,
		"aud":   f.audience,
		"sub":   "subject-default",
		"email": "coach@example.com",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}
	for k, v := range overrides {
		if v == nil {
			delete(claims, k)
			continue
		}
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = f.kid
	signed, err := tok.SignedString(f.signer)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

// mintTokenWithKid is like mintToken but lets the test inject an
// unknown / rotated kid for negative cases.
func (f *bearerFixture) mintTokenWithKid(t *testing.T, kid string, overrides jwt.MapClaims) string {
	t.Helper()
	claims := jwt.MapClaims{
		"iss":   f.issuer,
		"aud":   f.audience,
		"sub":   "subject-default",
		"email": "coach@example.com",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}
	for k, v := range overrides {
		if v == nil {
			delete(claims, k)
			continue
		}
		claims[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(f.signer)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

func (f *bearerFixture) newAuth(t *testing.T) *BearerAuth {
	t.Helper()
	return NewBearerAuth(f.db, BearerAuthConfig{
		Issuer:   f.issuer,
		Audience: f.audience,
		JWKSURL:  f.jwks.URL + "/oauth/jwks.json",
		CacheTTL: time.Hour,
	})
}

// bigEndianExp converts an int RSA exponent to the JWKS-canonical
// big-endian byte slice (typically [0x01, 0x00, 0x01] for 65537).
func bigEndianExp(e int) []byte {
	if e == 0 {
		return []byte{0}
	}
	var out []byte
	for e > 0 {
		out = append([]byte{byte(e & 0xff)}, out...)
		e >>= 8
	}
	return out
}

func (f *bearerFixture) createUser(t *testing.T, username, email string, mcpEnabled bool) *models.User {
	t.Helper()
	u, err := models.CreateUser(f.db, username, "", "password123", email, true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user %q: %v", username, err)
	}
	if mcpEnabled {
		if err := models.SetUserMCPEnabled(f.db, u.ID, true); err != nil {
			t.Fatalf("enable mcp for %q: %v", username, err)
		}
	}
	return u
}

func runMiddleware(t *testing.T, ba *BearerAuth, req *http.Request) (*httptest.ResponseRecorder, *models.User) {
	t.Helper()
	var got *models.User
	handler := ba.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = UserFromContext(r.Context())
	}))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr, got
}

func reqWithBearer(token string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api-mcp/dashboard", nil)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	return r
}

// --- happy path -------------------------------------------------------------

func TestBearerAuth_AttachesUserOnValidToken(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, got := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "coach1@example.com",
	})))
	if rr.Code != 0 && rr.Code != http.StatusOK {
		t.Fatalf("status = %d (body=%q)", rr.Code, rr.Body.String())
	}
	if got == nil {
		t.Fatal("expected user in context, got nil")
	}
	if got.Username != "coach1" {
		t.Errorf("user.Username = %q, want coach1", got.Username)
	}
	if !got.MCPEnabled {
		t.Error("user.MCPEnabled should be true")
	}
}

func TestBearerAuth_CaseInsensitiveEmail(t *testing.T) {
	// Token carries CoachOne@Example.COM but the user is stored with
	// the column's NOCASE collation. The middleware should lowercase
	// at the boundary AND the DB's NOCASE handles it independently —
	// belt-and-suspenders is the explicit design.
	f := newBearerFixture(t)
	f.createUser(t, "coachup", "Coach.Up@example.com", true)

	ba := f.newAuth(t)
	rr, got := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "COACH.UP@EXAMPLE.COM",
	})))
	if got == nil {
		t.Fatalf("expected user in context, body=%q status=%d", rr.Body.String(), rr.Code)
	}
	if got.Username != "coachup" {
		t.Errorf("user = %q, want coachup", got.Username)
	}
}

// --- rejection paths --------------------------------------------------------

func TestBearerAuth_RejectsMissingBearer(t *testing.T) {
	f := newBearerFixture(t)
	ba := f.newAuth(t)
	rr, got := runMiddleware(t, ba, reqWithBearer(""))
	assertReason(t, rr, http.StatusUnauthorized, "missing-bearer-token")
	if got != nil {
		t.Error("handler should not have run on missing bearer")
	}
}

func TestBearerAuth_RejectsMissingEmailClaim(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": nil, // explicitly drop the claim
	})))
	assertReason(t, rr, http.StatusUnauthorized, "missing-email-claim")
}

func TestBearerAuth_RejectsEmptyEmailClaim(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "   ",
	})))
	assertReason(t, rr, http.StatusUnauthorized, "missing-email-claim")
}

func TestBearerAuth_RejectsExpiredToken(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"iat": time.Now().Add(-2 * time.Hour).Unix(),
		"exp": time.Now().Add(-time.Hour).Unix(),
	})))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

func TestBearerAuth_RejectsWrongAudience(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"aud": "https://some-other-resource.test",
	})))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

func TestBearerAuth_RejectsWrongIssuer(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"iss": "https://impostor.test",
	})))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

func TestBearerAuth_RejectsTokenWithUnknownKid(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintTokenWithKid(t, "rotated-out", nil)))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

func TestBearerAuth_RejectsTokenSignedWithDifferentKey(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	other, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa keygen: %v", err)
	}
	// Mint a claims-valid token but sign with a key the JWKS doesn't know.
	claims := jwt.MapClaims{
		"iss":   f.issuer,
		"aud":   f.audience,
		"sub":   "subj",
		"email": "coach1@example.com",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = f.kid // same kid the JWKS knows, so the lookup succeeds
	bad, err := tok.SignedString(other)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(bad))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

func TestBearerAuth_RejectsUnknownUser(t *testing.T) {
	f := newBearerFixture(t)
	// Don't create the user the token claims to be.

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "ghost@example.com",
	})))
	assertReason(t, rr, http.StatusForbidden, "unknown-user")
}

func TestBearerAuth_RejectsUserWithoutMCPEnabled(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", false) // mcp_enabled=false (default)

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "coach1@example.com",
	})))
	assertReason(t, rr, http.StatusForbidden, "mcp-not-enabled")
}

func TestBearerAuth_RejectsHS256Token(t *testing.T) {
	// Algorithm-confusion guard: the validator pins RS256 so an attacker
	// who flips alg to HS256 and signs with the public key (the classic
	// JWT bug) is rejected at parse time.
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"iss":   f.issuer,
		"aud":   f.audience,
		"sub":   "subj",
		"email": "coach1@example.com",
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(time.Minute).Unix(),
	})
	tok.Header["kid"] = f.kid
	signed, _ := tok.SignedString([]byte("any-shared-secret-pretending"))

	ba := f.newAuth(t)
	rr, _ := runMiddleware(t, ba, reqWithBearer(signed))
	assertReason(t, rr, http.StatusUnauthorized, "invalid-token")
}

// --- caching behavior -------------------------------------------------------

func TestBearerAuth_CachesJWKSAcrossRequests(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	for i := 0; i < 5; i++ {
		rr, got := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
			"email": "coach1@example.com",
		})))
		if got == nil {
			t.Fatalf("iter %d: expected user, status=%d body=%q", i, rr.Code, rr.Body.String())
		}
	}
	if got := atomic.LoadInt32(f.fetchN); got != 1 {
		t.Errorf("jwks fetch count = %d, want 1 (cached across 5 requests)", got)
	}
}

func TestBearerAuth_RefetchesJWKSOnUnknownKid(t *testing.T) {
	f := newBearerFixture(t)
	f.createUser(t, "coach1", "coach1@example.com", true)

	ba := f.newAuth(t)
	// First call seeds the cache (1 fetch).
	if _, got := runMiddleware(t, ba, reqWithBearer(f.mintToken(t, jwt.MapClaims{
		"email": "coach1@example.com",
	}))); got == nil {
		t.Fatal("first call should succeed")
	}
	// Token with a kid not in the JWKS triggers a refetch attempt.
	_, _ = runMiddleware(t, ba, reqWithBearer(f.mintTokenWithKid(t, "rotated", jwt.MapClaims{
		"email": "coach1@example.com",
	})))
	if got := atomic.LoadInt32(f.fetchN); got != 2 {
		t.Errorf("jwks fetch count = %d, want 2 (cache + refetch on unknown kid)", got)
	}
}

// --- helpers ---------------------------------------------------------------

func assertReason(t *testing.T, rr *httptest.ResponseRecorder, wantStatus int, wantReason string) {
	t.Helper()
	if rr.Code != wantStatus {
		t.Errorf("status = %d, want %d (body=%q)", rr.Code, wantStatus, rr.Body.String())
	}
	var body map[string]any
	if err := json.NewDecoder(strings.NewReader(rr.Body.String())).Decode(&body); err != nil {
		t.Fatalf("decode body: %v (body=%q)", err, rr.Body.String())
	}
	if got, _ := body["reason"].(string); got != wantReason {
		t.Errorf("reason = %q, want %q (body=%q)", got, wantReason, rr.Body.String())
	}
}

// Silence unused-import linter when scs is imported only for the broader
// middleware package being in scope; keeps imports tidy.
var _ = scs.New
var _ = context.Background
