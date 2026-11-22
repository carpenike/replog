package middleware

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/carpenike/replog/internal/models"
)

// OnBehalfCredentialEnv is the dedicated on-behalf secret. It is not
// REPLOG_SERVICE_TOKEN and it is not an rlcst_ bearer. Empty disables
// the on-behalf path.
const OnBehalfCredentialEnv = "REPLOG_ON_BEHALF_CREDENTIAL"

// OnBehalfOfHeader carries the caller's verified email. RequireAuth
// accepts it only together with a matching OnBehalfCredentialEnv value.
// Every other auth path deletes it before the handler runs.
const OnBehalfOfHeader = "X-RepLog-On-Behalf-Of"

// OnBehalfContextKey is set when this request authenticated with the
// on-behalf credential and a mapped user. It is not a service credential:
// the mapped user's session permissions apply.
const OnBehalfContextKey contextKey = "onBehalf"

// onBehalfAuditAction is the audit_log.action for a successful on-behalf
// authentication. Details stay a fixed flag so the credential and the
// header value are never written.
const onBehalfAuditAction = "on_behalf"

// LogOnBehalfConfig reports whether the on-behalf path is active. It logs
// the env name only, never the secret.
func LogOnBehalfConfig() {
	switch onBehalfMode() {
	case onBehalfDisabled:
		log.Printf("On-behalf auth disabled: %s is unset", OnBehalfCredentialEnv)
	case onBehalfReservedPrefix:
		log.Printf("On-behalf auth disabled: %s must not start with %s", OnBehalfCredentialEnv, models.ServiceTokenPrefix)
	default:
		log.Printf("On-behalf auth enabled")
	}
}

type onBehalfModeKind int

const (
	onBehalfDisabled onBehalfModeKind = iota
	onBehalfReservedPrefix
	onBehalfEnabled
)

func onBehalfMode() onBehalfModeKind {
	v := os.Getenv(OnBehalfCredentialEnv)
	if v == "" {
		return onBehalfDisabled
	}
	if strings.HasPrefix(v, models.ServiceTokenPrefix) {
		return onBehalfReservedPrefix
	}
	return onBehalfEnabled
}

// onBehalfCredentialMatches reports whether presented is the configured
// on-behalf secret. Both sides are hashed before the compare so the check
// stays constant-time for secrets of different lengths. An empty setting,
// or a value that uses the rlcst_ prefix, never matches.
func onBehalfCredentialMatches(presented string) bool {
	if onBehalfMode() != onBehalfEnabled || presented == "" {
		return false
	}
	expected := os.Getenv(OnBehalfCredentialEnv)
	sumP := sha256.Sum256([]byte(presented))
	sumE := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(sumP[:], sumE[:]) == 1
}

// authenticateOnBehalf maps the verified email to a user and attaches that
// user with session permissions. A missing or unknown email is 403. It does
// not fall back to a coach-service token, a session cookie, or any default
// user, and it does not write a session.
func authenticateOnBehalf(w http.ResponseWriter, r *http.Request, db *sql.DB, email string, next http.Handler) {
	if email == "" {
		log.Printf("middleware: on-behalf rejected: missing email")
		writeAuthJSON(w, http.StatusForbidden, "on-behalf email is required")
		return
	}
	user, err := models.GetUserByEmail(r.Context(), db, email)
	if errors.Is(err, models.ErrNotFound) {
		log.Printf("middleware: on-behalf rejected: email not mapped")
		writeAuthJSON(w, http.StatusForbidden, "no user for that email")
		return
	}
	if err != nil {
		log.Printf("middleware: on-behalf lookup failed")
		writeAuthJSON(w, http.StatusInternalServerError, "authentication failed")
		return
	}

	if err := models.WriteAuditLog(r.Context(), db, user.ID, sql.NullInt64{}, onBehalfAuditAction, "on_behalf=true"); err != nil {
		log.Printf("middleware: on-behalf audit write failed")
	}
	log.Printf("middleware: on-behalf authenticated user id=%d on_behalf=true", user.ID)

	ctx := contextWithUser(r.Context(), db, user, false)
	ctx = context.WithValue(ctx, OnBehalfContextKey, true)
	next.ServeHTTP(w, r.WithContext(ctx))
}

// IsOnBehalf reports whether this request authenticated via the on-behalf
// credential and a mapped user.
func IsOnBehalf(ctx context.Context) bool {
	v, _ := ctx.Value(OnBehalfContextKey).(bool)
	return v
}
