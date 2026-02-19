package models

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
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

// SettingsRegistry defines all known application settings.
var SettingsRegistry = []SettingDefinition{
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

// secretKey returns the 32-byte encryption key derived from REPLOG_SECRET_KEY.
// Returns nil if the env var is not set.
func secretKey() []byte {
	key := os.Getenv("REPLOG_SECRET_KEY")
	if key == "" {
		return nil
	}
	// Pad or truncate to 32 bytes for AES-256.
	b := make([]byte, 32)
	copy(b, []byte(key))
	return b
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
