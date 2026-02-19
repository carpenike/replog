package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"database/sql"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// Journal holds dependencies for the journal timeline and athlete notes handlers.
type Journal struct {
	DB        *sql.DB
	Templates TemplateCache
}

// Timeline renders the unified journal view for an athlete.
func (h *Journal) Timeline(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		h.Templates.Forbidden(w, r)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for journal: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	canManage := middleware.CanManageAthlete(user, athlete)
	entries, err := models.ListJournalEntries(h.DB, athleteID, canManage, 200)
	if err != nil {
		log.Printf("handlers: list journal entries for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Athlete":    athlete,
		"Entries":    entries,
		"CanManage":  canManage,
		"Today":      time.Now().Format("2006-01-02"),
	}

	if err := h.Templates.Render(w, r, "journal.html", data); err != nil {
		log.Printf("handlers: journal template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// CreateNote handles new athlete note submission (coach/admin only).
func (h *Journal) CreateNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for create note: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	content := r.FormValue("content")
	if content == "" {
		http.Error(w, "Note content is required", http.StatusUnprocessableEntity)
		return
	}

	date := r.FormValue("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}

	isPrivate := r.FormValue("is_private") == "1"
	pinned := r.FormValue("pinned") == "1"

	_, err = models.CreateAthleteNote(h.DB, athleteID, user.ID, date, content, isPrivate, pinned)
	if err != nil {
		log.Printf("handlers: create note for athlete %d: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/journal", http.StatusSeeOther)
}

// DeleteNote removes an athlete note (coach/admin only).
func (h *Journal) DeleteNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())

	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid athlete ID", http.StatusBadRequest)
		return
	}

	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Athlete not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get athlete %d for delete note: %v", athleteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if !middleware.CanManageAthlete(user, athlete) {
		h.Templates.Forbidden(w, r)
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("noteID"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid note ID", http.StatusBadRequest)
		return
	}

	// Verify the note belongs to this athlete.
	note, err := models.GetAthleteNoteByID(h.DB, noteID)
	if errors.Is(err, models.ErrNotFound) {
		http.Error(w, "Note not found", http.StatusNotFound)
		return
	}
	if err != nil {
		log.Printf("handlers: get note %d: %v", noteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if note.AthleteID != athleteID {
		http.Error(w, "Note does not belong to this athlete", http.StatusForbidden)
		return
	}

	if err := models.DeleteAthleteNote(h.DB, noteID); err != nil {
		log.Printf("handlers: delete note %d: %v", noteID, err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/athletes/"+strconv.FormatInt(athleteID, 10)+"/journal", http.StatusSeeOther)
}
