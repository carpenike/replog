package llm

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// ErrNotConfigured is returned when no AI Coach provider is configured.
var ErrNotConfigured = fmt.Errorf("llm: AI Coach provider not configured")

// APIError represents a structured error from an LLM provider's API.
// It carries both a user-friendly message and the raw status/details for logging.
type APIError struct {
	Provider   string // e.g. "Anthropic", "OpenAI"
	StatusCode int    // HTTP status code
	Code       string // provider error code, e.g. "invalid_request_error"
	Message    string // human-readable message from the API
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s API error (HTTP %d): %s", e.Provider, e.StatusCode, e.Message)
}

// UserMessage returns a coach-friendly error description.
func (e *APIError) UserMessage() string {
	switch {
	case e.StatusCode == 401:
		return fmt.Sprintf("%s: Invalid API key. Please check the API key in Settings.", e.Provider)
	case e.StatusCode == 403:
		return fmt.Sprintf("%s: Access denied. Your API key may not have permission for this model.", e.Provider)
	case e.StatusCode == 429:
		return fmt.Sprintf("%s: Rate limit exceeded. Please wait a moment and try again.", e.Provider)
	case e.StatusCode == 400 && containsAny(e.Message, "credit", "balance", "billing", "payment"):
		return fmt.Sprintf("%s: Insufficient credits. Please check your billing at the provider's website.", e.Provider)
	case e.StatusCode == 400 && containsAny(e.Message, "model", "not found", "does not exist"):
		return fmt.Sprintf("%s: Model not found. Please check the model name in Settings.", e.Provider)
	case e.StatusCode == 400:
		return fmt.Sprintf("%s: Bad request — %s", e.Provider, e.Message)
	case e.StatusCode == 404:
		return fmt.Sprintf("%s: Endpoint not found. Please check the Base URL in Settings.", e.Provider)
	case e.StatusCode == 500, e.StatusCode == 502, e.StatusCode == 503:
		return fmt.Sprintf("%s: The service is temporarily unavailable. Please try again later.", e.Provider)
	default:
		return fmt.Sprintf("%s: Unexpected error (HTTP %d). Please try again or check Settings.", e.Provider, e.StatusCode)
	}
}

// containsAny returns true if s contains any of the substrings (case-insensitive).
func containsAny(s string, subs ...string) bool {
	lower := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(lower, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

// Provider is the interface for LLM backends.
type Provider interface {
	// Generate sends a system prompt and user prompt to the LLM and returns
	// the response text. The response should be CatalogJSON for program
	// generation.
	Generate(ctx context.Context, systemPrompt, userPrompt string, opts Options) (*Response, error)

	// Ping validates connectivity and credentials. Returns nil if the
	// provider is reachable and authenticated. Used for admin "Test Connection".
	Ping(ctx context.Context) error

	// Name returns the display name of this provider (e.g. "OpenAI", "Anthropic").
	Name() string
}

// Options controls LLM generation behavior.
type Options struct {
	Temperature float64
	MaxTokens   int
}

// Response holds the LLM's output.
type Response struct {
	Content    string
	Model      string
	TokensUsed int
	Duration   time.Duration
	StopReason string // "end_turn", "max_tokens", "stop", etc.
}

// GenerationRequest describes what to generate.
type GenerationRequest struct {
	AthleteID            int64
	ProgramName          string   // e.g. "Sport Performance Month 4"
	NumWeeks             int      // 1 (loop) or N (fixed block)
	NumDays              int      // training days per week
	IsLoop               bool
	FocusAreas           []string // e.g. ["power", "conditioning"]
	CoachDirections      string   // free-text instructions for the LLM
	ReferenceTemplateIDs []int64  // coach-selected reference program IDs (empty = none)
}

// GenerationResult holds the complete output from a generation.
type GenerationResult struct {
	CatalogJSON []byte        // valid CatalogJSON ready for import
	Reasoning   string        // LLM's explanation of choices
	RawResponse string        // full unprocessed LLM response
	TokensUsed  int
	Duration    time.Duration
	Model       string
	StopReason  string // "end_turn"/"stop" = complete, "max_tokens"/"length" = truncated
}

// NewProviderFromSettings creates a Provider using the current app_settings
// configuration (with env var overrides).
func NewProviderFromSettings(db *sql.DB) (Provider, error) {
	provider := models.GetSetting(db, "llm.provider")
	if provider == "" {
		return nil, ErrNotConfigured
	}

	model := models.GetSetting(db, "llm.model")
	apiKey := models.GetSetting(db, "llm.api_key")
	baseURL := models.GetSetting(db, "llm.base_url")

	switch provider {
	case "openai":
		return NewOpenAIProvider(apiKey, model, baseURL), nil
	case "anthropic":
		return NewAnthropicProvider(apiKey, model), nil
	case "ollama":
		return NewOllamaProvider(baseURL, model), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q", provider)
	}
}

// TemperatureFromSettings reads the temperature setting.
func TemperatureFromSettings(db *sql.DB) float64 {
	v := models.GetSetting(db, "llm.temperature")
	var temp float64
	if _, err := fmt.Sscanf(v, "%f", &temp); err != nil {
		return 0.7 // fallback default
	}
	return temp
}

// MaxTokensFromSettings reads the max_tokens setting.
func MaxTokensFromSettings(db *sql.DB) int {
	v := models.GetSetting(db, "llm.max_tokens")
	var tokens int
	if _, err := fmt.Sscanf(v, "%d", &tokens); err != nil || tokens <= 0 {
		return 32768 // fallback default
	}
	return tokens
}

// SystemPromptOverrideFromSettings reads the system_prompt_override setting.
// Returns empty string if not set, in which case the default prompt is used.
func SystemPromptOverrideFromSettings(db *sql.DB) string {
	return models.GetSetting(db, "llm.system_prompt_override")
}
