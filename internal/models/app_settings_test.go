package models

import (
	"os"
	"testing"
)

func TestGetSetting_EnvOverride(t *testing.T) {
	db := testDB(t)

	// Set a DB value.
	if err := SetSetting(db, "llm.provider", "ollama"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	// Without env var, should return DB value.
	got := GetSetting(db, "llm.provider")
	if got != "ollama" {
		t.Errorf("expected 'ollama' from DB, got %q", got)
	}

	// With env var, env should win.
	t.Setenv("REPLOG_LLM_PROVIDER", "openai")
	got = GetSetting(db, "llm.provider")
	if got != "openai" {
		t.Errorf("expected 'openai' from env, got %q", got)
	}
}

func TestGetSetting_Default(t *testing.T) {
	db := testDB(t)

	got := GetSetting(db, "llm.temperature")
	if got != "0.7" {
		t.Errorf("expected default '0.7', got %q", got)
	}
}

func TestGetSetting_UnknownKey(t *testing.T) {
	db := testDB(t)

	got := GetSetting(db, "nonexistent.key")
	if got != "" {
		t.Errorf("expected empty string for unknown key, got %q", got)
	}
}

func TestSetSetting_CreateAndUpdate(t *testing.T) {
	db := testDB(t)

	// Create.
	if err := SetSetting(db, "llm.model", "gpt-4o"); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	got := GetSetting(db, "llm.model")
	if got != "gpt-4o" {
		t.Errorf("expected 'gpt-4o', got %q", got)
	}

	// Update (upsert).
	if err := SetSetting(db, "llm.model", "claude-sonnet-4-20250514"); err != nil {
		t.Fatalf("update setting: %v", err)
	}
	got = GetSetting(db, "llm.model")
	if got != "claude-sonnet-4-20250514" {
		t.Errorf("expected 'claude-sonnet-4-20250514', got %q", got)
	}
}

func TestSetSetting_UnknownKey(t *testing.T) {
	db := testDB(t)
	err := SetSetting(db, "fake.key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestDeleteSetting(t *testing.T) {
	db := testDB(t)

	if err := SetSetting(db, "llm.base_url", "http://localhost:11434"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := DeleteSetting(db, "llm.base_url"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Should fall back to default (empty string).
	got := GetSetting(db, "llm.base_url")
	if got != "" {
		t.Errorf("expected empty default after delete, got %q", got)
	}
}

func TestListSettings(t *testing.T) {
	db := testDB(t)

	settings := ListSettings(db)
	if len(settings) != len(SettingsRegistry) {
		t.Errorf("expected %d settings, got %d", len(SettingsRegistry), len(settings))
	}

	// All should have source "default" in a fresh DB with no env vars.
	for _, sv := range settings {
		// Skip if this env var happens to be set in the test environment.
		def := findDefinition(sv.Key)
		if def != nil && def.EnvVar != "" && os.Getenv(def.EnvVar) != "" {
			continue
		}
		if sv.Source != "default" {
			t.Errorf("setting %q: expected source 'default', got %q", sv.Key, sv.Source)
		}
	}
}

func TestListSettingsByCategory(t *testing.T) {
	db := testDB(t)

	groups := ListSettingsByCategory(db)
	aiSettings, ok := groups["AI Coach"]
	if !ok {
		t.Fatal("expected 'AI Coach' category in settings")
	}
	if len(aiSettings) < 3 {
		t.Errorf("expected at least 3 AI Coach settings, got %d", len(aiSettings))
	}

	// Verify new categories exist.
	if _, ok := groups["General"]; !ok {
		t.Error("expected 'General' category in settings")
	}
	if _, ok := groups["Defaults"]; !ok {
		t.Error("expected 'Defaults' category in settings")
	}
}

func TestSensitiveEncryption(t *testing.T) {
	db := testDB(t)

	// Set a secret key for encryption.
	t.Setenv("REPLOG_SECRET_KEY", "test-secret-key-for-unit-tests!")

	// Store an API key (sensitive field).
	if err := SetSetting(db, "llm.api_key", "sk-test-12345"); err != nil {
		t.Fatalf("set sensitive setting: %v", err)
	}

	// Verify it's stored encrypted in DB.
	var raw string
	db.QueryRow(`SELECT value FROM app_settings WHERE key = 'llm.api_key'`).Scan(&raw)
	if !hasPrefix(raw, "enc:") {
		t.Errorf("expected encrypted value with 'enc:' prefix, got %q", raw)
	}

	// Reading back should decrypt.
	got := GetSetting(db, "llm.api_key")
	if got != "sk-test-12345" {
		t.Errorf("expected decrypted 'sk-test-12345', got %q", got)
	}
}

func TestSensitiveWithoutSecretKey(t *testing.T) {
	db := testDB(t)

	// Ensure no secret key is set.
	t.Setenv("REPLOG_SECRET_KEY", "")

	err := SetSetting(db, "llm.api_key", "sk-test-12345")
	if err == nil {
		t.Fatal("expected error when setting sensitive value without REPLOG_SECRET_KEY")
	}
}

func TestIsAICoachConfigured(t *testing.T) {
	db := testDB(t)

	if IsAICoachConfigured(db) {
		t.Error("expected AI Coach not configured in fresh DB")
	}

	if err := SetSetting(db, "llm.provider", "ollama"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if !IsAICoachConfigured(db) {
		t.Error("expected AI Coach configured after setting provider")
	}
}

func TestMaskValue(t *testing.T) {
	tests := []struct {
		value    string
		expected string
	}{
		{"sk-ant-api03-very-long-key-1234", "sk-a••••1234"},
		{"short", "••••••••"},
		{"", ""},
	}

	for _, tt := range tests {
		got := maskValue(tt.value, true)
		if got != tt.expected {
			t.Errorf("maskValue(%q, true) = %q, want %q", tt.value, got, tt.expected)
		}
	}

	// Non-sensitive should not mask.
	got := maskValue("sk-ant-api03-very-long-key-1234", false)
	if got != "sk-ant-api03-very-long-key-1234" {
		t.Errorf("maskValue with sensitive=false should not mask, got %q", got)
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func TestGetDefaultWeightUnit(t *testing.T) {
	db := testDB(t)

	// Default (no setting configured).
	if got := GetDefaultWeightUnit(db); got != "lbs" {
		t.Errorf("expected default 'lbs', got %q", got)
	}

	// Override via app_settings.
	if err := SetSetting(db, "defaults.weight_unit", "kg"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultWeightUnit(db); got != "kg" {
		t.Errorf("expected 'kg' after override, got %q", got)
	}
}

func TestGetDefaultTimezone(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultTimezone(db); got != "America/New_York" {
		t.Errorf("expected default 'America/New_York', got %q", got)
	}

	if err := SetSetting(db, "defaults.timezone", "Europe/London"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultTimezone(db); got != "Europe/London" {
		t.Errorf("expected 'Europe/London', got %q", got)
	}
}

func TestGetDefaultDateFormat(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultDateFormat(db); got != "Jan 2, 2006" {
		t.Errorf("expected default 'Jan 2, 2006', got %q", got)
	}

	if err := SetSetting(db, "defaults.date_format", "2006-01-02"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultDateFormat(db); got != "2006-01-02" {
		t.Errorf("expected '2006-01-02', got %q", got)
	}
}

func TestGetDefaultRestSeconds(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultRestSeconds(db); got != 90 {
		t.Errorf("expected default 90, got %d", got)
	}

	if err := SetSetting(db, "defaults.rest_seconds", "120"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultRestSeconds(db); got != 120 {
		t.Errorf("expected 120, got %d", got)
	}
}

func TestGetAppName(t *testing.T) {
	db := testDB(t)

	if got := GetAppName(db); got != "RepLog" {
		t.Errorf("expected default 'RepLog', got %q", got)
	}

	if err := SetSetting(db, "app.name", "Smith Family Gym"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetAppName(db); got != "Smith Family Gym" {
		t.Errorf("expected 'Smith Family Gym', got %q", got)
	}
}

func TestGetOrCreateSecretKey(t *testing.T) {
	db := testDB(t)

	// Clear any env var.
	t.Setenv("REPLOG_SECRET_KEY", "")

	// First call should generate and store a key.
	key1, source, err := GetOrCreateSecretKey(db)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if key1 == "" {
		t.Fatal("expected non-empty key")
	}
	if source != "generated" {
		t.Errorf("expected source 'generated', got %q", source)
	}

	// Verify it was set as env var.
	if got := os.Getenv("REPLOG_SECRET_KEY"); got != key1 {
		t.Errorf("expected env var set to %q, got %q", key1, got)
	}

	// Clear the env var again to test DB retrieval.
	t.Setenv("REPLOG_SECRET_KEY", "")

	// Second call should retrieve the same key from DB.
	key2, source, err := GetOrCreateSecretKey(db)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if key2 != key1 {
		t.Errorf("expected same key %q, got %q", key1, key2)
	}
	if source != "database" {
		t.Errorf("expected source 'database', got %q", source)
	}

	// With env var set, should prefer env var.
	t.Setenv("REPLOG_SECRET_KEY", "explicit-key")
	key3, source, err := GetOrCreateSecretKey(db)
	if err != nil {
		t.Fatalf("env var call: %v", err)
	}
	if key3 != "explicit-key" {
		t.Errorf("expected 'explicit-key', got %q", key3)
	}
	if source != "env" {
		t.Errorf("expected source 'env', got %q", source)
	}
}

func TestListSettingsByCategoryOrdered(t *testing.T) {
	db := testDB(t)

	ordered := ListSettingsByCategoryOrdered(db)

	if len(ordered) < 3 {
		t.Fatalf("expected at least 3 categories, got %d", len(ordered))
	}

	// Verify ordering: General, Defaults, AI Coach.
	expectedOrder := []string{"General", "Defaults", "AI Coach"}
	for i, expected := range expectedOrder {
		if i >= len(ordered) {
			t.Fatalf("missing category at position %d: want %q", i, expected)
		}
		if ordered[i].Name != expected {
			t.Errorf("category[%d] = %q, want %q", i, ordered[i].Name, expected)
		}
	}
}
