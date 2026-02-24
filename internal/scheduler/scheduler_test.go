package scheduler

import (
	"database/sql"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/database"
	"github.com/carpenike/replog/internal/models"
)

// testDB creates a fresh in-memory SQLite database with migrations applied.
func testDB(t testing.TB) *sql.DB {
	t.Helper()

	db, err := database.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		db.Close()
		t.Fatalf("run migrations: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func TestSchedulerStartStop(t *testing.T) {
	db := testDB(t)
	s := New(db)
	s.Start()
	// Give the initial maintenance run a moment to complete.
	time.Sleep(100 * time.Millisecond)
	// Stop should return without blocking.
	s.Stop()

	// After startup, Status should reflect the initial run.
	st := s.Status()
	if st.LastRun.IsZero() {
		t.Error("expected LastRun to be set after Start()")
	}
	if st.NextRun.IsZero() {
		t.Error("expected NextRun to be set after Start()")
	}
	if st.IntervalHours == 0 {
		t.Error("expected IntervalHours > 0")
	}
	if st.RetentionDays == 0 {
		t.Error("expected RetentionDays > 0")
	}
}

func TestMaintenanceCleanup(t *testing.T) {
	db := testDB(t)

	// Create a user for tokens and notifications.
	user, err := models.CreateUser(db, "testuser", "", "password", "", false, false, sql.NullInt64{})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Create expired login token.
	past := time.Now().Add(-1 * time.Hour)
	models.CreateLoginToken(db, user.ID, "expired", &past)

	// Create valid login token.
	future := time.Now().Add(24 * time.Hour)
	models.CreateLoginToken(db, user.ID, "valid", &future)

	// Create old read notification.
	models.CreateNotification(db, user.ID, "workout_review", "Old notification", "", "/test", sql.NullInt64{})
	// Mark it read, then backdate it past the 90-day cutoff.
	notifications, _ := models.ListNotifications(db, user.ID, 10, 0)
	if len(notifications) > 0 {
		models.MarkAsRead(db, notifications[0].ID, user.ID)
		db.Exec(`UPDATE notifications SET created_at = datetime('now', '-100 days') WHERE id = ?`, notifications[0].ID)
	}

	// Create recent unread notification.
	models.CreateNotification(db, user.ID, "workout_review", "Recent notification", "", "/test2", sql.NullInt64{})

	// Run maintenance directly.
	s := &Scheduler{db: db}
	s.runMaintenance()

	// Verify status was recorded.
	st := s.Status()
	if st.LastRun.IsZero() {
		t.Error("expected LastRun to be set after runMaintenance()")
	}
	if st.TokensDeleted != 1 {
		t.Errorf("TokensDeleted = %d, want 1", st.TokensDeleted)
	}
	if st.NotificationsPruned != 1 {
		t.Errorf("NotificationsPruned = %d, want 1", st.NotificationsPruned)
	}

	// Expired token should be gone, valid token should remain.
	tokens, _ := models.ListLoginTokensByUser(db, user.ID)
	if len(tokens) != 1 {
		t.Errorf("tokens remaining = %d, want 1", len(tokens))
	}

	// Old read notification should be pruned, recent one should remain.
	remaining, _ := models.ListNotifications(db, user.ID, 10, 0)
	if len(remaining) != 1 {
		t.Errorf("notifications remaining = %d, want 1", len(remaining))
	}
}

func TestMaintenanceReadsSettings(t *testing.T) {
	db := testDB(t)

	// Override the default interval to 48 hours.
	models.SetSetting(db, "maintenance.interval_hours", "48")
	// Override the default retention to 30 days.
	models.SetSetting(db, "maintenance.retention_days", "30")

	s := &Scheduler{db: db}
	s.runMaintenance()

	st := s.Status()
	if st.IntervalHours != 48 {
		t.Errorf("IntervalHours = %d, want 48", st.IntervalHours)
	}
	if st.RetentionDays != 30 {
		t.Errorf("RetentionDays = %d, want 30", st.RetentionDays)
	}
}
