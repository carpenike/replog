package models

import (
	"context"
	"database/sql"
	"testing"
)

func TestWriteAuditLog(t *testing.T) {
	db := testDB(t)

	t.Run("insert with target and details", func(t *testing.T) {
		err := WriteAuditLog(context.Background(), db, 1, sql.NullInt64{Int64: 2, Valid: true}, "impersonate_start", "admin -> kid")
		if err != nil {
			t.Fatalf("write audit log: %v", err)
		}

		entries, err := ListAuditLog(context.Background(), db, 10)
		if err != nil {
			t.Fatalf("list audit log: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("len = %d, want 1", len(entries))
		}
		e := entries[0]
		if e.RealUserID != 1 {
			t.Errorf("real_user_id = %d, want 1", e.RealUserID)
		}
		if !e.TargetUserID.Valid || e.TargetUserID.Int64 != 2 {
			t.Errorf("target_user_id = %v, want 2", e.TargetUserID)
		}
		if e.Action != "impersonate_start" {
			t.Errorf("action = %q, want impersonate_start", e.Action)
		}
		if !e.Details.Valid || e.Details.String != "admin -> kid" {
			t.Errorf("details = %v, want %q", e.Details, "admin -> kid")
		}
		if e.CreatedAt.IsZero() {
			t.Error("created_at should be set")
		}
	})

	t.Run("empty details stored as NULL", func(t *testing.T) {
		err := WriteAuditLog(context.Background(), db, 5, sql.NullInt64{}, "impersonate_stop", "")
		if err != nil {
			t.Fatalf("write audit log: %v", err)
		}

		entries, err := ListAuditLog(context.Background(), db, 1)
		if err != nil {
			t.Fatalf("list audit log: %v", err)
		}
		if len(entries) != 1 {
			t.Fatalf("len = %d, want 1", len(entries))
		}
		if entries[0].Details.Valid {
			t.Errorf("details should be NULL, got %q", entries[0].Details.String)
		}
		if entries[0].TargetUserID.Valid {
			t.Errorf("target_user_id should be NULL, got %d", entries[0].TargetUserID.Int64)
		}
	})
}
