package main

import (
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/carpenike/replog/internal/api"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// mcpTestEnv stages a router that mounts ONLY the /api-mcp/* sub-router
// against a fresh in-memory DB and a synthetic JWKS server, so we can
// exercise the curated route group end-to-end with real bearer tokens.
type mcpTestEnv struct {
	db       *sql.DB
	signer   *rsa.PrivateKey
	kid      string
	jwks     *httptest.Server
	router   chi.Router
	issuer   string
	audience string
}

func newMCPTestEnv(t *testing.T) *mcpTestEnv {
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
	kid := "integration-kid"
	mux := http.NewServeMux()
	mux.HandleFunc("/oauth/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   base64.RawURLEncoding.EncodeToString(signer.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(intToBigEndian(signer.E)),
			}},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	h := &api.Handlers{DB: db, Sessions: sm, AvatarDir: t.TempDir()}

	issuer := srv.URL
	audience := "https://replog.test"
	bearer := middleware.NewBearerAuth(db, middleware.BearerAuthConfig{
		Issuer:   issuer,
		Audience: audience,
		JWKSURL:  srv.URL + "/oauth/jwks.json",
	})
	limiter := middleware.NewRateLimiter(100, time.Minute)

	r := chi.NewRouter()
	mountMCPRoutes(r, bearer, limiter, h)

	return &mcpTestEnv{
		db: db, signer: signer, kid: kid, jwks: srv, router: r,
		issuer: issuer, audience: audience,
	}
}

func intToBigEndian(e int) []byte {
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

func (e *mcpTestEnv) mintTokenForEmail(t *testing.T, email string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   e.issuer,
		"aud":   e.audience,
		"sub":   "subj-" + email,
		"email": email,
		"iat":   time.Now().Add(-time.Minute).Unix(),
		"exp":   time.Now().Add(5 * time.Minute).Unix(),
	})
	tok.Header["kid"] = e.kid
	signed, err := tok.SignedString(e.signer)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return signed
}

func (e *mcpTestEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader *strings.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		bodyReader = strings.NewReader(string(buf))
	}
	var req *http.Request
	if bodyReader != nil {
		req = httptest.NewRequest(method, path, bodyReader)
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

// createCoach inserts a coach user, optionally with mcp_enabled set.
func (e *mcpTestEnv) createCoach(t *testing.T, username, email string, mcpEnabled bool) *models.User {
	t.Helper()
	u, err := models.CreateUser(e.db, username, "", "password123", email, true, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create %q: %v", username, err)
	}
	if mcpEnabled {
		if err := models.SetUserMCPEnabled(e.db, u.ID, true); err != nil {
			t.Fatalf("enable mcp for %q: %v", username, err)
		}
	}
	return u
}

func (e *mcpTestEnv) createAthleteFor(t *testing.T, name string, coachID int64) *models.Athlete {
	t.Helper()
	a, err := models.CreateAthlete(e.db, name, "", "", "", "", "", "",
		sql.NullInt64{Int64: coachID, Valid: true}, false)
	if err != nil {
		t.Fatalf("create athlete %q: %v", name, err)
	}
	return a
}

// --- end-to-end tests -------------------------------------------------------

// TestAPIMCP_CanManageAthlete_ParityWithWebui asserts that a bearer-auth
// request inherits the SAME ownership checks as the webui — a coach
// cannot reach an athlete assigned to a different coach. This is the
// load-bearing parity claim of HOF-004 (success-criterion #1 + #2).
func TestAPIMCP_CanManageAthlete_ParityWithWebui(t *testing.T) {
	env := newMCPTestEnv(t)

	coachA := env.createCoach(t, "coachA", "a@example.com", true)
	coachB := env.createCoach(t, "coachB", "b@example.com", true)
	athleteOfA := env.createAthleteFor(t, "Alpha", coachA.ID)
	athleteOfB := env.createAthleteFor(t, "Bravo", coachB.ID)

	tokenA := env.mintTokenForEmail(t, "a@example.com")

	t.Run("coachA can read their own athlete", func(t *testing.T) {
		rr := env.do(t, http.MethodGet, fmt.Sprintf("/api-mcp/athletes/%d", athleteOfA.ID), tokenA, nil)
		if rr.Code != http.StatusOK {
			t.Errorf("status = %d, want 200 (body=%q)", rr.Code, rr.Body.String())
		}
	})

	t.Run("coachA cannot read coachB's athlete", func(t *testing.T) {
		rr := env.do(t, http.MethodGet, fmt.Sprintf("/api-mcp/athletes/%d", athleteOfB.ID), tokenA, nil)
		// Per replog's existing handler: returns 403 access denied for
		// athletes not visible to this user (CanAccessAthlete returns false).
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 403 or 404 (body=%q)", rr.Code, rr.Body.String())
		}
	})

	t.Run("coachA can log a set for their own athlete", func(t *testing.T) {
		// First create a workout via the MCP endpoint, then log a set.
		today := time.Now().UTC().Format("2006-01-02")
		rr := env.do(t, http.MethodPost, fmt.Sprintf("/api-mcp/athletes/%d/workouts", athleteOfA.ID), tokenA,
			map[string]any{"date": today})
		if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
			t.Fatalf("create workout: status = %d body=%q", rr.Code, rr.Body.String())
		}
		var workout map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &workout); err != nil {
			t.Fatalf("decode workout: %v", err)
		}
		workoutID := int64(workout["id"].(float64))

		// Create an exercise the set can reference.
		ex, err := models.CreateExercise(env.db, "TestSquat", "", "", "", 0, false)
		if err != nil {
			t.Fatalf("create exercise: %v", err)
		}

		// Log a set.
		rr = env.do(t, http.MethodPost,
			fmt.Sprintf("/api-mcp/athletes/%d/workouts/%d/sets", athleteOfA.ID, workoutID),
			tokenA,
			map[string]any{
				"exercise_id": ex.ID,
				"set_number":  1,
				"reps":        5,
				"weight":      135.0,
			})
		if rr.Code != http.StatusCreated && rr.Code != http.StatusOK {
			t.Fatalf("add set: status = %d body=%q", rr.Code, rr.Body.String())
		}

		// Confirm audit attribution: set's parent workout was created
		// under coachA's identity. The workout-creation path uses the
		// authenticated user's CanAccessAthlete check; here we verify by
		// reading back the workout.
		rr = env.do(t, http.MethodGet,
			fmt.Sprintf("/api-mcp/athletes/%d/workouts/%d", athleteOfA.ID, workoutID),
			tokenA, nil)
		if rr.Code != http.StatusOK {
			t.Fatalf("read back workout: status = %d body=%q", rr.Code, rr.Body.String())
		}
	})

	t.Run("coachA cannot log a set for coachB's athlete", func(t *testing.T) {
		today := time.Now().UTC().Format("2006-01-02")
		rr := env.do(t, http.MethodPost, fmt.Sprintf("/api-mcp/athletes/%d/workouts", athleteOfB.ID), tokenA,
			map[string]any{"date": today})
		if rr.Code != http.StatusForbidden && rr.Code != http.StatusNotFound {
			t.Errorf("status = %d, want 403 or 404 (body=%q)", rr.Code, rr.Body.String())
		}
	})
}

// TestAPIMCP_RejectsUserWithoutMCPEnabled is the success-criterion #3
// end-to-end test: a valid JWT for a real user with mcp_enabled=0 is
// rejected at the middleware boundary with 403 mcp-not-enabled, BEFORE
// any handler runs (so the user's row is loaded but no athlete data
// leaks).
func TestAPIMCP_RejectsUserWithoutMCPEnabled(t *testing.T) {
	env := newMCPTestEnv(t)
	env.createCoach(t, "coachgated", "gated@example.com", false /* mcp_enabled */)

	token := env.mintTokenForEmail(t, "gated@example.com")
	rr := env.do(t, http.MethodGet, "/api-mcp/dashboard", token, nil)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 (body=%q)", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["reason"] != "mcp-not-enabled" {
		t.Errorf("reason = %v, want mcp-not-enabled (body=%q)", body["reason"], rr.Body.String())
	}
}

// TestAPIMCP_RejectsMissingEmailClaim is the success-criterion #4 test:
// a syntactically-valid token without an `email` claim is rejected with
// 401 missing-email-claim. This guards against the NULL-email-row
// resolution failure mode (users.email is nullable + UNIQUE permits
// multiple NULLs in SQLite).
func TestAPIMCP_RejectsMissingEmailClaim(t *testing.T) {
	env := newMCPTestEnv(t)

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": env.issuer,
		"aud": env.audience,
		"sub": "subj-no-email",
		"iat": time.Now().Add(-time.Minute).Unix(),
		"exp": time.Now().Add(5 * time.Minute).Unix(),
		// no email claim
	})
	tok.Header["kid"] = env.kid
	signed, _ := tok.SignedString(env.signer)

	rr := env.do(t, http.MethodGet, "/api-mcp/dashboard", signed, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 (body=%q)", rr.Code, rr.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rr.Body.Bytes(), &body)
	if body["reason"] != "missing-email-claim" {
		t.Errorf("reason = %v, want missing-email-claim", body["reason"])
	}
}

// TestAPIMCP_404OnMissingRoute confirms the curated mount surface — a
// route NOT in mcpRouteList returns 404 even when wrapped in /api-mcp/*.
// Belt for the [forbidden] block: /api-mcp/me must not exist.
func TestAPIMCP_404OnMissingRoute(t *testing.T) {
	env := newMCPTestEnv(t)
	env.createCoach(t, "coach", "c@example.com", true)
	token := env.mintTokenForEmail(t, "c@example.com")

	// These would all exist on /api/* (the webui side) but must NOT exist
	// on /api-mcp/*.
	cases := []struct {
		method, path string
	}{
		{http.MethodGet, "/api-mcp/me"},
		{http.MethodPost, "/api-mcp/admin/impersonate/1"},
		{http.MethodPost, "/api-mcp/athletes/1/generations/1/execute"},
		{http.MethodPost, "/api-mcp/athletes/1/training-maxes"},
		{http.MethodPost, "/api-mcp/athletes/1/promote"},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rr := env.do(t, tc.method, tc.path, token, map[string]any{})
			if rr.Code != http.StatusNotFound && rr.Code != http.StatusMethodNotAllowed {
				t.Errorf("status = %d, want 404 or 405 (body=%q)", rr.Code, rr.Body.String())
			}
		})
	}
}
