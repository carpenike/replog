package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestPrograms_List(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	models.CreateProgramTemplate(db, "5/3/1 BBB", "Boring But Big", 4, 4)
	models.CreateProgramTemplate(db, "GZCL", "", 3, 4)

	h := &Programs{DB: db, Templates: tc}
	req := requestWithUser("GET", "/programs", nil, coach)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPrograms_NewForm_CoachOnly(t *testing.T) {
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

	h := &Programs{DB: db, Templates: tc}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := requestWithUser("GET", "/programs/new", nil, tt.user)
			rr := httptest.NewRecorder()
			h.NewForm(rr, req)

			if rr.Code != tt.wantCode {
				t.Errorf("expected %d, got %d", tt.wantCode, rr.Code)
			}
		})
	}
}

func TestPrograms_Create_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{
		"name":      {"Test Program"},
		"num_weeks": {"4"},
		"num_days":  {"4"},
	}
	req := requestWithUser("POST", "/programs", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestPrograms_Create_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{"name": {"Test"}, "num_weeks": {"1"}, "num_days": {"1"}}
	req := requestWithUser("POST", "/programs", form, nonCoach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_Create_MissingName(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{"name": {""}, "num_weeks": {"4"}, "num_days": {"4"}}
	req := requestWithUser("POST", "/programs", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestPrograms_Create_DefaultsMinValues(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	// num_weeks and num_days are 0, should default to 1.
	form := url.Values{"name": {"Minimal"}, "num_weeks": {"0"}, "num_days": {"0"}}
	req := requestWithUser("POST", "/programs", form, coach)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	templates, _ := models.ListProgramTemplates(db)
	if len(templates) != 1 {
		t.Fatalf("templates = %d, want 1", len(templates))
	}
	if templates[0].NumWeeks != 1 || templates[0].NumDays != 1 {
		t.Errorf("weeks=%d days=%d, want 1/1", templates[0].NumWeeks, templates[0].NumDays)
	}
}

func TestPrograms_Show(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	tmpl, _ := models.CreateProgramTemplate(db, "Show Test", "", 4, 4)

	h := &Programs{DB: db, Templates: tc}
	req := requestWithUser("GET", "/programs/"+itoa(tmpl.ID), nil, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestPrograms_Show_NotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Programs{DB: db, Templates: tc}
	req := requestWithUser("GET", "/programs/999", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Show(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestPrograms_EditForm_CoachOnly(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	nonCoach := seedUnlinkedNonCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Edit Test", "", 4, 4)

	h := &Programs{DB: db, Templates: tc}

	t.Run("coach sees form", func(t *testing.T) {
		req := requestWithUser("GET", "/programs/"+itoa(tmpl.ID)+"/edit", nil, coach)
		req.SetPathValue("id", itoa(tmpl.ID))
		rr := httptest.NewRecorder()
		h.EditForm(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("non-coach forbidden", func(t *testing.T) {
		req := requestWithUser("GET", "/programs/"+itoa(tmpl.ID)+"/edit", nil, nonCoach)
		req.SetPathValue("id", itoa(tmpl.ID))
		rr := httptest.NewRecorder()
		h.EditForm(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}

func TestPrograms_Update_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Old Name", "", 4, 4)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{
		"name":      {"New Name"},
		"num_weeks": {"3"},
		"num_days":  {"3"},
	}
	req := requestWithUser("POST", "/programs/"+itoa(tmpl.ID), form, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	updated, _ := models.GetProgramTemplateByID(db, tmpl.ID)
	if updated.Name != "New Name" {
		t.Errorf("name = %q, want New Name", updated.Name)
	}
}

func TestPrograms_Update_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{"name": {"X"}, "num_weeks": {"1"}, "num_days": {"1"}}
	req := requestWithUser("POST", "/programs/1", form, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_Delete_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Delete Me", "", 1, 1)

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/programs/"+itoa(tmpl.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
	if loc := rr.Header().Get("Location"); loc != "/programs" {
		t.Errorf("redirect to %q, want /programs", loc)
	}
}

func TestPrograms_Delete_InUseConflict(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "In Use", "", 1, 1)
	a := seedAthlete(t, db, "Athlete", "")
	models.AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "")

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/programs/"+itoa(tmpl.ID)+"/delete", nil, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestPrograms_Delete_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/programs/1/delete", nil, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_AddSet_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "AddSet Test", "", 4, 4)
	ex := seedExercise(t, db, "Bench Press", "", 0)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{
		"exercise_id": {itoa(ex.ID)},
		"week":        {"1"},
		"day":         {"1"},
		"set_number":  {"1"},
		"reps":        {"5"},
		"percentage":  {"75"},
	}
	req := requestWithUser("POST", "/programs/"+itoa(tmpl.ID)+"/sets", form, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	sets, _ := models.ListPrescribedSets(db, tmpl.ID)
	if len(sets) != 1 {
		t.Errorf("sets = %d, want 1", len(sets))
	}
}

func TestPrograms_AddSet_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{"exercise_id": {"1"}, "week": {"1"}, "day": {"1"}, "set_number": {"1"}}
	req := requestWithUser("POST", "/programs/1/sets", form, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.AddSet(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_AddSet_MissingFields(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Missing Fields", "", 4, 4)

	h := &Programs{DB: db, Templates: tc}

	tests := []struct {
		name string
		form url.Values
	}{
		{"missing exercise", url.Values{"week": {"1"}, "day": {"1"}, "set_number": {"1"}}},
		{"missing week", url.Values{"exercise_id": {"1"}, "day": {"1"}, "set_number": {"1"}}},
		{"invalid week", url.Values{"exercise_id": {"1"}, "week": {"0"}, "day": {"1"}, "set_number": {"1"}}},
		{"missing day", url.Values{"exercise_id": {"1"}, "week": {"1"}, "set_number": {"1"}}},
		{"invalid day", url.Values{"exercise_id": {"1"}, "week": {"1"}, "day": {"0"}, "set_number": {"1"}}},
		{"invalid set_number", url.Values{"exercise_id": {"1"}, "week": {"1"}, "day": {"1"}, "set_number": {"0"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := requestWithUser("POST", "/programs/"+itoa(tmpl.ID)+"/sets", tt.form, coach)
			req.SetPathValue("id", itoa(tmpl.ID))
			rr := httptest.NewRecorder()
			h.AddSet(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d", rr.Code)
			}
		})
	}
}

func TestPrograms_DeleteSet_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "DeleteSet Test", "", 4, 4)
	ex := seedExercise(t, db, "Bench", "", 0)

	reps := 5
	ps, _ := models.CreatePrescribedSet(db, tmpl.ID, ex.ID, 1, 1, 1, &reps, nil, "")

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", fmt.Sprintf("/programs/%d/sets/%d/delete", tmpl.ID, ps.ID), nil, coach)
	req.SetPathValue("id", itoa(tmpl.ID))
	req.SetPathValue("setID", itoa(ps.ID))
	rr := httptest.NewRecorder()
	h.DeleteSet(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	sets, _ := models.ListPrescribedSets(db, tmpl.ID)
	if len(sets) != 0 {
		t.Errorf("sets after delete = %d, want 0", len(sets))
	}
}

func TestPrograms_DeleteSet_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/programs/1/sets/1/delete", nil, nonCoach)
	req.SetPathValue("id", "1")
	req.SetPathValue("setID", "1")
	rr := httptest.NewRecorder()
	h.DeleteSet(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_AssignProgram_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Assign Test", "", 4, 4)
	a := seedAthlete(t, db, "Athlete", "")

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{
		"template_id": {itoa(tmpl.ID)},
		"start_date":  {"2026-02-01"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/program", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.AssignProgram(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	ap, _ := models.GetActiveProgram(db, a.ID)
	if ap == nil {
		t.Fatal("expected active program")
	}
	if ap.TemplateName != "Assign Test" {
		t.Errorf("template = %q, want Assign Test", ap.TemplateName)
	}
}

func TestPrograms_AssignProgram_AlreadyActive(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Conflict Test", "", 4, 4)
	a := seedAthlete(t, db, "Athlete", "")
	models.AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "")

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{
		"template_id": {itoa(tmpl.ID)},
		"start_date":  {"2026-02-15"},
	}
	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/program", form, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.AssignProgram(rr, req)

	if rr.Code != http.StatusConflict {
		t.Errorf("expected 409, got %d", rr.Code)
	}
}

func TestPrograms_AssignProgram_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	form := url.Values{"template_id": {"1"}}
	req := requestWithUser("POST", "/athletes/1/program", form, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.AssignProgram(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_DeactivateProgram_Success(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	tmpl, _ := models.CreateProgramTemplate(db, "Deactivate Test", "", 4, 4)
	a := seedAthlete(t, db, "Athlete", "")
	models.AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "")

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/program/deactivate", nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.DeactivateProgram(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}

	ap, _ := models.GetActiveProgram(db, a.ID)
	if ap != nil {
		t.Error("expected nil active program after deactivation")
	}
}

func TestPrograms_DeactivateProgram_NoActiveProgram(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/"+itoa(a.ID)+"/program/deactivate", nil, coach)
	req.SetPathValue("id", itoa(a.ID))
	rr := httptest.NewRecorder()
	h.DeactivateProgram(rr, req)

	// Should redirect gracefully, not error.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303, got %d", rr.Code)
	}
}

func TestPrograms_DeactivateProgram_NonCoachForbidden(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	nonCoach := seedUnlinkedNonCoach(t, db)

	h := &Programs{DB: db, Templates: tc}

	req := requestWithUser("POST", "/athletes/1/program/deactivate", nil, nonCoach)
	req.SetPathValue("id", "1")
	rr := httptest.NewRecorder()
	h.DeactivateProgram(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", rr.Code)
	}
}

func TestPrograms_Prescription(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")

	t.Run("no program", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/prescription", nil, coach)
		req.SetPathValue("id", itoa(a.ID))
		rr := httptest.NewRecorder()
		h := &Programs{DB: db, Templates: tc}
		h.Prescription(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("with program", func(t *testing.T) {
		tmpl, _ := models.CreateProgramTemplate(db, "Rx Test", "", 4, 4)
		models.AssignProgram(db, a.ID, tmpl.ID, "2026-02-01", "")

		req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/prescription", nil, coach)
		req.SetPathValue("id", itoa(a.ID))
		rr := httptest.NewRecorder()
		h := &Programs{DB: db, Templates: tc}
		h.Prescription(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestPrograms_Prescription_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Programs{DB: db, Templates: tc}
	req := requestWithUser("GET", "/athletes/999/prescription", nil, coach)
	req.SetPathValue("id", "999")
	rr := httptest.NewRecorder()
	h.Prescription(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestPrograms_AssignProgramForm(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)
	a := seedAthlete(t, db, "Athlete", "")
	models.CreateProgramTemplate(db, "Template A", "", 4, 4)

	h := &Programs{DB: db, Templates: tc}

	t.Run("coach sees form", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/program/assign", nil, coach)
		req.SetPathValue("id", itoa(a.ID))
		rr := httptest.NewRecorder()
		h.AssignProgramForm(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("non-coach forbidden", func(t *testing.T) {
		nonCoach := seedUnlinkedNonCoach(t, db)
		req := requestWithUser("GET", "/athletes/"+itoa(a.ID)+"/program/assign", nil, nonCoach)
		req.SetPathValue("id", itoa(a.ID))
		rr := httptest.NewRecorder()
		h.AssignProgramForm(rr, req)

		if rr.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rr.Code)
		}
	})
}
