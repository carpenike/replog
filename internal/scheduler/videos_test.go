package scheduler

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// TestSweepOrphanVideos checks the maintenance sweep removes clip files no
// row references (left by a cascade delete) and keeps referenced ones.
func TestSweepOrphanVideos(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	dir := t.TempDir()

	a, err := models.CreateAthlete(ctx, db, "Sam", "", "", "", "", "", "", sql.NullInt64{}, false)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := models.CreateExercise(ctx, db, "Squat", "", "", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	w, err := models.CreateWorkout(ctx, db, a.ID, "2026-11-20", "", 0)
	if err != nil {
		t.Fatal(err)
	}
	set, err := models.AddSet(ctx, db, w.ID, ex.ID, 5, 100, 0, "reps", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := models.ReplaceSetVideo(ctx, db, set.ID, models.SetVideoInput{
		FileName: "kept.mp4", ContentType: "video/mp4", SizeBytes: 1,
	}); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	for _, n := range []string{"kept.mp4", "orphan.mp4"} {
		p := filepath.Join(dir, n)
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
		_ = os.Chtimes(p, old, old)
	}

	s := New(db)
	s.VideoDir = dir
	if n := s.sweepOrphanVideos(); n != 1 {
		t.Fatalf("swept %d, want 1", n)
	}
	if _, err := os.Stat(filepath.Join(dir, "kept.mp4")); err != nil {
		t.Fatal("referenced clip was swept")
	}

	// Deleting the athlete cascades the row; the next sweep takes the file.
	if _, err := db.Exec(`DELETE FROM athletes WHERE id = ?`, a.ID); err != nil {
		t.Fatal(err)
	}
	if n := s.sweepOrphanVideos(); n != 1 {
		t.Fatalf("after cascade swept %d, want 1", n)
	}

	s.VideoDir = ""
	if n := s.sweepOrphanVideos(); n != 0 {
		t.Fatalf("disabled sweep removed %d", n)
	}
}
