package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carpenike/replog/internal/api"
	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

// mcpTestEnv stages the native MCP server's in-process invoker against a
// fresh in-memory DB, so we can exercise tool calls end-to-end through the
// SAME REST handlers the webui uses — and prove the bearer-authenticated MCP
// path inherits identical ownership checks (ADR 019 Phase 3 parity).
type mcpTestEnv struct {
	db *sql.DB
	h  *api.Handlers
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

	sm := scs.New()
	sm.Lifetime = 30 * 24 * time.Hour
	h := &api.Handlers{DB: db, Sessions: sm, AvatarDir: t.TempDir()}

	return &mcpTestEnv{db: db, h: h}
}

// invokerFor builds the per-identity invoker the streamable handler would
// construct for an authenticated MCP session, with default preferences.
func (e *mcpTestEnv) invokerFor(user *models.User) *mcpInvoker {
	return &mcpInvoker{
		h:    e.h,
		user: user,
		prefs: &models.UserPreferences{
			UserID:     user.ID,
			WeightUnit: models.DefaultWeightUnit,
			Timezone:   models.DefaultTimezone,
			DateFormat: models.DefaultDateFormat,
		},
	}
}

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

// toolText extracts the first text payload from a tool result for assertions.
func toolText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// --- end-to-end tests -------------------------------------------------------

// TestMCPInvoker_CanManageAthlete_ParityWithWebui asserts that an MCP tool
// call inherits the SAME ownership checks as the webui — a coach cannot reach
// an athlete assigned to a different coach. The invoker injects the
// authenticated user into the request context exactly as the scs and
// opaque-token middleware do, so CanAccessAthlete / CanManageAthlete enforce
// identically on both surfaces (ADR 019 Phase 3 parity claim).
func TestMCPInvoker_CanManageAthlete_ParityWithWebui(t *testing.T) {
	env := newMCPTestEnv(t)

	coachA := env.createCoach(t, "coachA", "a@example.com", true)
	coachB := env.createCoach(t, "coachB", "b@example.com", true)
	athleteOfA := env.createAthleteFor(t, "Alpha", coachA.ID)
	athleteOfB := env.createAthleteFor(t, "Bravo", coachB.ID)

	invA := env.invokerFor(coachA)
	ctx := context.Background()

	t.Run("coachA can read their own athlete", func(t *testing.T) {
		res, _, err := invA.run(ctx, invA.h.GetAthlete, http.MethodGet,
			map[string]string{"id": i64(athleteOfA.ID)}, nil, nil)
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		if res.IsError {
			t.Errorf("expected success, got tool error: %s", toolText(t, res))
		}
	})

	t.Run("coachA cannot read coachB's athlete", func(t *testing.T) {
		res, _, err := invA.run(ctx, invA.h.GetAthlete, http.MethodGet,
			map[string]string{"id": i64(athleteOfB.ID)}, nil, nil)
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected ownership denial (tool error), got success: %s", toolText(t, res))
		}
	})

	t.Run("coachA can log a workout + set for their own athlete", func(t *testing.T) {
		today := time.Now().UTC().Format("2006-01-02")
		res, _, err := invA.run(ctx, invA.h.CreateWorkout, http.MethodPost,
			map[string]string{"id": i64(athleteOfA.ID)}, nil,
			api.WorkoutRequest{Date: today})
		if err != nil {
			t.Fatalf("create workout: %v", err)
		}
		if res.IsError {
			t.Fatalf("create workout tool error: %s", toolText(t, res))
		}
		var workout map[string]any
		if err := json.Unmarshal([]byte(toolText(t, res)), &workout); err != nil {
			t.Fatalf("decode workout: %v", err)
		}
		workoutID := int64(workout["id"].(float64))

		ex, err := models.CreateExercise(env.db, "TestSquat", "", "", "", 0, false)
		if err != nil {
			t.Fatalf("create exercise: %v", err)
		}

		res, _, err = invA.run(ctx, invA.h.AddWorkoutSet, http.MethodPost,
			map[string]string{"id": i64(athleteOfA.ID), "workoutID": i64(workoutID)}, nil,
			api.WorkoutSetRequest{ExerciseID: ex.ID, Reps: 5, Weight: 135.0})
		if err != nil {
			t.Fatalf("add set: %v", err)
		}
		if res.IsError {
			t.Fatalf("add set tool error: %s", toolText(t, res))
		}
	})

	t.Run("coachA cannot log a workout for coachB's athlete", func(t *testing.T) {
		today := time.Now().UTC().Format("2006-01-02")
		res, _, err := invA.run(ctx, invA.h.CreateWorkout, http.MethodPost,
			map[string]string{"id": i64(athleteOfB.ID)}, nil,
			api.WorkoutRequest{Date: today})
		if err != nil {
			t.Fatalf("invoke: %v", err)
		}
		if !res.IsError {
			t.Errorf("expected ownership denial (tool error), got success: %s", toolText(t, res))
		}
	})
}
