package models

import (
	"context"
	"database/sql"
	"os"
	"testing"
)

func TestGetSetting_EnvOverride(t *testing.T) {
	db := testDB(t)

	// Set a DB value.
	if err := SetSetting(context.Background(), db, "llm.provider", "ollama"); err != nil {
		t.Fatalf("set setting: %v", err)
	}

	// Without env var, should return DB value.
	got := GetSetting(context.Background(), db, "llm.provider")
	if got != "ollama" {
		t.Errorf("expected 'ollama' from DB, got %q", got)
	}

	// With env var, env should win.
	t.Setenv("REPLOG_LLM_PROVIDER", "openai")
	got = GetSetting(context.Background(), db, "llm.provider")
	if got != "openai" {
		t.Errorf("expected 'openai' from env, got %q", got)
	}
}

func TestGetSetting_Default(t *testing.T) {
	db := testDB(t)

	got := GetSetting(context.Background(), db, "llm.temperature")
	if got != "0.7" {
		t.Errorf("expected default '0.7', got %q", got)
	}
}

func TestGetSetting_UnknownKey(t *testing.T) {
	db := testDB(t)

	got := GetSetting(context.Background(), db, "nonexistent.key")
	if got != "" {
		t.Errorf("expected empty string for unknown key, got %q", got)
	}
}

func TestSetSetting_CreateAndUpdate(t *testing.T) {
	db := testDB(t)

	// Create.
	if err := SetSetting(context.Background(), db, "llm.model", "gpt-4o"); err != nil {
		t.Fatalf("set setting: %v", err)
	}
	got := GetSetting(context.Background(), db, "llm.model")
	if got != "gpt-4o" {
		t.Errorf("expected 'gpt-4o', got %q", got)
	}

	// Update (upsert).
	if err := SetSetting(context.Background(), db, "llm.model", "claude-sonnet-4-20250514"); err != nil {
		t.Fatalf("update setting: %v", err)
	}
	got = GetSetting(context.Background(), db, "llm.model")
	if got != "claude-sonnet-4-20250514" {
		t.Errorf("expected 'claude-sonnet-4-20250514', got %q", got)
	}
}

func TestSetSetting_UnknownKey(t *testing.T) {
	db := testDB(t)
	err := SetSetting(context.Background(), db, "fake.key", "value")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestDeleteSetting(t *testing.T) {
	db := testDB(t)

	if err := SetSetting(context.Background(), db, "llm.base_url", "http://localhost:11434"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := DeleteSetting(context.Background(), db, "llm.base_url"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Should fall back to default (empty string).
	got := GetSetting(context.Background(), db, "llm.base_url")
	if got != "" {
		t.Errorf("expected empty default after delete, got %q", got)
	}
}

func TestListSettings(t *testing.T) {
	db := testDB(t)

	settings := ListSettings(context.Background(), db)
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

	groups := ListSettingsByCategory(context.Background(), db)
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
	if err := SetSetting(context.Background(), db, "llm.api_key", "sk-test-12345"); err != nil {
		t.Fatalf("set sensitive setting: %v", err)
	}

	// Verify it's stored encrypted in DB.
	var raw string
	db.QueryRow(`SELECT value FROM app_settings WHERE key = 'llm.api_key'`).Scan(&raw)
	if !hasPrefix(raw, "enc:") {
		t.Errorf("expected encrypted value with 'enc:' prefix, got %q", raw)
	}

	// Reading back should decrypt.
	got := GetSetting(context.Background(), db, "llm.api_key")
	if got != "sk-test-12345" {
		t.Errorf("expected decrypted 'sk-test-12345', got %q", got)
	}
}

func TestSensitiveWithoutSecretKey(t *testing.T) {
	db := testDB(t)

	// Ensure no secret key is set.
	t.Setenv("REPLOG_SECRET_KEY", "")

	err := SetSetting(context.Background(), db, "llm.api_key", "sk-test-12345")
	if err == nil {
		t.Fatal("expected error when setting sensitive value without REPLOG_SECRET_KEY")
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
	if got := GetDefaultWeightUnit(context.Background(), db); got != "lbs" {
		t.Errorf("expected default 'lbs', got %q", got)
	}

	// Override via app_settings.
	if err := SetSetting(context.Background(), db, "defaults.weight_unit", "kg"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultWeightUnit(context.Background(), db); got != "kg" {
		t.Errorf("expected 'kg' after override, got %q", got)
	}
}

func TestGetDefaultTimezone(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultTimezone(context.Background(), db); got != "America/New_York" {
		t.Errorf("expected default 'America/New_York', got %q", got)
	}

	if err := SetSetting(context.Background(), db, "defaults.timezone", "Europe/London"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultTimezone(context.Background(), db); got != "Europe/London" {
		t.Errorf("expected 'Europe/London', got %q", got)
	}
}

func TestGetDefaultDateFormat(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultDateFormat(context.Background(), db); got != "Jan 2, 2006" {
		t.Errorf("expected default 'Jan 2, 2006', got %q", got)
	}

	if err := SetSetting(context.Background(), db, "defaults.date_format", "2006-01-02"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultDateFormat(context.Background(), db); got != "2006-01-02" {
		t.Errorf("expected '2006-01-02', got %q", got)
	}
}

func TestGetDefaultRestSeconds(t *testing.T) {
	db := testDB(t)

	if got := GetDefaultRestSeconds(context.Background(), db); got != 90 {
		t.Errorf("expected default 90, got %d", got)
	}

	if err := SetSetting(context.Background(), db, "defaults.rest_seconds", "120"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetDefaultRestSeconds(context.Background(), db); got != 120 {
		t.Errorf("expected 120, got %d", got)
	}
}

func TestGetAppName(t *testing.T) {
	db := testDB(t)

	if got := GetAppName(context.Background(), db); got != "RepLog" {
		t.Errorf("expected default 'RepLog', got %q", got)
	}

	if err := SetSetting(context.Background(), db, "app.name", "Smith Family Gym"); err != nil {
		t.Fatalf("set: %v", err)
	}
	if got := GetAppName(context.Background(), db); got != "Smith Family Gym" {
		t.Errorf("expected 'Smith Family Gym', got %q", got)
	}
}

func countSecretKeyRows(t *testing.T, db *sql.DB) int {
	t.Helper()
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM app_settings WHERE key = '_internal.secret_key'`).Scan(&n)
	if err != nil {
		t.Fatalf("count secret key rows: %v", err)
	}
	return n
}

// TestNewDatabaseHasNoSecretKeyRow proves a migrated database does not
// contain app_settings._internal.secret_key, and that resolving the env
// key — set or unset — neither inserts that row nor reads a legacy one.
func TestNewDatabaseHasNoSecretKeyRow(t *testing.T) {
	db := testDB(t)

	if n := countSecretKeyRows(t, db); n != 0 {
		t.Fatalf("fresh database has %d _internal.secret_key rows, want 0", n)
	}

	t.Setenv("REPLOG_SECRET_KEY", "")
	if _, err := SecretKeyFromEnv(); err == nil {
		t.Fatal("expected error when REPLOG_SECRET_KEY is unset")
	}
	if n := countSecretKeyRows(t, db); n != 0 {
		t.Fatalf("unset env wrote %d _internal.secret_key rows, want 0", n)
	}

	const explicit = "explicit-key"
	t.Setenv("REPLOG_SECRET_KEY", explicit)
	got, err := SecretKeyFromEnv()
	if err != nil {
		t.Fatalf("env var call: %v", err)
	}
	if got != explicit {
		t.Errorf("expected %q, got %q", explicit, got)
	}
	if n := countSecretKeyRows(t, db); n != 0 {
		t.Fatalf("env key was persisted (%d rows), want 0", n)
	}

	// A legacy row is left unread and unmodified.
	const legacy = "legacy-db-key-do-not-load"
	if _, err := db.Exec(`INSERT INTO app_settings (key, value) VALUES ('_internal.secret_key', ?)`, legacy); err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	t.Setenv("REPLOG_SECRET_KEY", "")
	if _, err := SecretKeyFromEnv(); err == nil {
		t.Fatal("expected error when env is unset even if a legacy row exists")
	}
	if got := os.Getenv("REPLOG_SECRET_KEY"); got != "" {
		t.Errorf("legacy row was loaded into the environment as %q", got)
	}
	var stored string
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = '_internal.secret_key'`).Scan(&stored); err != nil {
		t.Fatalf("read legacy row: %v", err)
	}
	if stored != legacy {
		t.Errorf("legacy row changed to %q, want %q", stored, legacy)
	}

	t.Setenv("REPLOG_SECRET_KEY", explicit)
	got, err = SecretKeyFromEnv()
	if err != nil {
		t.Fatalf("env var with legacy row: %v", err)
	}
	if got != explicit {
		t.Errorf("expected env key %q, got %q", explicit, got)
	}
	if err := db.QueryRow(`SELECT value FROM app_settings WHERE key = '_internal.secret_key'`).Scan(&stored); err != nil {
		t.Fatalf("re-read legacy row: %v", err)
	}
	if stored != legacy {
		t.Errorf("env resolution overwrote legacy row with %q", stored)
	}
}

func TestListSettingsByCategoryOrdered(t *testing.T) {
	db := testDB(t)

	ordered := ListSettingsByCategoryOrdered(context.Background(), db)

	if len(ordered) < 5 {
		t.Fatalf("expected at least 5 categories, got %d", len(ordered))
	}

	// Verify ordering: General, Defaults, Notifications, AI Coach, Maintenance.
	expectedOrder := []string{"General", "Defaults", "Notifications", "AI Coach", "Maintenance"}
	for i, expected := range expectedOrder {
		if i >= len(ordered) {
			t.Fatalf("missing category at position %d: want %q", i, expected)
		}
		if ordered[i].Name != expected {
			t.Errorf("category[%d] = %q, want %q", i, ordered[i].Name, expected)
		}
	}
}
func TestGetMaintenanceIntervalHours(t *testing.T) {
	db := testDB(t)

	// Default should be 24.
	if got := GetMaintenanceIntervalHours(context.Background(), db); got != 24 {
		t.Errorf("default interval = %d, want 24", got)
	}

	// Override to 48.
	SetSetting(context.Background(), db, "maintenance.interval_hours", "48")
	if got := GetMaintenanceIntervalHours(context.Background(), db); got != 48 {
		t.Errorf("overridden interval = %d, want 48", got)
	}

	// Invalid value should fall back to default.
	SetSetting(context.Background(), db, "maintenance.interval_hours", "0")
	if got := GetMaintenanceIntervalHours(context.Background(), db); got != 24 {
		t.Errorf("invalid interval fallback = %d, want 24", got)
	}
}

func TestGetMaintenanceRetentionDays(t *testing.T) {
	db := testDB(t)

	// Default should be 90.
	if got := GetMaintenanceRetentionDays(context.Background(), db); got != 90 {
		t.Errorf("default retention = %d, want 90", got)
	}

	// Override to 30.
	SetSetting(context.Background(), db, "maintenance.retention_days", "30")
	if got := GetMaintenanceRetentionDays(context.Background(), db); got != 30 {
		t.Errorf("overridden retention = %d, want 30", got)
	}

	// Value above max should fall back to default.
	SetSetting(context.Background(), db, "maintenance.retention_days", "999")
	if got := GetMaintenanceRetentionDays(context.Background(), db); got != 90 {
		t.Errorf("invalid retention fallback = %d, want 90", got)
	}
}
