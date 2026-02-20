package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OllamaProvider implements Provider for local Ollama instances.
type OllamaProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

// NewOllamaProvider creates an Ollama provider.
// baseURL defaults to http://localhost:11434 if empty.
func NewOllamaProvider(baseURL, model string) *OllamaProvider {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "llama3"
	}
	return &OllamaProvider{
		baseURL: baseURL,
		model:   model,
		client:  &http.Client{},
	}
}

func (p *OllamaProvider) Name() string { return "Ollama" }

func (p *OllamaProvider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", p.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("llm/ollama: create request: %w", err)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("llm/ollama: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("llm/ollama: HTTP %d", resp.StatusCode)
	}
	return nil
}

func (p *OllamaProvider) Generate(ctx context.Context, systemPrompt, userPrompt string, opts Options) (*Response, error) {
	// Use Ollama's OpenAI-compatible chat completions endpoint.
	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": false,
		"options": map[string]any{
			"temperature": opts.Temperature,
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm/ollama: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/api/chat", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm/ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm/ollama: request failed: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm/ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{
			Provider:   "Ollama",
			StatusCode: resp.StatusCode,
		}
		var errResp struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			apiErr.Message = errResp.Error
		} else {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	var result struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("llm/ollama: parse response: %w", err)
	}

	return &Response{
		Content:    result.Message.Content,
		Model:      result.Model,
		TokensUsed: 0, // Ollama doesn't always report tokens
		Duration:   duration,
		StopReason: "stop", // Ollama doesn't report stop reason in chat API
	}, nil
}
