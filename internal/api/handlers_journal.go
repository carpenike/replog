package api

import (
	"log"
	"net/http"
	"strconv"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// --- Journal ---

// ListJournalEntries returns the athlete's journal timeline.
// ListJournalEntries returns the athlete's journal timeline.
//
//	@Summary      Athlete journal timeline
//	@Description  Combined timeline of workouts, notes, body weights, and goals. Coaches see private notes; non-coach linked athletes do not.
//	@Tags         Athletes
//	@Produce      json
//	@Param        id     path      int  true   "Athlete ID"
//	@Param        limit  query     int  false  "Page size (default 50)"
//	@Success      200  {array}   api.JournalEntry
//	@Failure      400  {object}  api.APIError
//	@Failure      403  {object}  api.APIError
//	@Router       /athletes/{id}/journal [get]
func (h *Handlers) ListJournalEntries(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	athleteID, ok := h.athleteAccess(w, r)
	if !ok {
		return
	}

	// Coaches and admins see private notes.
	includePrivate := user.IsCoach || user.IsAdmin

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		limit, _ = strconv.Atoi(l)
	}

	entries, err := models.ListJournalEntries(h.DB, athleteID, includePrivate, limit)
	if err != nil {
		log.Printf("api: list journal for athlete %d: %v", athleteID, err)
		WriteError(w, http.StatusInternalServerError, "failed to list journal")
		return
	}

	result := make([]*JournalEntry, len(entries))
	for i, e := range entries {
		result[i] = JournalEntryFromModel(e)
	}
	WriteJSON(w, http.StatusOK, result)
}
