package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestGenerate_Form(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	t.Run("renders form", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})

	t.Run("shows not configured", func(t *testing.T) {
		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		// Response should NOT contain "configured" since no provider is set.
		body := rr.Body.String()
		if contains(body, "configured") {
			t.Errorf("expected form to show not-configured state")
		}
	})

	t.Run("shows configured when provider set", func(t *testing.T) {
		models.SetSetting(db, "llm.provider", "openai")
		defer models.DeleteSetting(db, "llm.provider")

		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !contains(body, "configured") {
			t.Errorf("expected form to show configured state")
		}
	})

	t.Run("pre-fills from active program", func(t *testing.T) {
		// Create a program template and assign it.
		pt, err := models.CreateProgramTemplate(db, nil, "Sport Performance Month 3", "test", 4, 4, false, "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = models.AssignProgram(db, athlete.ID, pt.ID, "2026-01-15", "", "")
		if err != nil {
			t.Fatal(err)
		}

		req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate", nil, coach)
		req.SetPathValue("id", itoa(athlete.ID))
		rr := httptest.NewRecorder()

		h.Form(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rr.Code)
		}
	})
}

func TestGenerate_Form_AthleteNotFound(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	req := requestWithUser("GET", "/athletes/99999/programs/generate", nil, coach)
	req.SetPathValue("id", "99999")
	rr := httptest.NewRecorder()

	h.Form(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestGenerate_Submit_NotConfigured(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	body := url.Values{
		"program_name": {"Test Program"},
		"num_days":     {"3"},
		"num_weeks":    {"4"},
	}

	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/programs/generate", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Submit)).ServeHTTP(rr, req)

	// Without a configured provider, should get 200 with error message on form.
	if rr.Code != http.StatusOK {
		t.Errorf("expected 200 (re-rendered form with error), got %d", rr.Code)
	}
	if !contains(rr.Body.String(), "not configured") {
		t.Errorf("expected error about not configured, got: %s", rr.Body.String())
	}
}

func TestGenerate_Preview_NoSession(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	req := requestWithUser("GET", "/athletes/"+itoa(athlete.ID)+"/programs/generate/preview", nil, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Preview)).ServeHTTP(rr, req)

	// Without session data, should redirect to form.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
	loc := rr.Header().Get("Location")
	expected := fmt.Sprintf("/athletes/%d/programs/generate", athlete.ID)
	if loc != expected {
		t.Errorf("expected redirect to %s, got %s", expected, loc)
	}
}

func TestGenerate_Execute_NoSession(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	sm := testSessionManager()
	coach := seedCoach(t, db)
	athlete := seedAthlete(t, db, "Tommy", "sport_performance")

	h := &Generate{DB: db, Sessions: sm, Templates: tc}

	body := url.Values{}
	req := requestWithUser("POST", "/athletes/"+itoa(athlete.ID)+"/programs/generate/execute", body, coach)
	req.SetPathValue("id", itoa(athlete.ID))
	rr := httptest.NewRecorder()

	sm.LoadAndSave(http.HandlerFunc(h.Execute)).ServeHTTP(rr, req)

	// Without session data, should redirect to form.
	if rr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect, got %d", rr.Code)
	}
}

func TestSuggestNextProgramName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Sport Performance Month 3", "Sport Performance Month 4"},
		{"5/3/1 Cycle 1", "5/3/1 Cycle 2"},
		{"GZCL", "GZCL 2"},
		{"My Program 10", "My Program 11"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := suggestNextProgramName(tt.input)
			if got != tt.want {
				t.Errorf("suggestNextProgramName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// contains is a helper for checking substrings.
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
