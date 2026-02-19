package llm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/carpenike/replog/internal/models"
)

// ErrNotConfigured is returned when no AI Coach provider is configured.
var ErrNotConfigured = fmt.Errorf("llm: AI Coach provider not configured")

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
}

// GenerationRequest describes what to generate.
type GenerationRequest struct {
	AthleteID       int64
	ProgramName     string   // e.g. "Sport Performance Month 4"
	NumWeeks        int      // 1 (loop) or N (fixed block)
	NumDays         int      // training days per week
	IsLoop          bool
	FocusAreas      []string // e.g. ["power", "conditioning"]
	CoachDirections string   // free-text instructions for the LLM
}

// GenerationResult holds the complete output from a generation.
type GenerationResult struct {
	CatalogJSON []byte        // valid CatalogJSON ready for import
	Reasoning   string        // LLM's explanation of choices
	RawResponse string        // full unprocessed LLM response
	TokensUsed  int
	Duration    time.Duration
	Model       string
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
