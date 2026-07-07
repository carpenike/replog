package api

import (
	"net/http"
	"time"

	"github.com/carpenike/replog/internal/middleware"
	"github.com/carpenike/replog/internal/models"
)

// dateLayout is the wire format for all DATE fields (YYYY-MM-DD).
const dateLayout = "2006-01-02"

// userLocation resolves the *time.Location for the authenticated user's
// preference timezone, falling back to the app default when preferences are
// missing or the zone name is invalid. Used so "today" means today in the
// user's timezone, not the server's — the same posture GetPrescription uses.
func userLocation(r *http.Request) *time.Location {
	tz := models.DefaultTimezone
	if prefs := middleware.PrefsFromContext(r.Context()); prefs != nil && prefs.Timezone != "" {
		tz = prefs.Timezone
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

// todayInUserTZ returns today's date (YYYY-MM-DD) in the user's timezone.
func todayInUserTZ(r *http.Request) string {
	return time.Now().In(userLocation(r)).Format(dateLayout)
}

// validDate reports whether s is a well-formed YYYY-MM-DD calendar date.
// SQLite stores dates as text and will not reject "2026-13-99", so callers
// validate at the boundary and return 400 on garbage.
func validDate(s string) bool {
	_, err := time.ParseInLocation(dateLayout, s, time.UTC)
	return err == nil
}
