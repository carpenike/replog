package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestBodyWeights_List_CoachAccess(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	models.CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")

	h := &BodyWeights{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/body-weights", nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestBodyWeights_List_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "foundational")
	nonCoach := seedNonCoach(t, db, a.ID)

	h := &BodyWeights{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/body-weights", nil, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestBodyWeights_List_NonCoachOtherAthleteForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "My Kid", "")
	otherAthlete := seedAthlete(t, db, "Other Kid", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &BodyWeights{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/"+itoa(otherAthlete.ID)+"/body-weights", nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestBodyWeights_List_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &BodyWeights{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/999/body-weights", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestBodyWeights_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	h := &BodyWeights{DB: db, Templates: tc}

	form := url.Values{
		"date":   {"2026-02-01"},
		"weight": {"185.5"},
		"notes":  {"morning"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/body-weights", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	bw, _ := models.LatestBodyWeight(db, a.ID)
	if bw == nil {
		t.Fatal("expected body weight record")
	}
	if bw.Weight != 185.5 {
		t.Errorf("weight = %.1f, want 185.5", bw.Weight)
	}
}

func TestBodyWeights_Create_InvalidWeight(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	h := &BodyWeights{DB: db, Templates: tc}

	tests := []struct {
		name   string
		weight string
	}{
		{"empty", ""},
		{"zero", "0"},
		{"negative", "-5"},
		{"text", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := url.Values{"date": {"2026-02-01"}, "weight": {tt.weight}}
			req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/body-weights", form, coach)
			req.SetPathValue("id", itoa(a.ID))
			rr := httptest.NewRecorder()
			h.Create(rr, req)

			if rr.Code != http.StatusUnprocessableEntity {
				t.Errorf("expected 422, got %d", rr.Code)
			}
		})
	}
}

func TestBodyWeights_Create_DuplicateDate(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	models.CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")

	h := &BodyWeights{DB: db, Templates: tc}

	form := url.Values{"date": {"2026-02-01"}, "weight": {"186.0"}}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/body-weights", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", rr.Code)
	}
}

func TestBodyWeights_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "My Kid", "")
	otherAthlete := seedAthlete(t, db, "Other Kid", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)

	h := &BodyWeights{DB: db, Templates: tc}

	form := url.Values{"date": {"2026-02-01"}, "weight": {"150"}}
	req := requestWithUser("POST", "/athletes/"+itoa(otherAthlete.ID)+"/body-weights", form, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestBodyWeights_Create_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)

	h := &BodyWeights{DB: db, Templates: tc}

	form := url.Values{"date": {"2026-02-01"}, "weight": {"120.5"}}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/body-weights", form, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestBodyWeights_Delete_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")
	bw, _ := models.CreateBodyWeight(db, a.ID, "2026-02-01", 185.0, "")

	h := &BodyWeights{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/body-weights/%d/delete", a.ID, bw.ID), nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("bwID", itoa(bw.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	_, err := models.GetBodyWeightByID(db, bw.ID)
	if err != models.ErrNotFound {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestBodyWeights_Delete_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	h := &BodyWeights{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/body-weights/999/delete", a.ID), nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("bwID", "999")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestBodyWeights_Delete_WrongAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a1 := seedAthlete(t, db, "Athlete 1", "")
	a2 := seedAthlete(t, db, "Athlete 2", "")
	bw, _ := models.CreateBodyWeight(db, a1.ID, "2026-02-01", 185.0, "")

	h := &BodyWeights{DB: db, Templates: tc}

	// Try to delete a1's entry via a2's URL.
	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/body-weights/%d/delete", a2.ID, bw.ID), nil, coach)
	req.SetPathValue("id", itoa(a2.ID))
	req.SetPathValue("bwID", itoa(bw.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 for wrong athlete, got %d", rr.Code)
	}
}

func TestBodyWeights_Delete_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	myAthlete := seedAthlete(t, db, "My Kid", "")
	otherAthlete := seedAthlete(t, db, "Other Kid", "")
	nonCoach := seedNonCoach(t, db, myAthlete.ID)
	bw, _ := models.CreateBodyWeight(db, otherAthlete.ID, "2026-02-01", 150.0, "")

	h := &BodyWeights{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/body-weights/%d/delete", otherAthlete.ID, bw.ID), nil, nonCoach)
	req.SetPathValue("id", itoa(otherAthlete.ID))
	req.SetPathValue("bwID", itoa(bw.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestBodyWeights_Delete_NonCoachOwnAthlete(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	a := seedAthlete(t, db, "Kid", "")
	nonCoach := seedNonCoach(t, db, a.ID)
	bw, _ := models.CreateBodyWeight(db, a.ID, "2026-02-01", 120.0, "")

	h := &BodyWeights{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/athletes/%d/body-weights/%d/delete", a.ID, bw.ID), nil, nonCoach)
	req.SetPathValue("id", itoa(a.ID))
	req.SetPathValue("bwID", itoa(bw.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}
