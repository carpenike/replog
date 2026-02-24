package models

import (
	"database/sql"
	"testing"
)

func seedCoachUser(t testing.TB, db *sql.DB) *User {
	t.Helper()
	hash, _ := HashPassword("password")
	_, err := db.Exec(
		`INSERT INTO users (username, password_hash, is_coach) VALUES (?, ?, 1)`,
		"testcoach", hash,
	)
	if err != nil {
		t.Fatalf("seed coach user: %v", err)
	}
	user, err := GetUserByUsername(db, "testcoach")
	if err != nil {
		t.Fatalf("get seeded coach: %v", err)
	}
	return user
}

func TestWorkoutReviewCRUD(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Review Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)
	workout, _ := CreateWorkout(db, athlete.ID, "2026-02-15", "test workout", 0)

	t.Run("create review", func(t *testing.T) {
		rev, err := CreateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusApproved, "Great job!")
		if err != nil {
			t.Fatalf("create review: %v", err)
		}
		if rev.Status != ReviewStatusApproved {
			t.Errorf("status = %q, want %q", rev.Status, ReviewStatusApproved)
		}
		if !rev.Notes.Valid || rev.Notes.String != "Great job!" {
			t.Errorf("notes = %v, want Great job!", rev.Notes)
		}
		if !rev.CoachUsername.Valid || rev.CoachUsername.String != "testcoach" {
			t.Errorf("coach username = %v, want testcoach", rev.CoachUsername)
		}
		if rev.WorkoutID != workout.ID {
			t.Errorf("workout_id = %d, want %d", rev.WorkoutID, workout.ID)
		}
	})

	t.Run("duplicate review returns error", func(t *testing.T) {
		_, err := CreateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusApproved, "")
		if err == nil {
			t.Fatal("expected error for duplicate review, got nil")
		}
	})

	t.Run("get review by workout ID", func(t *testing.T) {
		rev, err := GetWorkoutReviewByWorkoutID(db, workout.ID)
		if err != nil {
			t.Fatalf("get review: %v", err)
		}
		if rev.Status != ReviewStatusApproved {
			t.Errorf("status = %q, want %q", rev.Status, ReviewStatusApproved)
		}
	})

	t.Run("update review", func(t *testing.T) {
		existing, _ := GetWorkoutReviewByWorkoutID(db, workout.ID)
		rev, err := UpdateWorkoutReview(db, existing.ID, coach.ID, ReviewStatusNeedsWork, "Try heavier next time")
		if err != nil {
			t.Fatalf("update review: %v", err)
		}
		if rev.Status != ReviewStatusNeedsWork {
			t.Errorf("status = %q, want %q", rev.Status, ReviewStatusNeedsWork)
		}
		if !rev.Notes.Valid || rev.Notes.String != "Try heavier next time" {
			t.Errorf("notes = %v, want 'Try heavier next time'", rev.Notes)
		}
	})

	t.Run("delete review", func(t *testing.T) {
		existing, _ := GetWorkoutReviewByWorkoutID(db, workout.ID)
		if err := DeleteWorkoutReview(db, existing.ID); err != nil {
			t.Fatalf("delete review: %v", err)
		}
		_, err := GetWorkoutReviewByWorkoutID(db, workout.ID)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound after delete, got %v", err)
		}
	})
}

func TestWorkoutReview_NotFound(t *testing.T) {
	db := testDB(t)

	t.Run("get by workout ID not found", func(t *testing.T) {
		_, err := GetWorkoutReviewByWorkoutID(db, 99999)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("get by ID not found", func(t *testing.T) {
		_, err := GetWorkoutReviewByID(db, 99999)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("delete not found", func(t *testing.T) {
		err := DeleteWorkoutReview(db, 99999)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("update not found", func(t *testing.T) {
		_, err := UpdateWorkoutReview(db, 99999, 1, ReviewStatusApproved, "")
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})
}

func TestCreateOrUpdateWorkoutReview(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Upsert Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)
	workout, _ := CreateWorkout(db, athlete.ID, "2026-03-01", "", 0)

	t.Run("creates when none exists", func(t *testing.T) {
		rev, err := CreateOrUpdateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusApproved, "Looks good")
		if err != nil {
			t.Fatalf("create or update: %v", err)
		}
		if rev.Status != ReviewStatusApproved {
			t.Errorf("status = %q, want %q", rev.Status, ReviewStatusApproved)
		}
	})

	t.Run("updates when already exists", func(t *testing.T) {
		rev, err := CreateOrUpdateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusNeedsWork, "Go heavier")
		if err != nil {
			t.Fatalf("create or update: %v", err)
		}
		if rev.Status != ReviewStatusNeedsWork {
			t.Errorf("status = %q, want %q", rev.Status, ReviewStatusNeedsWork)
		}
		if !rev.Notes.Valid || rev.Notes.String != "Go heavier" {
			t.Errorf("notes = %v, want 'Go heavier'", rev.Notes)
		}
	})
}

func TestListUnreviewedWorkouts(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Unreviewed Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)

	w1, _ := CreateWorkout(db, athlete.ID, "2026-02-10", "", 0)
	w2, _ := CreateWorkout(db, athlete.ID, "2026-02-11", "", 0)
	CreateWorkout(db, athlete.ID, "2026-02-12", "", 0)

	// Review w1 only.
	CreateWorkoutReview(db, w1.ID, coach.ID, ReviewStatusApproved, "")

	unreviewed, err := ListUnreviewedWorkouts(db)
	if err != nil {
		t.Fatalf("list unreviewed: %v", err)
	}

	// w2 and w3 should be unreviewed.
	if len(unreviewed) != 2 {
		t.Fatalf("expected 2 unreviewed workouts, got %d", len(unreviewed))
	}

	// Should be ordered by date DESC.
	if len(unreviewed) >= 2 && unreviewed[0].WorkoutID == unreviewed[1].WorkoutID {
		t.Error("unreviewed entries should have different workout IDs")
	}

	// Verify none of them are w1 (which was reviewed).
	for _, uw := range unreviewed {
		if uw.WorkoutID == w1.ID {
			t.Errorf("reviewed workout %d should not appear in unreviewed list", w1.ID)
		}
	}

	_ = w2 // suppress unused
}

func TestGetReviewStats(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Stats Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)

	w1, _ := CreateWorkout(db, athlete.ID, "2026-02-01", "", 0)
	w2, _ := CreateWorkout(db, athlete.ID, "2026-02-02", "", 0)
	CreateWorkout(db, athlete.ID, "2026-02-03", "", 0) // unreviewed

	CreateWorkoutReview(db, w1.ID, coach.ID, ReviewStatusApproved, "")
	CreateWorkoutReview(db, w2.ID, coach.ID, ReviewStatusNeedsWork, "Fix form")

	stats, err := GetReviewStats(db)
	if err != nil {
		t.Fatalf("get review stats: %v", err)
	}

	if stats.PendingCount != 1 {
		t.Errorf("pending = %d, want 1", stats.PendingCount)
	}
	if stats.ApprovedCount != 1 {
		t.Errorf("approved = %d, want 1", stats.ApprovedCount)
	}
	if stats.NeedsWork != 1 {
		t.Errorf("needs_work = %d, want 1", stats.NeedsWork)
	}
}

func TestDeleteCoachPreservesReview(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Preserved Review Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)
	workout, _ := CreateWorkout(db, athlete.ID, "2026-03-10", "", 0)

	// Coach reviews the workout.
	rev, err := CreateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusApproved, "Solid work")
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	// Delete the coach user account.
	if err := DeleteUser(db, coach.ID); err != nil {
		t.Fatalf("delete coach: %v", err)
	}

	// Review should still exist with NULL coach_id.
	got, err := GetWorkoutReviewByID(db, rev.ID)
	if err != nil {
		t.Fatalf("get review after coach delete: %v", err)
	}
	if got.CoachID.Valid {
		t.Errorf("coach_id should be NULL after coach deletion, got %d", got.CoachID.Int64)
	}
	if got.CoachUsername.Valid {
		t.Errorf("coach_username should be NULL after coach deletion, got %q", got.CoachUsername.String)
	}
	if got.Status != ReviewStatusApproved {
		t.Errorf("status = %q, want %q", got.Status, ReviewStatusApproved)
	}
	if !got.Notes.Valid || got.Notes.String != "Solid work" {
		t.Errorf("notes should be preserved, got %v", got.Notes)
	}
}

func TestAutoApproveWorkout(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Auto-Approve Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)
	workout, _ := CreateWorkout(db, athlete.ID, "2026-04-01", "", 0)

	// Auto-approve should create a review.
	if err := AutoApproveWorkout(db, workout.ID, coach.ID); err != nil {
		t.Fatalf("auto-approve: %v", err)
	}

	rev, err := GetWorkoutReviewByWorkoutID(db, workout.ID)
	if err != nil {
		t.Fatalf("get review after auto-approve: %v", err)
	}
	if rev.Status != ReviewStatusApproved {
		t.Errorf("status = %q, want %q", rev.Status, ReviewStatusApproved)
	}
	if !rev.CoachID.Valid || rev.CoachID.Int64 != coach.ID {
		t.Errorf("coach_id = %v, want %d", rev.CoachID, coach.ID)
	}

	// Calling again should be a no-op (no error, no duplicate).
	if err := AutoApproveWorkout(db, workout.ID, coach.ID); err != nil {
		t.Fatalf("auto-approve idempotent: %v", err)
	}

	// Verify it's still just one review.
	var count int
	db.QueryRow(`SELECT COUNT(*) FROM workout_reviews WHERE workout_id = ?`, workout.ID).Scan(&count)
	if count != 1 {
		t.Errorf("review count = %d, want 1", count)
	}
}

func TestAutoApproveWorkout_SkipsExistingReview(t *testing.T) {
	db := testDB(t)

	athlete, _ := CreateAthlete(db, "Skip Approve Athlete", "", "", "", "", "", "", sql.NullInt64{}, true)
	coach := seedCoachUser(t, db)
	workout, _ := CreateWorkout(db, athlete.ID, "2026-04-02", "", 0)

	// Manually mark as needs_work.
	_, err := CreateWorkoutReview(db, workout.ID, coach.ID, ReviewStatusNeedsWork, "Fix your form")
	if err != nil {
		t.Fatalf("create review: %v", err)
	}

	// Auto-approve should not overwrite the existing review.
	if err := AutoApproveWorkout(db, workout.ID, coach.ID); err != nil {
		t.Fatalf("auto-approve with existing review: %v", err)
	}

	rev, err := GetWorkoutReviewByWorkoutID(db, workout.ID)
	if err != nil {
		t.Fatalf("get review: %v", err)
	}
	if rev.Status != ReviewStatusNeedsWork {
		t.Errorf("status = %q, want %q (should not be overwritten)", rev.Status, ReviewStatusNeedsWork)
	}
}
