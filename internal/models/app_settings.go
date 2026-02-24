package models

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// SettingDefinition describes a configurable application setting.
type SettingDefinition struct {
	Key         string   // DB key, e.g. "llm.provider"
	EnvVar      string   // Override env var, e.g. "REPLOG_LLM_PROVIDER"
	Default     string   // Built-in default value
	Label       string   // Human-readable label for admin UI
	Description string   // Help text for admin UI
	FieldType   string   // "text", "password", "select", "number", "textarea"
	Options     []string // Valid values for "select" type
	Category    string   // Grouping key for admin UI
	Sensitive   bool     // If true, value is encrypted in DB and masked in UI
}

// SettingValue represents a resolved setting with its source.
type SettingValue struct {
	Key      string
	Value    string
	Source   string // "env", "db", "default"
	Masked   string // Display value (masked for sensitive settings)
	ReadOnly bool   // True if set via env var (not editable in UI)
}

// CategoryOrder defines the display order for setting categories in the admin UI.
var CategoryOrder = []string{"General", "Defaults", "Notifications", "AI Coach", "Maintenance"}

// SettingsRegistry defines all known application settings.
var SettingsRegistry = []SettingDefinition{
	// --- General ---
	{
		Key: "app.name", EnvVar: "REPLOG_APP_NAME", Default: "RepLog",
		Label: "Application Name", Description: "Custom name shown in page titles and navigation",
		FieldType: "text", Category: "General",
	},
	// --- Defaults ---
	{
		Key: "defaults.weight_unit", EnvVar: "", Default: "lbs",
		Label: "Default Weight Unit", Description: "Default weight unit for new users (lbs or kg)",
		FieldType: "select", Options: []string{"lbs", "kg"},
		Category: "Defaults",
	},
	{
		Key: "defaults.timezone", EnvVar: "", Default: "America/New_York",
		Label: "Default Timezone", Description: "Default timezone for new users (e.g. America/Chicago, Europe/London)",
		FieldType: "text", Category: "Defaults",
	},
	{
		Key: "defaults.date_format", EnvVar: "", Default: "Jan 2, 2006",
		Label: "Default Date Format", Description: "Default date display format for new users",
		FieldType: "select", Options: []string{"Jan 2, 2006", "2006-01-02", "02/01/2006", "01/02/2006", "2 Jan 2006", "Monday, Jan 2"},
		Category: "Defaults",
	},
	{
		Key: "defaults.rest_seconds", EnvVar: "", Default: "90",
		Label: "Default Rest Timer", Description: "Default rest time in seconds when an exercise doesn't specify one (e.g. 60, 90, 120)",
		FieldType: "number", Category: "Defaults",
	},
	// --- Notifications ---
	{
		Key: "smtp.host", EnvVar: "REPLOG_SMTP_HOST", Default: "",
		Label: "SMTP Host", Description: "SMTP server hostname (e.g. smtp.gmail.com, smtp.mailgun.org)",
		FieldType: "text", Category: "Notifications",
	},
	{
		Key: "smtp.port", EnvVar: "REPLOG_SMTP_PORT", Default: "587",
		Label: "SMTP Port", Description: "SMTP server port (587 for STARTTLS, 465 for SSL)",
		FieldType: "number", Category: "Notifications",
	},
	{
		Key: "smtp.username", EnvVar: "REPLOG_SMTP_USERNAME", Default: "",
		Label: "SMTP Username", Description: "SMTP authentication username",
		FieldType: "text", Category: "Notifications",
	},
	{
		Key: "smtp.password", EnvVar: "REPLOG_SMTP_PASSWORD", Default: "",
		Label: "SMTP Password", Description: "SMTP authentication password or app-specific password",
		FieldType: "password", Category: "Notifications", Sensitive: true,
	},
	{
		Key: "smtp.from", EnvVar: "REPLOG_SMTP_FROM", Default: "",
		Label: "From Address", Description: "Sender email address (e.g. replog@yourdomain.com)",
		FieldType: "text", Category: "Notifications",
	},
	{
		Key: "notify.urls", EnvVar: "REPLOG_NOTIFY_URLS", Default: "",
		Label: "Broadcast URLs", Description: "Shoutrrr URLs for broadcast notifications (ntfy, Discord, etc). One per line. Not per-user — use SMTP for per-user delivery.",
		FieldType: "textarea", Category: "Notifications",
	},
	// --- AI Coach ---
	{
		Key: "llm.provider", EnvVar: "REPLOG_LLM_PROVIDER", Default: "",
		Label: "Provider", Description: "AI provider for program generation",
		FieldType: "select", Options: []string{"", "openai", "anthropic", "ollama"},
		Category: "AI Coach",
	},
	{
		Key: "llm.model", EnvVar: "REPLOG_LLM_MODEL", Default: "",
		Label: "Model", Description: "Model name (e.g. gpt-4o, claude-sonnet-4-20250514, llama3)",
		FieldType: "text", Category: "AI Coach",
	},
	{
		Key: "llm.api_key", EnvVar: "REPLOG_LLM_API_KEY", Default: "",
		Label: "API Key", Description: "Provider API key (not needed for Ollama)",
		FieldType: "password", Category: "AI Coach", Sensitive: true,
	},
	{
		Key: "llm.base_url", EnvVar: "REPLOG_LLM_BASE_URL", Default: "",
		Label: "Base URL", Description: "Custom API endpoint (required for Ollama, optional for others)",
		FieldType: "text", Category: "AI Coach",
	},
	{
		Key: "llm.temperature", EnvVar: "REPLOG_LLM_TEMPERATURE", Default: "0.7",
		Label: "Temperature", Description: "Creativity level (0.0 = deterministic, 2.0 = very creative)",
		FieldType: "number", Category: "AI Coach",
	},
	{
		Key: "llm.max_tokens", EnvVar: "REPLOG_LLM_MAX_TOKENS", Default: "32768",
		Label: "Max Tokens", Description: "Maximum output tokens for generation (4096–65536)",
		FieldType: "number", Category: "AI Coach",
	},
	{
		Key: "llm.system_prompt_override", EnvVar: "", Default: "",
		Label: "System Prompt Override", Description: "Replace the default system prompt (leave empty to use built-in prompt)",
		FieldType: "textarea", Category: "AI Coach",
	},
	// --- Maintenance ---
	{
		Key: "maintenance.interval_hours", EnvVar: "", Default: "24",
		Label: "Schedule Interval (hours)", Description: "How often background maintenance runs (1–168 hours)",
		FieldType: "number", Category: "Maintenance",
	},
	{
		Key: "maintenance.retention_days", EnvVar: "", Default: "90",
		Label: "Notification Retention (days)", Description: "Read notifications older than this are pruned (1–365 days)",
		FieldType: "number", Category: "Maintenance",
	},
}

// GetSetting returns a configuration value using the resolution chain:
// env var → app_settings row → built-in default.
func GetSetting(db *sql.DB, key string) string {
	def := findDefinition(key)
	if def == nil {
		return ""
	}

	// 1. Environment variable always wins.
	if def.EnvVar != "" {
		if v := os.Getenv(def.EnvVar); v != "" {
			return v
		}
	}

	// 2. Database setting.
	var raw string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, key).Scan(&raw)
	if err == nil {
		if def.Sensitive && strings.HasPrefix(raw, "enc:") {
			decrypted, err := decryptValue(raw[4:])
			if err == nil {
				return decrypted
			}
			// Fall through to default if decryption fails.
		} else {
			return raw
		}
	}

	// 3. Built-in default.
	return def.Default
}

// SetSetting stores a configuration value in the database.
// Sensitive values are encrypted if REPLOG_SECRET_KEY is set.
func SetSetting(db *sql.DB, key, value string) error {
	def := findDefinition(key)
	if def == nil {
		return fmt.Errorf("models: unknown setting key %q", key)
	}

	storeValue := value
	if def.Sensitive && value != "" {
		encrypted, err := encryptValue(value)
		if err != nil {
			return fmt.Errorf("models: encrypt setting %q: %w", key, err)
		}
		storeValue = "enc:" + encrypted
	}

	_, err := db.Exec(
		`INSERT INTO app_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		key, storeValue,
	)
	if err != nil {
		return fmt.Errorf("models: set setting %q: %w", key, err)
	}
	return nil
}

// DeleteSetting removes a setting from the database (reverts to env var or default).
func DeleteSetting(db *sql.DB, key string) error {
	_, err := db.Exec(`DELETE FROM app_settings WHERE key = ?`, key)
	if err != nil {
		return fmt.Errorf("models: delete setting %q: %w", key, err)
	}
	return nil
}

// ListSettings returns all known settings with their resolved values and sources.
func ListSettings(db *sql.DB) []SettingValue {
	var results []SettingValue
	for _, def := range SettingsRegistry {
		sv := resolveSettingValue(db, def)
		results = append(results, sv)
	}
	return results
}

// ListSettingsByCategory returns settings grouped by category.
func ListSettingsByCategory(db *sql.DB) map[string][]SettingValue {
	groups := make(map[string][]SettingValue)
	for _, def := range SettingsRegistry {
		sv := resolveSettingValue(db, def)
		groups[def.Category] = append(groups[def.Category], sv)
	}
	return groups
}

// GetSettingDefinition returns the definition for a known setting key.
func GetSettingDefinition(key string) *SettingDefinition {
	return findDefinition(key)
}

// GetSettingValue returns the full SettingValue (with source, mask, etc.) for a key.
func GetSettingValue(db *sql.DB, key string) SettingValue {
	def := findDefinition(key)
	if def == nil {
		return SettingValue{Key: key}
	}
	return resolveSettingValue(db, *def)
}

// IsAICoachConfigured returns true if an AI Coach provider is configured.
func IsAICoachConfigured(db *sql.DB) bool {
	return GetSetting(db, "llm.provider") != ""
}

// GetDefaultWeightUnit returns the configured default weight unit from app settings,
// falling back to the hardcoded constant.
func GetDefaultWeightUnit(db *sql.DB) string {
	if v := GetSetting(db, "defaults.weight_unit"); v != "" {
		return v
	}
	return "lbs"
}

// GetDefaultTimezone returns the configured default timezone from app settings.
func GetDefaultTimezone(db *sql.DB) string {
	if v := GetSetting(db, "defaults.timezone"); v != "" {
		return v
	}
	return "America/New_York"
}

// GetDefaultDateFormat returns the configured default date format from app settings.
func GetDefaultDateFormat(db *sql.DB) string {
	if v := GetSetting(db, "defaults.date_format"); v != "" {
		return v
	}
	return "Jan 2, 2006"
}

// GetDefaultRestSeconds returns the configured default rest seconds from app settings.
func GetDefaultRestSeconds(db *sql.DB) int {
	if v := GetSetting(db, "defaults.rest_seconds"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 90
}

// GetAppName returns the configured application name from app settings.
func GetAppName(db *sql.DB) string {
	if v := GetSetting(db, "app.name"); v != "" {
		return v
	}
	return "RepLog"
}

// GetMaintenanceIntervalHours returns the scheduler interval from app settings.
func GetMaintenanceIntervalHours(db *sql.DB) int {
	if v := GetSetting(db, "maintenance.interval_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 168 {
			return n
		}
	}
	return 24
}

// GetMaintenanceRetentionDays returns the notification retention period from app settings.
func GetMaintenanceRetentionDays(db *sql.DB) int {
	if v := GetSetting(db, "maintenance.retention_days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 && n <= 365 {
			return n
		}
	}
	return 90
}

// GetOrCreateSecretKey ensures a secret key exists for encrypting sensitive settings.
// Resolution: REPLOG_SECRET_KEY env var → _internal.secret_key DB row → auto-generate.
// The key is stored in plaintext in app_settings (since it IS the encryption key).
// Returns the key and sets it as an env var so the rest of the code can use it.
func GetOrCreateSecretKey(db *sql.DB) (key, source string, err error) {
	// 1. Check env var — if provided, persist to DB so the key survives
	//    even if the env var is later removed.
	if key = os.Getenv("REPLOG_SECRET_KEY"); key != "" {
		_, _ = db.Exec(
			`INSERT INTO app_settings (key, value) VALUES ('_internal.secret_key', ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key,
		)
		return key, "env", nil
	}

	// 2. Check DB for previously generated key.
	err = db.QueryRow(`SELECT value FROM app_settings WHERE key = '_internal.secret_key'`).Scan(&key)
	if err == nil && key != "" {
		os.Setenv("REPLOG_SECRET_KEY", key)
		return key, "database", nil
	}

	// 3. Generate a new key.
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("models: generate secret key: %w", err)
	}
	key = base64.StdEncoding.EncodeToString(buf)

	_, err = db.Exec(
		`INSERT INTO app_settings (key, value) VALUES ('_internal.secret_key', ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key,
	)
	if err != nil {
		return "", "", fmt.Errorf("models: store secret key: %w", err)
	}

	os.Setenv("REPLOG_SECRET_KEY", key)
	return key, "generated", nil
}

// ListSettingsByCategoryOrdered returns settings grouped by category in the
// order defined by CategoryOrder.
func ListSettingsByCategoryOrdered(db *sql.DB) []CategoryGroup {
	groups := ListSettingsByCategory(db)
	var ordered []CategoryGroup
	seen := make(map[string]bool)
	for _, cat := range CategoryOrder {
		if settings, ok := groups[cat]; ok {
			ordered = append(ordered, CategoryGroup{Name: cat, Settings: settings})
			seen[cat] = true
		}
	}
	// Append any categories not in CategoryOrder (future-proofing).
	for cat, settings := range groups {
		if !seen[cat] {
			ordered = append(ordered, CategoryGroup{Name: cat, Settings: settings})
		}
	}
	return ordered
}

// CategoryGroup holds settings for a single category, for ordered rendering.
type CategoryGroup struct {
	Name     string
	Settings []SettingValue
}

// --- Internal helpers ---

func findDefinition(key string) *SettingDefinition {
	for i := range SettingsRegistry {
		if SettingsRegistry[i].Key == key {
			return &SettingsRegistry[i]
		}
	}
	return nil
}

func resolveSettingValue(db *sql.DB, def SettingDefinition) SettingValue {
	sv := SettingValue{Key: def.Key}

	// Check env var first.
	if def.EnvVar != "" {
		if v := os.Getenv(def.EnvVar); v != "" {
			sv.Value = v
			sv.Source = "env"
			sv.ReadOnly = true
			sv.Masked = maskValue(v, def.Sensitive)
			return sv
		}
	}

	// Check database.
	var raw string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = ?`, def.Key).Scan(&raw)
	if err == nil {
		sv.Source = "db"
		if def.Sensitive && strings.HasPrefix(raw, "enc:") {
			decrypted, err := decryptValue(raw[4:])
			if err == nil {
				sv.Value = decrypted
				sv.Masked = maskValue(decrypted, true)
			} else {
				sv.Value = ""
				sv.Masked = "(decryption failed)"
			}
		} else {
			sv.Value = raw
			sv.Masked = maskValue(raw, def.Sensitive)
		}
		return sv
	}

	// Default.
	sv.Value = def.Default
	sv.Source = "default"
	sv.Masked = maskValue(def.Default, def.Sensitive)
	return sv
}

func maskValue(value string, sensitive bool) string {
	if !sensitive || value == "" {
		return value
	}
	if len(value) <= 8 {
		return "••••••••"
	}
	return value[:4] + "••••" + value[len(value)-4:]
}

// --- Encryption helpers ---

// secretKey returns the 32-byte encryption key derived from REPLOG_SECRET_KEY
// using HKDF (RFC 5869). Returns nil if the env var is not set.
func secretKey() []byte {
	key := os.Getenv("REPLOG_SECRET_KEY")
	if key == "" {
		return nil
	}
	// Derive a proper 32-byte AES-256 key using HKDF with a fixed salt and info.
	h := hkdf.New(sha256.New, []byte(key), []byte("replog-settings-v1"), []byte("aes-256-gcm"))
	derived := make([]byte, 32)
	if _, err := io.ReadFull(h, derived); err != nil {
		return nil
	}
	return derived
}

func encryptValue(plaintext string) (string, error) {
	key := secretKey()
	if key == nil {
		return "", fmt.Errorf("REPLOG_SECRET_KEY not set — cannot encrypt sensitive settings")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := aesGCM.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptValue(encoded string) (string, error) {
	key := secretKey()
	if key == nil {
		return "", fmt.Errorf("REPLOG_SECRET_KEY not set — cannot decrypt")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}
