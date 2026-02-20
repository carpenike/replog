package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/carpenike/replog/internal/models"
)

// --- Settings helpers tests ---

func TestNewProviderFromSettings_NotConfigured(t *testing.T) {
	db := testDB(t)
	_, err := NewProviderFromSettings(db)
	if err != ErrNotConfigured {
		t.Errorf("got %v, want ErrNotConfigured", err)
	}
}

func TestNewProviderFromSettings_Anthropic(t *testing.T) {
	db := testDB(t)
	models.SetSetting(db, "llm.provider", "anthropic")
	models.SetSetting(db, "llm.model", "claude-3-haiku-20240307")
	models.SetSetting(db, "llm.api_key", "test-key")

	p, err := NewProviderFromSettings(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "Anthropic" {
		t.Errorf("name = %q, want Anthropic", p.Name())
	}
}

func TestNewProviderFromSettings_OpenAI(t *testing.T) {
	db := testDB(t)
	models.SetSetting(db, "llm.provider", "openai")
	models.SetSetting(db, "llm.model", "gpt-4o")

	p, err := NewProviderFromSettings(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "OpenAI" {
		t.Errorf("name = %q, want OpenAI", p.Name())
	}
}

func TestNewProviderFromSettings_Ollama(t *testing.T) {
	db := testDB(t)
	models.SetSetting(db, "llm.provider", "ollama")

	p, err := NewProviderFromSettings(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "Ollama" {
		t.Errorf("name = %q, want Ollama", p.Name())
	}
}

func TestNewProviderFromSettings_InvalidProvider(t *testing.T) {
	db := testDB(t)
	models.SetSetting(db, "llm.provider", "invalid")

	_, err := NewProviderFromSettings(db)
	if err == nil {
		t.Fatal("expected error for unknown provider")
	}
}

func TestTemperatureFromSettings(t *testing.T) {
	db := testDB(t)

	// Default when not set.
	temp := TemperatureFromSettings(db)
	if temp != 0.7 {
		t.Errorf("default temperature = %f, want 0.7", temp)
	}

	// Custom value.
	models.SetSetting(db, "llm.temperature", "0.3")
	temp = TemperatureFromSettings(db)
	if temp != 0.3 {
		t.Errorf("custom temperature = %f, want 0.3", temp)
	}
}

func TestMaxTokensFromSettings(t *testing.T) {
	db := testDB(t)

	// Default when not set.
	tokens := MaxTokensFromSettings(db)
	if tokens != 32768 {
		t.Errorf("default tokens = %d, want 32768", tokens)
	}

	// Custom value.
	models.SetSetting(db, "llm.max_tokens", "16384")
	tokens = MaxTokensFromSettings(db)
	if tokens != 16384 {
		t.Errorf("custom tokens = %d, want 16384", tokens)
	}
}

func TestSystemPromptOverrideFromSettings(t *testing.T) {
	db := testDB(t)

	// Default when not set.
	override := SystemPromptOverrideFromSettings(db)
	if override != "" {
		t.Errorf("default override = %q, want empty", override)
	}

	// Custom value.
	models.SetSetting(db, "llm.system_prompt_override", "Custom prompt")
	override = SystemPromptOverrideFromSettings(db)
	if override != "Custom prompt" {
		t.Errorf("override = %q, want 'Custom prompt'", override)
	}
}

// --- API Error tests ---

func TestAPIError_UserMessage(t *testing.T) {
	tests := []struct {
		name       string
		err        *APIError
		wantSubstr string
	}{
		{
			name:       "401 invalid key",
			err:        &APIError{Provider: "OpenAI", StatusCode: 401, Message: "invalid api key"},
			wantSubstr: "Invalid API key",
		},
		{
			name:       "429 rate limit",
			err:        &APIError{Provider: "Anthropic", StatusCode: 429, Message: "rate limited"},
			wantSubstr: "Rate limit exceeded",
		},
		{
			name:       "400 billing",
			err:        &APIError{Provider: "OpenAI", StatusCode: 400, Message: "insufficient credit balance"},
			wantSubstr: "Insufficient credits",
		},
		{
			name:       "400 model not found",
			err:        &APIError{Provider: "OpenAI", StatusCode: 400, Message: "model not found"},
			wantSubstr: "Model not found",
		},
		{
			name:       "503 unavailable",
			err:        &APIError{Provider: "Anthropic", StatusCode: 503, Message: "service unavailable"},
			wantSubstr: "temporarily unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.err.UserMessage()
			if msg == "" {
				t.Fatal("UserMessage returned empty string")
			}
			if !containsAny(msg, tt.wantSubstr) {
				t.Errorf("UserMessage = %q, want to contain %q", msg, tt.wantSubstr)
			}
		})
	}
}

// --- HTTP provider integration tests (using httptest) ---

func TestOpenAIProvider_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("auth header = %q", r.Header.Get("Authorization"))
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{
					"message":       map[string]string{"content": "Hello from OpenAI"},
					"finish_reason": "stop",
				},
			},
			"model": "gpt-4o",
			"usage": map[string]int{"total_tokens": 42},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProvider("test-key", "gpt-4o", srv.URL)
	result, err := p.Generate(context.Background(), "system", "user", Options{Temperature: 0.7, MaxTokens: 100})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "Hello from OpenAI" {
		t.Errorf("content = %q", result.Content)
	}
	if result.Model != "gpt-4o" {
		t.Errorf("model = %q", result.Model)
	}
	if result.TokensUsed != 42 {
		t.Errorf("tokens = %d", result.TokensUsed)
	}
	if result.StopReason != "stop" {
		t.Errorf("stop_reason = %q", result.StopReason)
	}
}

func TestOpenAIProvider_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"type": "invalid_api_key", "message": "bad key"},
		})
	}))
	defer srv.Close()

	p := NewOpenAIProvider("bad-key", "gpt-4o", srv.URL)
	_, err := p.Generate(context.Background(), "system", "user", Options{})
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("status = %d", apiErr.StatusCode)
	}
	if apiErr.Provider != "OpenAI" {
		t.Errorf("provider = %q", apiErr.Provider)
	}
}

func TestAnthropicProvider_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("x-api-key = %q", r.Header.Get("x-api-key"))
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version header")
		}

		resp := map[string]any{
			"content":     []map[string]string{{"type": "text", "text": "Hello from Anthropic"}},
			"model":       "claude-sonnet-4-20250514",
			"stop_reason": "end_turn",
			"usage":       map[string]int{"input_tokens": 10, "output_tokens": 20},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	// Override the client to point at our test server.
	p.client = srv.Client()
	// We need to use a custom transport to redirect to our test server.
	// For simplicity, create a new provider that points at the test server.
	// Instead, set up a round-tripper that redirects.
	origURL := "https://api.anthropic.com"
	p.client.Transport = &rewriteTransport{
		base:    http.DefaultTransport,
		fromURL: origURL,
		toURL:   srv.URL,
	}

	result, err := p.Generate(context.Background(), "system", "user", Options{Temperature: 0.5, MaxTokens: 100})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "Hello from Anthropic" {
		t.Errorf("content = %q", result.Content)
	}
	if result.TokensUsed != 30 {
		t.Errorf("tokens = %d, want 30", result.TokensUsed)
	}
}

func TestOllamaProvider_Generate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"message": map[string]string{"content": "Hello from Ollama"},
			"model":   "llama3",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "llama3")
	result, err := p.Generate(context.Background(), "system", "user", Options{Temperature: 0.5})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if result.Content != "Hello from Ollama" {
		t.Errorf("content = %q", result.Content)
	}
	if result.Model != "llama3" {
		t.Errorf("model = %q", result.Model)
	}
}

func TestOllamaProvider_Ping(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"models":[]}`)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "llama3")
	if err := p.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

// rewriteTransport intercepts requests to fromURL and rewrites them to toURL.
// Used to test the Anthropic provider which hardcodes the API URL.
type rewriteTransport struct {
	base    http.RoundTripper
	fromURL string
	toURL   string
}

func (t *rewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	reqURL := req.URL.String()
	if len(reqURL) >= len(t.fromURL) && reqURL[:len(t.fromURL)] == t.fromURL {
		newURL := t.toURL + reqURL[len(t.fromURL):]
		newReq := req.Clone(req.Context())
		u, _ := req.URL.Parse(newURL)
		newReq.URL = u
		return t.base.RoundTrip(newReq)
	}
	return t.base.RoundTrip(req)
}
