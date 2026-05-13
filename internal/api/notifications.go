package api

import (
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/carpenike/replog/internal/models"
	"github.com/carpenike/replog/internal/notify"
)

// notifyAthlete dispatches a notification to the user linked to the given
// athlete. It is a thin wrapper around notify.Send that handles the
// athlete-id-to-user-id lookup and the "no linked user" no-op so handlers
// stay one line.
//
// athleteID may refer to an athlete with no linked user (e.g. young kids
// whose parent logs sets on their behalf). In that case the function is a
// no-op — there is nobody to notify.
//
// Errors are logged but never propagated. Notifications must never block the
// triggering action.
func (h *Handlers) notifyAthlete(athleteID int64, nType, title, message, link string) {
	user, err := models.GetUserByAthleteID(h.DB, athleteID)
	if err != nil {
		if !errors.Is(err, models.ErrNotFound) {
			log.Printf("api: notify athlete %d: lookup user: %v", athleteID, err)
		}
		return
	}
	notify.Send(h.DB, notify.Request{
		UserID:    user.ID,
		Type:      nType,
		Title:     title,
		Message:   message,
		Link:      link,
		AthleteID: sql.NullInt64{Int64: athleteID, Valid: true},
	})
}

// notifyCoach dispatches a notification to the coach who owns the given
// athlete. No-op if the athlete has no coach assigned.
func (h *Handlers) notifyCoach(athleteID int64, nType, title, message, link string) {
	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		log.Printf("api: notify coach for athlete %d: load athlete: %v", athleteID, err)
		return
	}
	if !athlete.CoachID.Valid {
		return
	}
	notify.Send(h.DB, notify.Request{
		UserID:    athlete.CoachID.Int64,
		Type:      nType,
		Title:     title,
		Message:   message,
		Link:      link,
		AthleteID: sql.NullInt64{Int64: athleteID, Valid: true},
	})
}

// athleteDisplayName returns the athlete's name for use in notification
// titles. Falls back to "athlete" if the lookup fails — the notification
// itself is more important than a perfect title.
func (h *Handlers) athleteDisplayName(athleteID int64) string {
	athlete, err := models.GetAthleteByID(h.DB, athleteID)
	if err != nil {
		return "athlete"
	}
	if athlete.Name == "" {
		return fmt.Sprintf("athlete #%d", athleteID)
	}
	return athlete.Name
}
