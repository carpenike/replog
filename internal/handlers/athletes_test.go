package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestAthletes_List_CoachSeesAll(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	seedAthlete(t, db, "Alice", "foundational")
	seedAthlete(t, db, "Bob", "")

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes", nil, coach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAthletes_List_NonCoachRedirectsToOwnProfile(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "foundational")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes", nil, nonCoach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
}

func TestAthletes_List_UnlinkedNonCoachRedirectsHome(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	unlinked := seedUnlinkedNonCoach(t, db)

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes", nil, unlinked)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/" {
		t.Errorf("expected redirect to /, got %q", loc)
	}
}

func TestAthletes_NewForm_CoachOnly(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)

	tests := []struct {
		name     string
		user     *models.User
		wantCode int
	}{
		{"coach sees form", seedCoach(t, db), http.StatusOK},
		{"non-coach forbidden", seedUnlinkedNonCoach(t, db), http.StatusForbidden},
	}

	h := &Athletes{DB: db, Templates: tc}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := requestWithUser("GET", "/athletes/new", nil, tt.user)
			rr := httptest.NewRecorder()
			h.NewForm(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rr.Code)
			}
		})
	}
}

func TestAthletes_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Athletes{DB: db, Templates: tc}

	form := url.Values{"name": {"Alice"}, "tier": {"foundational"}}
	req := requestWithUser("POST", "/athletes", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}
}

func TestAthletes_Create_EmptyName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Athletes{DB: db, Templates: tc}

	form := url.Values{"name": {""}}
	req := requestWithUser("POST", "/athletes", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestAthletes_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Athletes{DB: db, Templates: tc}

	form := url.Values{"name": {"Alice"}}
	req := requestWithUser("POST", "/athletes", form, nonCoach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAthletes_Show_CoachCanViewAny(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "foundational")

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID), nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAthletes_Show_NonCoachCanViewOwn(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "foundational")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID), nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestAthletes_Show_NonCoachCannotViewOther(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "Kid", "")
	otherAthlete := seedAthlete(t, db, "Other", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID), nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAthletes_Show_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("GET", "/athletes/999", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestAthletes_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "foundational")

	h := &Athletes{DB: db, Templates: tc}

	form := url.Values{"name": {"Alice Updated"}, "tier": {"intermediate"}}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID), form, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}

	// Verify update persisted.
	updated, err := models.GetAthleteByID(db, athlete.ID)
	if err != nil {
		t.Fatalf("get updated athlete: %v", err)
	}
	if updated.Name != "Alice Updated" {
		t.Errorf("expected name 'Alice Updated', got %q", updated.Name)
	}
}

func TestAthletes_Delete_CoachOnly(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "ToDelete", "")

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected redirect 303, got %d", rr.Code)
	}

	// Verify deletion.
	_, err := models.GetAthleteByID(db, athlete.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestAthletes_Delete_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Athletes{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/delete", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestAthletes_Promote_CoachSuccess(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Alice", "foundational")

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/promote", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Promote(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, err := models.GetAthleteByID(db, athlete.ID)
	if err != nil {
		t.Fatalf("get athlete: %v", err)
	}
	if !updated.Tier.Valid || updated.Tier.String != "intermediate" {
		t.Errorf("expected tier intermediate, got %v", updated.Tier)
	}
}

func TestAthletes_Promote_AlreadyHighestTier(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Bob", "sport_performance")

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/promote", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Promote(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestAthletes_Promote_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	athlete := seedAthlete(t, db, "Kid", "foundational")
	nonCoach := seedNonCoach(t, db, athlete.ID)

	h := &Athletes{DB: db, Templates: tc}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/promote", nil, nonCoach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()
	h.Promote(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}
