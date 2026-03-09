package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// UpdateAthleteNote updates an existing journal note.
func (h *Handlers) UpdateAthleteNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("noteID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid note ID")
		return
	}

	var req struct {
		Content   string `json:"content"`
		IsPrivate bool   `json:"is_private"`
		Pinned    bool   `json:"pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Content == "" {
		WriteError(w, http.StatusBadRequest, "content is required")
		return
	}

	note, err := models.UpdateAthleteNote(h.DB, noteID, req.Content, req.IsPrivate, req.Pinned)
	if err != nil {
		log.Printf("api: update note %d: %v", noteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to update note")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]any{
		"id":         note.ID,
		"content":    note.Content,
		"is_private": note.IsPrivate,
		"pinned":     note.Pinned,
	})
}

// DeleteAthleteNote deletes a journal note.
func (h *Handlers) DeleteAthleteNote(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid athlete ID")
		return
	}
	if !middleware.CanAccessAthlete(h.DB, user, athleteID) {
		WriteError(w, http.StatusForbidden, "access denied")
		return
	}

	noteID, err := strconv.ParseInt(r.PathValue("noteID"), 10, 64)
	if err != nil {
		WriteError(w, http.StatusBadRequest, "invalid note ID")
		return
	}

	if err := models.DeleteAthleteNote(h.DB, noteID); err != nil {
		log.Printf("api: delete note %d: %v", noteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to delete note")
		return
	}

	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
