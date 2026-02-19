package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// ---------------------------------------------------------------------------
// Timeline (GET)
// ---------------------------------------------------------------------------

func TestJournal_Timeline_CoachAccess(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "foundational")

	h := &Journal{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/journal", nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.Timeline(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestJournal_Timeline_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "foundational")
	nonCoach := seedNonCoach(t, db, a.ID)

	h := &Journal{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/journal", nil, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.Timeline(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestJournal_Timeline_NonCoachOtherAthleteForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "My Kid", "")
	otherAthlete := seedAthlete(t, db, "Other Kid", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &Journal{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/journal", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.Timeline(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_Timeline_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/999/journal", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Timeline(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJournal_Timeline_InvalidAthleteID(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/abc/journal", nil, coach)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.Timeline(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// CreateNote (POST)
// ---------------------------------------------------------------------------

func TestJournal_CreateNote_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {"Great progress today!"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify note was created.
	notes, err := models.ListAthleteNotes(db, a.ID, true)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if notes[0].Content != "Great progress today!" {
		t.Errorf("content = %q, want %q", notes[0].Content, "Great progress today!")
	}
	if notes[0].IsPrivate {
		t.Error("expected public note, got private")
	}
}

func TestJournal_CreateNote_Private(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":       {"2026-03-01"},
		"content":    {"Private coaching observation"},
		"is_private": {"1"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	notes, err := models.ListAthleteNotes(db, a.ID, true)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if !notes[0].IsPrivate {
		t.Error("expected private note")
	}
}

func TestJournal_CreateNote_Pinned(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {"Important: check form on squats"},
		"pinned":  {"1"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	notes, err := models.ListAthleteNotes(db, a.ID, true)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	if !notes[0].Pinned {
		t.Error("expected pinned note")
	}
}

func TestJournal_CreateNote_EmptyContent(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {""},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestJournal_CreateNote_DefaultDate(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	// Omit date — should default to today.
	form := url.Values{
		"content": {"Note without explicit date"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	notes, err := models.ListAthleteNotes(db, a.ID, true)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("expected 1 note, got %d", len(notes))
	}
	// Date should be non-empty (defaults to today).
	if notes[0].Date == "" {
		t.Error("expected date to be set")
	}
}

func TestJournal_CreateNote_OwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {"My own journal note"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestJournal_CreateNote_OtherAthleteForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a1 := seedAthlete(t, db, "Kid1", "")
	a2 := seedAthlete(t, db, "Kid2", "")
	nonCoach := seedNonCoach(t, db, a1.ID)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {"Trying to post on someone else's journal"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a2.ID)+"/notes", form, nonCoach)
	req.SetPathValue("id", itoa(a2.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_CreateNote_NonCoachPrivatePinnedIgnored(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":       {"2026-03-01"},
		"content":    {"My note"},
		"is_private": {"1"},
		"pinned":     {"1"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/notes", form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", rr.Code)
	}

	// Verify the note was created without private/pinned flags.
	notes, err := models.ListAthleteNotes(db, a.ID, true)
	if err != nil {
		t.Fatalf("list notes: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected at least one note")
	}
	if notes[0].IsPrivate {
		t.Error("non-coach note should not be private")
	}
	if notes[0].Pinned {
		t.Error("non-coach note should not be pinned")
	}
}

func TestJournal_CreateNote_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"date":    {"2026-03-01"},
		"content": {"Note for nonexistent athlete"},
	}
	req := requestWithUser("POST", "/athletes/999/notes", form, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJournal_CreateNote_InvalidAthleteID(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"test"},
	}
	req := requestWithUser("POST", "/athletes/abc/notes", form, coach)
	req.SetPathValue("id", "abc")
	rr := httptest.NewRecorder()
	h.CreateNote(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// UpdateNote (POST)
// ---------------------------------------------------------------------------

func TestJournal_UpdateNote_AuthorSuccess(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	note, _ := models.CreateAthleteNote(db, a.ID, nonCoach.ID, "2026-03-01", "Original content", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"Updated content"},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetAthleteNoteByID(db, note.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if updated.Content != "Updated content" {
		t.Errorf("content = %q, want %q", updated.Content, "Updated content")
	}
}

func TestJournal_UpdateNote_CoachAuthorWithFlags(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	note, _ := models.CreateAthleteNote(db, a.ID, coach.ID, "2026-03-01", "Coach note", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content":    {"Updated coach note"},
		"is_private": {"1"},
		"pinned":     {"1"},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetAthleteNoteByID(db, note.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if updated.Content != "Updated coach note" {
		t.Errorf("content = %q, want %q", updated.Content, "Updated coach note")
	}
	if !updated.IsPrivate {
		t.Error("expected private flag to be set")
	}
	if !updated.Pinned {
		t.Error("expected pinned flag to be set")
	}
}

func TestJournal_UpdateNote_NonCoachFlagsPreserved(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	// Coach creates a private+pinned note, then transfer authorship to nonCoach
	// for testing. Actually, let's just create the note as the nonCoach and have
	// a coach set the flags first.
	note, _ := models.CreateAthleteNote(db, a.ID, nonCoach.ID, "2026-03-01", "My note", true, true)

	h := &Journal{DB: db, Templates: tc}

	// Non-coach sends private+pinned form values — these should be ignored.
	form := url.Values{
		"content":    {"Updated by me"},
		"is_private": {"1"},
		"pinned":     {"1"},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetAthleteNoteByID(db, note.ID)
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	if updated.Content != "Updated by me" {
		t.Errorf("content = %q, want %q", updated.Content, "Updated by me")
	}
	// Flags should be preserved from the original note, not changed by non-coach.
	if !updated.IsPrivate {
		t.Error("expected private flag preserved")
	}
	if !updated.Pinned {
		t.Error("expected pinned flag preserved")
	}
}

func TestJournal_UpdateNote_NonAuthorForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	// Coach authored the note.
	note, _ := models.CreateAthleteNote(db, a.ID, coach.ID, "2026-03-01", "Coach's note", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"Trying to edit coach's note"},
	}
	// Non-coach (athlete) tries to edit coach's note — forbidden.
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_CoachNonAuthorForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	// Athlete authored the note.
	note, _ := models.CreateAthleteNote(db, a.ID, nonCoach.ID, "2026-03-01", "Athlete note", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"Coach trying to edit athlete's note"},
	}
	// Coach tries to edit athlete's note — forbidden (author-only editing).
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_EmptyContent(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	note, _ := models.CreateAthleteNote(db, a.ID, coach.ID, "2026-03-01", "Original", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {""},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a.ID, note.ID), form, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_NoteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"test"},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/999", a.ID), form, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", "999")
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a1 := seedAthlete(t, db, "Kid 1", "")
	a2 := seedAthlete(t, db, "Kid 2", "")

	note, _ := models.CreateAthleteNote(db, a1.ID, coach.ID, "2026-03-01", "Belongs to a1", false, false)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"Updated"},
	}
	// Try to update a1's note via a2's URL.
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d", a2.ID, note.ID), form, coach)
	req.SetPathValue("id", itoa(a2.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_InvalidNoteID(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"test"},
	}
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/abc", a.ID), form, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", "abc")
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestJournal_UpdateNote_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}

	form := url.Values{
		"content": {"test"},
	}
	req := requestWithUser("POST", "/athletes/999/notes/1", form, coach)
	req.SetPathValue("id", "999")
	req.SetPathValue("noteID", "1")
	rr := httptest.NewRecorder()
	h.UpdateNote(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

// ---------------------------------------------------------------------------
// DeleteNote (POST)
// ---------------------------------------------------------------------------

func TestJournal_DeleteNote_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	note, _ := models.CreateAthleteNote(db, a.ID, coach.ID, "2026-03-01", "To be deleted", false, false)

	h := &Journal{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d/delete", a.ID, note.ID), nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify note was deleted.
	_, err := models.GetAthleteNoteByID(db, note.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestJournal_DeleteNote_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/999/delete", a.ID), nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", "999")
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestJournal_DeleteNote_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a1 := seedAthlete(t, db, "Kid 1", "")
	a2 := seedAthlete(t, db, "Kid 2", "")

	note, _ := models.CreateAthleteNote(db, a1.ID, coach.ID, "2026-03-01", "Belongs to a1", false, false)

	h := &Journal{DB: db, Templates: tc}

	// Try to delete a1's note via a2's URL.
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d/delete", a2.ID, note.ID), nil, coach)
	req.SetPathValue("id", itoa(a2.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_DeleteNote_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	note, _ := models.CreateAthleteNote(db, a.ID, coach.ID, "2026-03-01", "Coach note", false, false)

	h := &Journal{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/%d/delete", a.ID, note.ID), nil, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", itoa(note.ID))
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestJournal_DeleteNote_InvalidNoteID(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Kid", "")

	h := &Journal{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/notes/abc/delete", a.ID), nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("noteID", "abc")
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestJournal_DeleteNote_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Journal{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/999/notes/1/delete", nil, coach)
	req.SetPathValue("id", "999")
	req.SetPathValue("noteID", "1")
	rr := httptest.NewRecorder()
	h.DeleteNote(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}
