package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/carpenike/replog/internal/api"
	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Native MCP server (ADR 019 Phase 3 — HOF-013).
//
// RepLog hosts its own Model Context Protocol server at /api/mcp using the
// official go-sdk. It OWNS the tool catalog: the canonical list in mcpTools()
// is the single source of truth, and that list IS the doctrine boundary —
// only clerical logbook reads/writes and the draft-generation enqueue/status/
// cancel are present. No coaching-decision tool (training-max edits, program
// assignment, promotion, cycle-review apply, generation EXECUTE, template/rule
// authoring, season-phase writes) is registered. Their ABSENCE is the safety
// property asserted by mcp_server_test.go (ADR 007 / 015 "no automated
// coaching").
//
// Each tool reuses the EXISTING REST handler rather than reimplementing logic
// or authz. mcpInvoker synthesizes an in-process *http.Request — path params
// via http.Request.SetPathValue (the handlers read r.PathValue), the
// authenticated user + prefs injected under the same context keys the
// scs-cookie and opaque-token middleware use — runs the handler against an
// httptest recorder, and returns the JSON body as MCP text content. This keeps
// CanAccessAthlete / CanManageAthlete / coach-gate checks on the single
// enforced path.

// mcpInvoker carries the per-request identity and the handler set for a single
// authenticated MCP session.
type mcpInvoker struct {
	h     *api.Handlers
	user  *models.User
	prefs *models.UserPreferences
}

// run executes an existing REST handler in-process and packs its response into
// an MCP tool result. pathParams populate r.PathValue; body (when non-nil) is
// JSON-marshaled as the request body. A >=400 status is surfaced as an MCP
// tool error (IsError) carrying the handler's JSON error body, so the model
// can see and self-correct rather than receiving an opaque protocol failure.
func (inv *mcpInvoker) run(
	ctx context.Context,
	handler http.HandlerFunc,
	method string,
	pathParams map[string]string,
	query url.Values,
	body any,
) (*mcp.CallToolResult, any, error) {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		reader = bytes.NewReader(b)
	}

	target := "/"
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range pathParams {
		req.SetPathValue(k, v)
	}

	rc := context.WithValue(ctx, middleware.UserContextKey, inv.user)
	rc = context.WithValue(rc, middleware.PrefsContextKey, inv.prefs)
	req = req.WithContext(rc)

	rec := httptest.NewRecorder()
	handler(rec, req)

	res := rec.Result()
	defer res.Body.Close()
	payload, _ := io.ReadAll(res.Body)

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(payload)}},
		IsError: res.StatusCode >= 400,
	}, nil, nil
}

// mcpTool binds a tool name to its registration closure. The name is the
// single source of truth shared by buildMCPServer (which registers from this
// list) and mcp_server_test.go (which asserts the list).
type mcpTool struct {
	name string
	reg  func(s *mcp.Server, inv *mcpInvoker)
}

// readTool registers a GET-style tool: typed input → path params, no body.
func readTool[In any](
	name, desc, method string,
	pick func(*api.Handlers) http.HandlerFunc,
	params func(In) map[string]string,
) mcpTool {
	return mcpTool{name: name, reg: func(s *mcp.Server, inv *mcpInvoker) {
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				return inv.run(ctx, pick(inv.h), method, params(in), nil, nil)
			})
	}}
}

// writeTool registers a mutating tool: typed input → path params + JSON body.
func writeTool[In any](
	name, desc, method string,
	pick func(*api.Handlers) http.HandlerFunc,
	params func(In) map[string]string,
	body func(In) any,
) mcpTool {
	return mcpTool{name: name, reg: func(s *mcp.Server, inv *mcpInvoker) {
		mcp.AddTool(s, &mcp.Tool{Name: name, Description: desc},
			func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
				return inv.run(ctx, pick(inv.h), method, params(in), nil, body(in))
			})
	}}
}

func i64(v int64) string { return strconv.FormatInt(v, 10) }

// --- tool input types -------------------------------------------------------
//
// Identifier-only inputs carry the path params; write inputs embed the
// existing api request DTO so the wire shape (and its JSON schema) stays in
// lockstep with the REST endpoint without duplicating field definitions.

type athleteInput struct {
	AthleteID int64 `json:"athlete_id"`
}
type workoutInput struct {
	AthleteID int64 `json:"athlete_id"`
	WorkoutID int64 `json:"workout_id"`
}
type setInput struct {
	AthleteID int64 `json:"athlete_id"`
	WorkoutID int64 `json:"workout_id"`
	SetID     int64 `json:"set_id"`
}
type exerciseTMInput struct {
	AthleteID  int64 `json:"athlete_id"`
	ExerciseID int64 `json:"exercise_id"`
}
type generationInput struct {
	AthleteID    int64 `json:"athlete_id"`
	GenerationID int64 `json:"generation_id"`
}

type createWorkoutInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.WorkoutRequest
}
type addSetInput struct {
	AthleteID int64 `json:"athlete_id"`
	WorkoutID int64 `json:"workout_id"`
	api.WorkoutSetRequest
}
type updateSetInput struct {
	AthleteID int64 `json:"athlete_id"`
	WorkoutID int64 `json:"workout_id"`
	SetID     int64 `json:"set_id"`
	api.WorkoutSetUpdateRequest
}
type workoutNotesInput struct {
	AthleteID int64 `json:"athlete_id"`
	WorkoutID int64 `json:"workout_id"`
	api.WorkoutNotesRequest
}
type bodyWeightInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.BodyWeightRequest
}
type athleteNoteInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.AthleteNoteRequest
}
type throwingInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.ThrowingSessionRequest
}
type conditioningInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.ConditioningSessionRequest
}
type skillInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.SkillSessionRequest
}
type recoveryInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.RecoveryCheckinRequest
}
type bioSampleInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.BioSampleRequest
}
type generateInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.GenerateSubmitRequest
}
type wodSubmitInput struct {
	AthleteID int64 `json:"athlete_id"`
	api.WODSubmitRequest
}
type wodLogInput struct {
	AthleteID    int64 `json:"athlete_id"`
	GenerationID int64 `json:"generation_id"`
	api.WODLogRequest
}

func athleteParams(in athleteInput) map[string]string {
	return map[string]string{"id": i64(in.AthleteID)}
}

// mcpTools is the canonical MCP tool catalog. Adding a tool here exposes it to
// MCP clients; the test asserts both the exact allowlist AND the absence of
// every forbidden coaching-decision tool.
func mcpTools() []mcpTool {
	return []mcpTool{
		// --- reads ---
		readTool[struct{}]("dashboard", "Coach dashboard: athletes, pending reviews, and recent activity.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.Dashboard },
			func(struct{}) map[string]string { return nil }),
		readTool[athleteInput]("get_athlete", "Get one athlete's profile and program summary.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetAthlete }, athleteParams),
		readTool[athleteInput]("list_workouts", "List an athlete's resistance-training workouts.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListWorkouts }, athleteParams),
		readTool[workoutInput]("get_workout", "Get one workout with its logged sets.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetWorkout },
			func(in workoutInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "workoutID": i64(in.WorkoutID)}
			}),
		readTool[athleteInput]("get_prescription", "Get the athlete's prescribed workout for today.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetPrescription }, athleteParams),
		readTool[athleteInput]("list_training_maxes", "List the athlete's current training maxes.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListTrainingMaxes }, athleteParams),
		readTool[exerciseTMInput]("get_training_max_history", "Get an exercise's training-max history for an athlete.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetTrainingMaxHistory },
			func(in exerciseTMInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "exerciseID": i64(in.ExerciseID)}
			}),
		readTool[athleteInput]("list_journal", "List the athlete's journal entries and notes.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListJournalEntries }, athleteParams),
		readTool[athleteInput]("list_athlete_programs", "List the programs assigned to an athlete.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListAthletePrograms }, athleteParams),
		readTool[athleteInput]("list_athlete_equipment", "List the athlete's available equipment.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListAthleteEquipment }, athleteParams),
		readTool[athleteInput]("get_load_summary", "Get the athlete's training-load summary (ACWR and trends).",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetLoadSummary }, athleteParams),
		readTool[athleteInput]("get_pitch_smart", "Get the athlete's Pitch Smart compliance status.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GetPitchSmartStatus }, athleteParams),
		readTool[athleteInput]("list_throwing_sessions", "List the athlete's logged throwing sessions.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListThrowingSessions }, athleteParams),
		readTool[athleteInput]("list_conditioning_sessions", "List the athlete's logged conditioning sessions.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListConditioningSessions }, athleteParams),
		readTool[athleteInput]("list_skill_sessions", "List the athlete's logged skill sessions.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListSkillSessions }, athleteParams),
		readTool[athleteInput]("list_recovery_checkins", "List the athlete's recovery check-ins.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListRecoveryCheckins }, athleteParams),
		readTool[athleteInput]("list_bio_samples", "List the athlete's biometric samples.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.ListBioSamples }, athleteParams),

		// --- clerical writes (gated by CanManageAthlete in the handlers) ---
		writeTool[createWorkoutInput]("create_workout", "Create a resistance-training workout for an athlete on a date.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateWorkout },
			athleteParamsFrom(func(in createWorkoutInput) int64 { return in.AthleteID }),
			func(in createWorkoutInput) any { return in.WorkoutRequest }),
		writeTool[addSetInput]("add_workout_set", "Add a set to a workout.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.AddWorkoutSet },
			func(in addSetInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "workoutID": i64(in.WorkoutID)}
			},
			func(in addSetInput) any { return in.WorkoutSetRequest }),
		writeTool[updateSetInput]("update_workout_set", "Update an existing set's reps, weight, RPE, or notes.",
			http.MethodPut, func(h *api.Handlers) http.HandlerFunc { return h.UpdateWorkoutSet },
			func(in updateSetInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "workoutID": i64(in.WorkoutID), "setID": i64(in.SetID)}
			},
			func(in updateSetInput) any { return in.WorkoutSetUpdateRequest }),
		writeTool[setInput]("delete_workout_set", "Delete a set from a workout.",
			http.MethodDelete, func(h *api.Handlers) http.HandlerFunc { return h.DeleteWorkoutSet },
			func(in setInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "workoutID": i64(in.WorkoutID), "setID": i64(in.SetID)}
			},
			func(setInput) any { return nil }),
		writeTool[workoutNotesInput]("update_workout_notes", "Set the free-text notes on a workout.",
			http.MethodPut, func(h *api.Handlers) http.HandlerFunc { return h.UpdateWorkoutNotes },
			func(in workoutNotesInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "workoutID": i64(in.WorkoutID)}
			},
			func(in workoutNotesInput) any { return in.WorkoutNotesRequest }),
		writeTool[bodyWeightInput]("create_body_weight", "Log a body-weight measurement for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateBodyWeight },
			athleteParamsFrom(func(in bodyWeightInput) int64 { return in.AthleteID }),
			func(in bodyWeightInput) any { return in.BodyWeightRequest }),
		writeTool[athleteNoteInput]("create_athlete_note", "Add a journal note for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateAthleteNote },
			athleteParamsFrom(func(in athleteNoteInput) int64 { return in.AthleteID }),
			func(in athleteNoteInput) any { return in.AthleteNoteRequest }),
		writeTool[throwingInput]("create_throwing_session", "Log a throwing session for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateThrowingSession },
			athleteParamsFrom(func(in throwingInput) int64 { return in.AthleteID }),
			func(in throwingInput) any { return in.ThrowingSessionRequest }),
		writeTool[conditioningInput]("create_conditioning_session", "Log a conditioning session for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateConditioningSession },
			athleteParamsFrom(func(in conditioningInput) int64 { return in.AthleteID }),
			func(in conditioningInput) any { return in.ConditioningSessionRequest }),
		writeTool[skillInput]("create_skill_session", "Log a skill session for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateSkillSession },
			athleteParamsFrom(func(in skillInput) int64 { return in.AthleteID }),
			func(in skillInput) any { return in.SkillSessionRequest }),
		writeTool[recoveryInput]("create_recovery_checkin", "Log a recovery check-in for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateRecoveryCheckin },
			athleteParamsFrom(func(in recoveryInput) int64 { return in.AthleteID }),
			func(in recoveryInput) any { return in.RecoveryCheckinRequest }),
		writeTool[bioSampleInput]("create_bio_sample", "Log a biometric sample for an athlete.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.CreateBioSample },
			athleteParamsFrom(func(in bioSampleInput) int64 { return in.AthleteID }),
			func(in bioSampleInput) any { return in.BioSampleRequest }),

		// --- program draft (enqueue + status + cancel only) ---
		// generation EXECUTE is INTENTIONALLY ABSENT — the commit step stays on
		// the webui where the human's click is the approval (ADR 007 / 015).
		writeTool[generateInput]("generate_submit", "Enqueue an AI-drafted program proposal for a coach to review.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.GenerateSubmit },
			athleteParamsFrom(func(in generateInput) int64 { return in.AthleteID }),
			func(in generateInput) any { return in.GenerateSubmitRequest }),
		readTool[generationInput]("generation_status", "Poll the status of a draft-program generation.",
			http.MethodGet, func(h *api.Handlers) http.HandlerFunc { return h.GenerationStatus },
			func(in generationInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "genID": i64(in.GenerationID)}
			}),
		writeTool[generationInput]("generation_cancel", "Cancel an in-flight draft-program generation.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.GenerationCancel },
			func(in generationInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "genID": i64(in.GenerationID)}
			},
			func(generationInput) any { return nil }),

		// --- ad-hoc WOD (submit + log; status reuses generation_status) ---
		// wod_log is the ONE commit exposed past the generation_execute line,
		// and it is exposed deliberately. It is "execute-shaped" — it calls
		// MarkGenerationExecuted and sets the SAME ExecutedAt column
		// generation_execute sets — but what it MATERIALIZES is categorically a
		// Group-B log, not a coaching commit: LogWODFromCatalog writes an
		// assignment_id-NULL ad-hoc resistance workout (no program assignment,
		// no progression, no training-max change, fully reversible), the same
		// class as create_workout. generation_execute, by contrast, commits an
		// assigned multi-week program that drives progression — a coaching
		// decision that stays on the webui (ADR 007 / 015). The boundary is
		// WHAT is committed (an ad-hoc logged session), not whether ExecutedAt
		// is set. Both tools are coach-gated (IsCoach||IsAdmin) + CanAccessAthlete
		// through the reused handlers, and wod_submit is adult-only — a non-coach
		// athlete identity cannot drive this, same as generate_submit.
		writeTool[wodSubmitInput]("wod_submit", "Generate an ad-hoc Sarge-circuit WOD (adult athlete) for review. Returns a generation_id — poll it with generation_status; the result is a proposal to review before logging.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.WODSubmit },
			athleteParamsFrom(func(in wodSubmitInput) int64 { return in.AthleteID }),
			func(in wodSubmitInput) any { return in.WODSubmitRequest }),
		writeTool[wodLogInput]("wod_log", "Log a reviewed WOD to the athlete's logbook as an ad-hoc resistance workout. If a workout already exists for the date, returns collision:true — ask the user to replace or cancel, then re-call with replace:true to supersede it.",
			http.MethodPost, func(h *api.Handlers) http.HandlerFunc { return h.WODLog },
			func(in wodLogInput) map[string]string {
				return map[string]string{"id": i64(in.AthleteID), "genID": i64(in.GenerationID)}
			},
			func(in wodLogInput) any { return in.WODLogRequest }),
	}
}

// athleteParamsFrom builds a path-param func that extracts the athlete id from
// an arbitrary input type via the supplied accessor.
func athleteParamsFrom[In any](get func(In) int64) func(In) map[string]string {
	return func(in In) map[string]string {
		return map[string]string{"id": i64(get(in))}
	}
}

// buildMCPServer constructs a per-identity MCP server with the full canonical
// catalog registered. A fresh server is built per authenticated request by the
// streamable HTTP handler so each session's tools close over that caller's
// user + prefs.
func buildMCPServer(h *api.Handlers, user *models.User, prefs *models.UserPreferences) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "replog",
		Title:   "RepLog",
		Version: "1.0.0",
	}, nil)
	inv := &mcpInvoker{h: h, user: user, prefs: prefs}
	for _, t := range mcpTools() {
		t.reg(s, inv)
	}
	return s
}

// newMCPHTTPHandler returns the streamable-HTTP MCP endpoint. The opaque-token
// middleware authenticates and attaches the user/prefs to the request context
// upstream; getServer reads them here to build the per-identity server.
//
// DisableLocalhostProtection is set deliberately: the go-sdk's default
// DNS-rebinding guard rejects requests that ARRIVE from a localhost address but
// carry a non-localhost Host header — exactly the shape of a request proxied by
// the co-located reverse proxy in front of RepLog. RepLog performs its own
// bearer authentication and is served same-origin, so the guard is redundant
// here and would otherwise 403 every proxied call.
func newMCPHTTPHandler(h *api.Handlers) http.Handler {
	getServer := func(r *http.Request) *mcp.Server {
		user := middleware.UserFromContext(r.Context())
		if user == nil {
			return nil // → 400; should not happen behind the auth middleware
		}
		prefs := middleware.PrefsFromContext(r.Context())
		return buildMCPServer(h, user, prefs)
	}
	return mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		DisableLocalhostProtection: true,
		JSONResponse:               true,
	})
}
