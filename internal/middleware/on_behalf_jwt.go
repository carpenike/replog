package middleware

import (
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// Signed on-behalf tokens (ADR 025, amended). Homelab-mcp signs a short-lived
// Ed25519 JWT per call: aud=replog, sub and email = the PocketID-verified
// email, act.sub = homelab-mcp, iss, iat, exp, jti. RepLog verifies it with
// a public key, so a leaked RepLog config cannot mint a token.
const (
	// OnBehalfModeEnv selects which on-behalf proofs RequireAuth accepts:
	// "shared" (default: REPLOG_ON_BEHALF_CREDENTIAL only), "signed"
	// (JWT only), or "both" (either, for the switch-over release).
	OnBehalfModeEnv = "REPLOG_ON_BEHALF_MODE"
	// OnBehalfPublicKeyEnv is the Ed25519 public key, PEM (SPKI).
	OnBehalfPublicKeyEnv = "REPLOG_ON_BEHALF_JWT_PUBLIC_KEY"
	// OnBehalfPublicKeyFileEnv names a file holding that PEM instead.
	OnBehalfPublicKeyFileEnv = "REPLOG_ON_BEHALF_JWT_PUBLIC_KEY_FILE"
	// OnBehalfIssuerEnv is the required iss. Default homelab-mcp.
	OnBehalfIssuerEnv = "REPLOG_ON_BEHALF_JWT_ISSUER"
	// OnBehalfActorEnv is the required act.sub. Default homelab-mcp.
	OnBehalfActorEnv = "REPLOG_ON_BEHALF_JWT_ACTOR"

	onBehalfAudience    = "replog"
	onBehalfDefaultName = "homelab-mcp"
	onBehalfLeeway      = 30 * time.Second
	onBehalfMaxLifetime = 5 * time.Minute
)

type onBehalfProofs struct {
	shared bool
	signed bool
}

// onBehalfAccepts reads OnBehalfModeEnv. An unknown value accepts nothing.
func onBehalfAccepts() onBehalfProofs {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(OnBehalfModeEnv))) {
	case "", "shared":
		return onBehalfProofs{shared: true}
	case "signed":
		return onBehalfProofs{signed: true}
	case "both":
		return onBehalfProofs{shared: true, signed: true}
	default:
		return onBehalfProofs{}
	}
}

// LogOnBehalfSignedConfig reports the mode and whether the public key loads.
// It never logs key material.
func LogOnBehalfSignedConfig() {
	mode := strings.TrimSpace(os.Getenv(OnBehalfModeEnv))
	if mode == "" {
		mode = "shared"
	}
	accepts := onBehalfAccepts()
	if !accepts.shared && !accepts.signed {
		log.Printf("On-behalf auth disabled: %s=%q is not shared, signed, or both", OnBehalfModeEnv, mode)
		return
	}
	if accepts.signed {
		if _, err := onBehalfPublicKey(); err != nil {
			log.Printf("On-behalf signed tokens disabled: %v", err)
		} else {
			log.Printf("On-behalf signed tokens enabled (iss=%s, act=%s)", onBehalfIssuer(), onBehalfActor())
		}
	}
	log.Printf("On-behalf mode %s", mode)
}

func onBehalfIssuer() string {
	if v := strings.TrimSpace(os.Getenv(OnBehalfIssuerEnv)); v != "" {
		return v
	}
	return onBehalfDefaultName
}

func onBehalfActor() string {
	if v := strings.TrimSpace(os.Getenv(OnBehalfActorEnv)); v != "" {
		return v
	}
	return onBehalfDefaultName
}

var publicKeyCache struct {
	sync.Mutex
	pem string
	key ed25519.PublicKey
	err error
}

// onBehalfPublicKey parses the configured key. The parse is cached per PEM
// value, so a test can change it.
func onBehalfPublicKey() (ed25519.PublicKey, error) {
	raw := os.Getenv(OnBehalfPublicKeyEnv)
	if raw == "" {
		if path := os.Getenv(OnBehalfPublicKeyFileEnv); path != "" {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil, fmt.Errorf("%s is unreadable", OnBehalfPublicKeyFileEnv)
			}
			raw = string(b)
		}
	}
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("%s is unset", OnBehalfPublicKeyEnv)
	}
	publicKeyCache.Lock()
	defer publicKeyCache.Unlock()
	if publicKeyCache.pem == raw {
		return publicKeyCache.key, publicKeyCache.err
	}
	key, err := parseEd25519PublicKey(raw)
	publicKeyCache.pem, publicKeyCache.key, publicKeyCache.err = raw, key, err
	return key, err
}

func parseEd25519PublicKey(raw string) (ed25519.PublicKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(raw)))
	if block == nil || block.Type != "PUBLIC KEY" {
		return nil, errors.New("on-behalf public key is not a PEM PUBLIC KEY")
	}
	parsed, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, errors.New("on-behalf public key does not parse")
	}
	key, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("on-behalf public key is not Ed25519")
	}
	return key, nil
}

// looksLikeJWT is a cheap shape check so a shared secret is not parsed as one.
func looksLikeJWT(s string) bool {
	return strings.HasPrefix(s, "eyJ") && strings.Count(s, ".") == 2
}

type onBehalfClaims struct {
	Email string `json:"email"`
	Act   struct {
		Sub string `json:"sub"`
	} `json:"act"`
}

// jtiCache remembers accepted jti values until they expire, so a captured
// token cannot be replayed inside its short life. One replica, one process.
var jtiCache = struct {
	sync.Mutex
	seen map[string]time.Time
}{seen: map[string]time.Time{}}

func jtiFirstUse(jti string, exp, now time.Time) bool {
	jtiCache.Lock()
	defer jtiCache.Unlock()
	for k, until := range jtiCache.seen {
		if now.After(until) {
			delete(jtiCache.seen, k)
		}
	}
	if _, ok := jtiCache.seen[jti]; ok {
		return false
	}
	jtiCache.seen[jti] = exp.Add(onBehalfLeeway)
	return true
}

var errOnBehalfToken = errors.New("invalid on-behalf token")

// verifyOnBehalfJWT checks signature, alg, iss, aud, exp/iat/nbf with a
// small leeway, a lifetime cap, act.sub, a jti seen once, and that sub and
// email name the same address. It returns that email.
func verifyOnBehalfJWT(raw string, now time.Time) (string, error) {
	key, err := onBehalfPublicKey()
	if err != nil {
		return "", errOnBehalfToken
	}
	tok, err := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.EdDSA})
	if err != nil || len(tok.Headers) != 1 {
		return "", errOnBehalfToken
	}
	var std jwt.Claims
	var extra onBehalfClaims
	if err := tok.Claims(crypto.PublicKey(key), &std, &extra); err != nil {
		return "", errOnBehalfToken
	}
	if std.Expiry == nil || std.IssuedAt == nil || std.ID == "" {
		return "", errOnBehalfToken
	}
	if err := std.ValidateWithLeeway(jwt.Expected{
		Issuer:      onBehalfIssuer(),
		AnyAudience: jwt.Audience{onBehalfAudience},
		Time:        now,
	}, onBehalfLeeway); err != nil {
		return "", errOnBehalfToken
	}
	if std.IssuedAt.Time().After(now.Add(onBehalfLeeway)) {
		return "", errOnBehalfToken
	}
	if std.Expiry.Time().Sub(std.IssuedAt.Time()) > onBehalfMaxLifetime {
		return "", errOnBehalfToken
	}
	if extra.Act.Sub != onBehalfActor() {
		return "", errOnBehalfToken
	}
	email := strings.TrimSpace(extra.Email)
	if email == "" || !strings.EqualFold(email, strings.TrimSpace(std.Subject)) {
		return "", errOnBehalfToken
	}
	if !jtiFirstUse(std.ID, std.Expiry.Time(), now) {
		return "", errOnBehalfToken
	}
	return email, nil
}
