package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

func TestSettingsShow(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Settings{DB: db, Templates: tc}

	r := requestWithUser("GET", "/admin/settings", nil, coach)
	w := httptest.NewRecorder()
	h.Show(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Settings") {
		t.Error("expected 'Settings' in response body")
	}
	if !strings.Contains(body, "AI Coach") {
		t.Error("expected 'AI Coach' category in response body")
	}
}

func TestSettingsUpdate(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Settings{DB: db, Templates: tc}

	form := url.Values{
		"setting_llm.provider": {"ollama"},
		"setting_llm.model":    {"llama3"},
		"setting_llm.base_url": {"http://localhost:11434"},
	}

	r := requestWithUser("POST", "/admin/settings", form, coach)
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// Verify settings were persisted.
	if got := models.GetSetting(db, "llm.provider"); got != "ollama" {
		t.Errorf("expected provider 'ollama', got %q", got)
	}
	if got := models.GetSetting(db, "llm.model"); got != "llama3" {
		t.Errorf("expected model 'llama3', got %q", got)
	}
	if got := models.GetSetting(db, "llm.base_url"); got != "http://localhost:11434" {
		t.Errorf("expected base_url 'http://localhost:11434', got %q", got)
	}
}

func TestSettingsUpdateClear(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	h := &Settings{DB: db, Templates: tc}

	// Set a value first.
	if err := models.SetSetting(db, "llm.provider", "ollama"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Submit empty form to clear it.
	form := url.Values{
		"setting_llm.provider": {""},
	}
	r := requestWithUser("POST", "/admin/settings", form, coach)
	w := httptest.NewRecorder()
	h.Update(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	if got := models.GetSetting(db, "llm.provider"); got != "" {
		t.Errorf("expected empty provider after clear, got %q", got)
	}
}

func TestSettingsMaskedNotOverwritten(t *testing.T) {
	db := testDB(t)
	tc := testTemplateCache(t)
	coach := seedCoach(t, db)

	t.Setenv("REPLOG_SECRET_KEY", "test-secret-key-for-unit-tests!")

	h := &Settings{DB: db, Templates: tc}

	// Set an API key.
	if err := models.SetSetting(db, "llm.api_key", "sk-real-key-12345"); err != nil {
		t.Fatalf("set: %v", err)
	}

	// Submit the masked value (simulating what the browser sends back).
	form := url.Values{
		"setting_llm.api_key": {"sk-r••••2345"},
	}
	r := requestWithUser("POST", "/admin/settings", form, coach)
	w := httptest.NewRecorder()
	h.Update(w, r)

	// The original value should be preserved.
	if got := models.GetSetting(db, "llm.api_key"); got != "sk-real-key-12345" {
		t.Errorf("expected original API key preserved, got %q", got)
	}
}
