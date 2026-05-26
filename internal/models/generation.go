package models

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Generation status values. See migration 0002 for the CHECK constraint.
const (
	GenerationPending   = "pending"   // row inserted, goroutine not started yet
	GenerationRunning   = "running"   // LLM call in flight
	GenerationSucceeded = "succeeded" // catalog_json ready for execute
	GenerationFailed    = "failed"    // error column populated
	GenerationCancelled = "cancelled" // coach cancelled before completion
)

// Generation is one AI Coach program-draft request.
//
// The lifecycle is: pending → running → (succeeded | failed | cancelled).
// On succeeded, catalog_json contains the parsed CatalogJSON the coach can
// review and then commit via ExecuteCatalogImport. executed_at is set on
// commit to prevent double-import.
type Generation struct {
	ID          int64
	AthleteID   int64
	RequestedBy int64
	Status      string

	RequestJSON string
	CatalogJSON sql.NullString
	Reasoning   sql.NullString

	Model      sql.NullString
	TokensUsed int
	DurationMS int
	StopReason sql.NullString

	Error sql.NullString

	// Audit payload — see migration 0003. ContextJSON is the marshalled
	// AthleteContext sent to the LLM; Prompt is the system+user prompt
	// that went over the wire. Both NULL on rows created before 0003.
	ContextJSON sql.NullString
	Prompt      sql.NullString

	ExecutedAt  sql.NullTime
	CreatedAt   time.Time
	StartedAt   sql.NullTime
	CompletedAt sql.NullTime
}

// IsTerminal reports whether the generation has reached an end state.
func (g *Generation) IsTerminal() bool {
	return g.Status == GenerationSucceeded ||
		g.Status == GenerationFailed ||
		g.Status == GenerationCancelled
}

// CreateGeneration inserts a pending generation row and returns it.
// requestJSON is the marshalled GenerationRequest snapshot.
func CreateGeneration(db *sql.DB, athleteID, requestedBy int64, requestJSON string) (*Generation, error) {
	row := db.QueryRow(
		`INSERT INTO generations (athlete_id, requested_by, status, request_json)
		 VALUES (?, ?, ?, ?)
		 RETURNING id, created_at`,
		athleteID, requestedBy, GenerationPending, requestJSON,
	)
	g := &Generation{
		AthleteID:   athleteID,
		RequestedBy: requestedBy,
		Status:      GenerationPending,
		RequestJSON: requestJSON,
	}
	if err := row.Scan(&g.ID, &g.CreatedAt); err != nil {
		return nil, fmt.Errorf("models: create generation: %w", err)
	}
	return g, nil
}

// MarkGenerationRunning transitions a pending generation to running and
// stamps started_at. Returns ErrNotFound if the row is missing and a
// descriptive error if the row is not currently pending (e.g. cancelled
// between the insert and the goroutine pickup).
func MarkGenerationRunning(db *sql.DB, id int64) error {
	res, err := db.Exec(
		`UPDATE generations
		    SET status = ?, started_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND status = ?`,
		GenerationRunning, id, GenerationPending,
	)
	if err != nil {
		return fmt.Errorf("models: mark generation %d running: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		// Either the row vanished or its status changed (likely cancelled).
		// The caller treats both as "stop, do not invoke the LLM".
		return ErrNotFound
	}
	return nil
}

// CompleteGeneration marks a running generation as succeeded and stores the
// LLM output plus the audit payload (assembled context + final prompt sent
// to the provider). catalogJSON should be the raw bytes the importer will
// parse. contextJSON / prompt may be empty/zero when called from a path
// that doesn't have them (none today, but we'd rather have the per-arg
// zero-value than a panic).
func CompleteGeneration(db *sql.DB, id int64, catalogJSON, reasoning, model, stopReason string, tokensUsed, durationMS int, contextJSON, prompt string) error {
	var ctxVal sql.NullString
	if contextJSON != "" {
		ctxVal = sql.NullString{String: contextJSON, Valid: true}
	}
	var promptVal sql.NullString
	if prompt != "" {
		promptVal = sql.NullString{String: prompt, Valid: true}
	}
	res, err := db.Exec(
		`UPDATE generations
		    SET status        = ?,
		        catalog_json  = ?,
		        reasoning     = ?,
		        model         = ?,
		        stop_reason   = ?,
		        tokens_used   = ?,
		        duration_ms   = ?,
		        context_json  = ?,
		        prompt        = ?,
		        completed_at  = CURRENT_TIMESTAMP
		  WHERE id = ? AND status = ?`,
		GenerationSucceeded, catalogJSON, reasoning, model, stopReason, tokensUsed, durationMS, ctxVal, promptVal,
		id, GenerationRunning,
	)
	if err != nil {
		return fmt.Errorf("models: complete generation %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// FailGeneration marks a running generation as failed with a user-friendly
// error message. Safe to call after cancellation — the WHERE clause filters
// to running rows so a cancelled row is left untouched.
func FailGeneration(db *sql.DB, id int64, userErr string, durationMS int) error {
	res, err := db.Exec(
		`UPDATE generations
		    SET status       = ?,
		        error        = ?,
		        duration_ms  = ?,
		        completed_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND status = ?`,
		GenerationFailed, userErr, durationMS, id, GenerationRunning,
	)
	if err != nil {
		return fmt.Errorf("models: fail generation %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// CancelGeneration moves a pending or running generation to cancelled.
// Callers use this to honor a coach's "stop" button. If the LLM call has
// already produced a response, the goroutine's CompleteGeneration will
// no-op (status no longer 'running') and the cancelled row stands.
func CancelGeneration(db *sql.DB, id int64) error {
	res, err := db.Exec(
		`UPDATE generations
		    SET status       = ?,
		        completed_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND status IN (?, ?)`,
		GenerationCancelled, id, GenerationPending, GenerationRunning,
	)
	if err != nil {
		return fmt.Errorf("models: cancel generation %d: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkGenerationExecuted stamps executed_at so the SPA can render "imported"
// and the execute handler can reject a second commit of the same draft.
func MarkGenerationExecuted(db *sql.DB, id int64) error {
	res, err := db.Exec(
		`UPDATE generations SET executed_at = CURRENT_TIMESTAMP
		  WHERE id = ? AND executed_at IS NULL`,
		id,
	)
	if err != nil {
		return fmt.Errorf("models: mark generation %d executed: %w", id, err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// GetGeneration loads a single generation by ID. Returns ErrNotFound if
// the row does not exist.
func GetGeneration(db *sql.DB, id int64) (*Generation, error) {
	row := db.QueryRow(
		`SELECT id, athlete_id, requested_by, status, request_json,
		        catalog_json, reasoning, model, tokens_used, duration_ms,
		        stop_reason, error, context_json, prompt,
		        executed_at, created_at, started_at, completed_at
		   FROM generations WHERE id = ?`,
		id,
	)
	g := &Generation{}
	err := row.Scan(
		&g.ID, &g.AthleteID, &g.RequestedBy, &g.Status, &g.RequestJSON,
		&g.CatalogJSON, &g.Reasoning, &g.Model, &g.TokensUsed, &g.DurationMS,
		&g.StopReason, &g.Error, &g.ContextJSON, &g.Prompt,
		&g.ExecutedAt, &g.CreatedAt, &g.StartedAt, &g.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: get generation %d: %w", id, err)
	}
	return g, nil
}

// LatestGenerationForAthlete returns the most recently created generation
// for an athlete, or ErrNotFound if there isn't one. Used by the form-data
// endpoint so the SPA can resume a still-running draft after page reload.
func LatestGenerationForAthlete(db *sql.DB, athleteID int64) (*Generation, error) {
	row := db.QueryRow(
		`SELECT id, athlete_id, requested_by, status, request_json,
		        catalog_json, reasoning, model, tokens_used, duration_ms,
		        stop_reason, error, context_json, prompt,
		        executed_at, created_at, started_at, completed_at
		   FROM generations
		  WHERE athlete_id = ?
		  ORDER BY created_at DESC LIMIT 1`,
		athleteID,
	)
	g := &Generation{}
	err := row.Scan(
		&g.ID, &g.AthleteID, &g.RequestedBy, &g.Status, &g.RequestJSON,
		&g.CatalogJSON, &g.Reasoning, &g.Model, &g.TokensUsed, &g.DurationMS,
		&g.StopReason, &g.Error, &g.ContextJSON, &g.Prompt,
		&g.ExecutedAt, &g.CreatedAt, &g.StartedAt, &g.CompletedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("models: latest generation for athlete %d: %w", athleteID, err)
	}
	return g, nil
}

// PendingOrRunningGenerationForAthlete returns the first pending/running
// generation row for an athlete (id + status only — enough for the
// duplicate-submit guard at the handler boundary). Returns (nil, nil)
// when no draft is in flight. Uses the (athlete_id, status) covering
// index from migration 0002.
func PendingOrRunningGenerationForAthlete(db *sql.DB, athleteID int64) (*Generation, error) {
	row := db.QueryRow(
		`SELECT id, status FROM generations
		  WHERE athlete_id = ? AND status IN (?, ?)
		  ORDER BY created_at DESC LIMIT 1`,
		athleteID, GenerationPending, GenerationRunning,
	)
	g := &Generation{AthleteID: athleteID}
	if err := row.Scan(&g.ID, &g.Status); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("models: lookup in-flight generation for athlete %d: %w", athleteID, err)
	}
	return g, nil
}

// StaleRunningGeneration is the minimal projection ListStaleRunningGenerations
// returns — enough for the startup sweep to fire a per-row notification.
type StaleRunningGeneration struct {
	ID          int64
	AthleteID   int64
	RequestedBy int64
}

// ListStaleRunningGenerations returns every generation currently in
// pending/running. Called at startup before the HTTP server begins
// accepting requests so we can notify each requester after the sweep
// resets the row. Race-free: the prior process's goroutines are dead and
// the current process isn't yet enqueueing new work.
func ListStaleRunningGenerations(db *sql.DB) ([]StaleRunningGeneration, error) {
	rows, err := db.Query(
		`SELECT id, athlete_id, requested_by
		   FROM generations
		  WHERE status IN (?, ?)`,
		GenerationPending, GenerationRunning,
	)
	if err != nil {
		return nil, fmt.Errorf("models: list stale generations: %w", err)
	}
	defer rows.Close()

	var out []StaleRunningGeneration
	for rows.Next() {
		var g StaleRunningGeneration
		if err := rows.Scan(&g.ID, &g.AthleteID, &g.RequestedBy); err != nil {
			return nil, fmt.Errorf("models: scan stale generation: %w", err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("models: iterate stale generations: %w", err)
	}
	return out, nil
}

// ResetStaleRunningGenerations marks every still-`running` generation as
// failed with a "server restarted" message. Called once at startup before
// the HTTP server begins accepting requests so the SPA never shows
// forever-spinning drafts from a crashed/restarted process.
//
// Returns the number of rows reset, for the startup log line. Callers that
// want to notify each requester should call ListStaleRunningGenerations
// FIRST so they have the per-row identifiers — this UPDATE clears them.
func ResetStaleRunningGenerations(db *sql.DB) (int64, error) {
	res, err := db.Exec(
		`UPDATE generations
		    SET status       = ?,
		        error        = 'Server restarted during generation. Please try again.',
		        completed_at = CURRENT_TIMESTAMP
		  WHERE status IN (?, ?)`,
		GenerationFailed, GenerationPending, GenerationRunning,
	)
	if err != nil {
		return 0, fmt.Errorf("models: reset stale generations: %w", err)
	}
	return res.RowsAffected()
}
