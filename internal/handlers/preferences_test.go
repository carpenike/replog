package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestPreferences_EditForm_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Preferences{DB: db, Templates: tc}

	req := requestWithUser("GET", "/preferences", nil, coach)
	rr := httptest.NewRecorder()
	h.EditForm(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPreferences_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Preferences{DB: db, Templates: tc}

	form := url.Values{
		"weight_unit": {"kg"},
		"timezone":    {"America/Chicago"},
		"date_format":  {"02/01/2006"},
	}
	req := requestWithUser("POST", "/preferences", form, coach)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	// Verify preferences were saved.
	prefs, err := models.GetUserPreferences(db, coach.ID)
	if err != nil {
		t.Fatalf("get preferences: %v", err)
	}
	if prefs.WeightUnit != "kg" {
		t.Errorf("weight_unit = %q, want kg", prefs.WeightUnit)
	}
	if prefs.Timezone != "America/Chicago" {
		t.Errorf("timezone = %q, want America/Chicago", prefs.Timezone)
	}
	if prefs.DateFormat != "02/01/2006" {
		t.Errorf("date_format = %q, want 02/01/2006", prefs.DateFormat)
	}
}

func TestPreferences_Update_MissingFields(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Preferences{DB: db, Templates: tc}

	form := url.Values{
		"weight_unit": {"lb"},
		// timezone and date_format omitted
	}
	req := requestWithUser("POST", "/preferences", form, coach)
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}
