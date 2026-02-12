package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPages_Index_CoachSeesDashboard(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	p := &Pages{DB: db, Templates: tc}

	req := requestWithUser("GET", "/", nil, coach)
	rr := httptest.NewRecorder()
	p.Index(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPages_Index_NonCoachLinkedRedirects(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	p := &Pages{DB: db, Templates: tc}

	req := requestWithUser("GET", "/", nil, nonCoach)
	rr := httptest.NewRecorder()
	p.Index(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	expected := "/athletes/" + itoa(athlete.ID)
	if loc != expected {
		t.Errorf("expected redirect to %s, got %s", expected, loc)
	}
}

func TestPages_Index_UnlinkedNonCoachSeesMessage(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	unlinked := seedUnlinkedNonCoach(t, db)

	p := &Pages{DB: db, Templates: tc}

	req := requestWithUser("GET", "/", nil, unlinked)
	rr := httptest.NewRecorder()
	p.Index(rr, req)

	// Unlinked non-coach should see the index page (200) with an informative message.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}
