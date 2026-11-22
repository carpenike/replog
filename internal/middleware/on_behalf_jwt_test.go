package middleware

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/carpenike/replog/internal/models"
)

type testSigner struct {
	priv ed25519.PrivateKey
	pem  string
}

func newTestSigner(t *testing.T) testSigner {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return testSigner{priv: priv, pem: string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))}
}

type tokenOpts struct {
	iss, aud, sub, email, act, jti string
	iat, exp                       time.Time
	alg                            jose.SignatureAlgorithm
	key                            any
}

var jtiCounter int

func (s testSigner) token(t *testing.T, edit func(*tokenOpts)) string {
	t.Helper()
	now := time.Now()
	jtiCounter++
	o := tokenOpts{
		iss: "homelab-mcp", aud: "replog", sub: "coach@example.com", email: "coach@example.com", act: "homelab-mcp",
		jti: "jti-" + t.Name() + "-" + strings.Repeat("x", jtiCounter), iat: now, exp: now.Add(60 * time.Second),
		alg: jose.EdDSA, key: s.priv,
	}
	if edit != nil {
		edit(&o)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: o.alg, Key: o.key}, (&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}
	std := jwt.Claims{Issuer: o.iss, Subject: o.sub, Audience: jwt.Audience{o.aud}, IssuedAt: jwt.NewNumericDate(o.iat), Expiry: jwt.NewNumericDate(o.exp), ID: o.jti}
	extra := map[string]any{"email": o.email, "act": map[string]string{"sub": o.act}}
	raw, err := jwt.Signed(signer).Claims(std).Claims(extra).Serialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestOnBehalfAcceptsMode(t *testing.T) {
	for mode, want := range map[string]onBehalfProofs{
		"": {shared: true}, "shared": {shared: true}, "signed": {signed: true}, "BOTH": {shared: true, signed: true}, "jwt": {},
	} {
		t.Setenv(OnBehalfModeEnv, mode)
		if got := onBehalfAccepts(); got != want {
			t.Errorf("mode %q = %+v, want %+v", mode, got, want)
		}
	}
}

func TestVerifyOnBehalfJWT(t *testing.T) {
	s := newTestSigner(t)
	t.Setenv(OnBehalfPublicKeyEnv, s.pem)
	now := time.Now()

	email, err := verifyOnBehalfJWT(s.token(t, nil), now)
	if err != nil || email != "coach@example.com" {
		t.Fatalf("valid token: %q %v", email, err)
	}

	other := newTestSigner(t)
	_, hsKey, _ := ed25519.GenerateKey(rand.Reader)
	cases := map[string]string{
		"other key":      other.token(t, nil),
		"HS256":          s.token(t, func(o *tokenOpts) { o.alg, o.key = jose.HS256, []byte(hsKey[:32]) }),
		"expired":        s.token(t, func(o *tokenOpts) { o.iat, o.exp = now.Add(-3*time.Minute), now.Add(-time.Minute) }),
		"iat in future":  s.token(t, func(o *tokenOpts) { o.iat, o.exp = now.Add(2*time.Minute), now.Add(3*time.Minute) }),
		"too long":       s.token(t, func(o *tokenOpts) { o.exp = now.Add(time.Hour) }),
		"wrong aud":      s.token(t, func(o *tokenOpts) { o.aud = "homelab-mcp" }),
		"wrong iss":      s.token(t, func(o *tokenOpts) { o.iss = "someone" }),
		"wrong act":      s.token(t, func(o *tokenOpts) { o.act = "hermes" }),
		"no jti":         s.token(t, func(o *tokenOpts) { o.jti = "" }),
		"sub not email":  s.token(t, func(o *tokenOpts) { o.sub = "other@example.com" }),
		"no email":       s.token(t, func(o *tokenOpts) { o.email, o.sub = "", "" }),
		"garbage":        "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ4In0.",
		"tampered claim": tamper(s.token(t, nil)),
	}
	for name, tok := range cases {
		if _, err := verifyOnBehalfJWT(tok, now); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}

	// A jti is accepted once.
	once := s.token(t, nil)
	if _, err := verifyOnBehalfJWT(once, now); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if _, err := verifyOnBehalfJWT(once, now); err == nil {
		t.Fatal("replayed jti accepted")
	}

	// Within the leeway is fine.
	if _, err := verifyOnBehalfJWT(s.token(t, func(o *tokenOpts) { o.iat, o.exp = now.Add(-80*time.Second), now.Add(-10*time.Second) }), now); err != nil {
		t.Fatalf("leeway: %v", err)
	}

	// No key, or a key that is not Ed25519, accepts nothing.
	t.Setenv(OnBehalfPublicKeyEnv, "")
	if _, err := verifyOnBehalfJWT(s.token(t, nil), now); err == nil {
		t.Fatal("accepted with no key")
	}
	t.Setenv(OnBehalfPublicKeyEnv, "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE\n-----END PUBLIC KEY-----\n")
	if _, err := verifyOnBehalfJWT(s.token(t, nil), now); err == nil {
		t.Fatal("accepted with a bad key")
	}
}

func tamper(tok string) string {
	parts := strings.Split(tok, ".")
	// Flip the payload to another email with the original signature.
	parts[1] = strings.TrimRight(parts[1], "=") + "A"
	return strings.Join(parts, ".")
}

func TestRequireAuth_SignedOnBehalf(t *testing.T) {
	db := testDB(t)
	sm := testSessionManager()
	coach, err := models.CreateUser(context.Background(), db, "coach", "Coach", "password123", "Coach@Example.com", true, false, sql.NullInt64{})
	if err != nil {
		t.Fatal(err)
	}
	s := newTestSigner(t)
	t.Setenv(OnBehalfPublicKeyEnv, s.pem)
	t.Setenv(OnBehalfCredentialEnv, testOnBehalfCredential)

	var gotID int64
	var onBehalf bool
	handler := RequireAuth(sm, db, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = UserFromContext(r.Context()).ID
		onBehalf = IsOnBehalf(r.Context())
		w.WriteHeader(http.StatusOK)
	}))
	call := func(path, bearer, header string) *httptest.ResponseRecorder {
		gotID, onBehalf = 0, false
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+bearer)
		if header != "" {
			req.Header.Set(OnBehalfOfHeader, header)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	// signed: the JWT works, the shared credential does not.
	t.Setenv(OnBehalfModeEnv, "signed")
	logs := captureLog(t)
	tok := s.token(t, nil)
	if rr := call("/api/me", tok, ""); rr.Code != http.StatusOK || gotID != coach.ID || !onBehalf {
		t.Fatalf("signed: %d user=%d onBehalf=%v", rr.Code, gotID, onBehalf)
	}
	assertLogOmitsSecrets(t, logs, tok, "coach@example.com")
	if rr := call("/api/me", testOnBehalfCredential, "coach@example.com"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("shared in signed mode = %d", rr.Code)
	}
	// The scope still applies to a signed request.
	if rr := call("/api/users", s.token(t, nil), ""); rr.Code != http.StatusForbidden {
		t.Fatalf("admin route = %d", rr.Code)
	}
	// A header naming someone else is refused, and the same address passes.
	if rr := call("/api/me", s.token(t, nil), "other@example.com"); rr.Code != http.StatusForbidden {
		t.Fatalf("mismatched header = %d", rr.Code)
	}
	if rr := call("/api/me", s.token(t, nil), "COACH@example.com"); rr.Code != http.StatusOK {
		t.Fatalf("matching header = %d", rr.Code)
	}
	if rr := call("/api/me", s.token(t, func(o *tokenOpts) { o.email, o.sub = "nobody@example.com", "nobody@example.com" }), ""); rr.Code != http.StatusForbidden {
		t.Fatalf("unmapped email = %d", rr.Code)
	}
	if rr := call("/api/me", tok, ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("replay = %d", rr.Code)
	}

	// shared (the default): a JWT is not accepted.
	t.Setenv(OnBehalfModeEnv, "")
	if rr := call("/api/me", s.token(t, nil), ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("JWT in shared mode = %d", rr.Code)
	}
	if rr := call("/api/me", testOnBehalfCredential, "coach@example.com"); rr.Code != http.StatusOK {
		t.Fatalf("shared in shared mode = %d", rr.Code)
	}

	// both: either proof.
	t.Setenv(OnBehalfModeEnv, "both")
	if rr := call("/api/me", s.token(t, nil), ""); rr.Code != http.StatusOK {
		t.Fatalf("JWT in both = %d", rr.Code)
	}
	if rr := call("/api/me", testOnBehalfCredential, "coach@example.com"); rr.Code != http.StatusOK {
		t.Fatalf("shared in both = %d", rr.Code)
	}

	entries, err := models.ListAuditLog(context.Background(), db, 50)
	if err != nil {
		t.Fatal(err)
	}
	proofs := map[string]int{}
	for _, e := range entries {
		proofs[e.Details.String]++
	}
	if proofs["on_behalf=true proof=signed"] == 0 || proofs["on_behalf=true proof=shared"] == 0 {
		t.Fatalf("audit proofs = %v", proofs)
	}
}
