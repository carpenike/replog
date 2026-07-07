package models

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// AuditLog is one append-only record of a privileged identity action (e.g.
// an admin/coach starting or stopping impersonation). RealUserID is the
// acting identity; TargetUserID is the user acted upon (NULL when an action
// has no distinct target).
type AuditLog struct {
	ID           int64
	RealUserID   int64
	TargetUserID sql.NullInt64
	Action       string
	Details      sql.NullString
	CreatedAt    time.Time
}

// WriteAuditLog appends an audit record. Callers treat this as best-effort:
// a failure to audit must not fail the underlying request. details may be
// empty (stored as NULL).
func WriteAuditLog(ctx context.Context, db *sql.DB, realUserID int64, targetUserID sql.NullInt64, action, details string) error {
	var detailsVal sql.NullString
	if details != "" {
		detailsVal = sql.NullString{String: details, Valid: true}
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO audit_log (real_user_id, target_user_id, action, details)
		 VALUES (?, ?, ?, ?)`,
		realUserID, targetUserID, action, detailsVal,
	)
	if err != nil {
		return errors.New("models: write audit log: " + err.Error())
	}
	return nil
}

// ListAuditLog returns the most recent audit records, newest first, capped at
// limit rows (limit <= 0 falls back to 100).
func ListAuditLog(ctx context.Context, db *sql.DB, limit int) ([]AuditLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, real_user_id, target_user_id, action, details, created_at
		 FROM audit_log ORDER BY created_at DESC, id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, errors.New("models: list audit log: " + err.Error())
	}
	defer rows.Close()

	var entries []AuditLog
	for rows.Next() {
		var e AuditLog
		if err := rows.Scan(&e.ID, &e.RealUserID, &e.TargetUserID, &e.Action, &e.Details, &e.CreatedAt); err != nil {
			return nil, errors.New("models: scan audit log: " + err.Error())
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.New("models: list audit log: " + err.Error())
	}
	return entries, nil
}
