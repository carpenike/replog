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
		client: &http.Client{
			Timeout: 10 * time.Minute,
		},
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
	// Ollama's native chat endpoint (/api/chat). num_predict caps the output
	// (equivalent to max_tokens) and num_ctx sizes the context window — without
	// it Ollama's default (often 2k–4k) SILENTLY head-truncates this large
	// prompt, discarding the system safety rules. Size num_ctx generously
	// relative to the requested output so the full prompt survives.
	numCtx := opts.MaxTokens + 8192
	if numCtx < 8192 {
		numCtx = 8192
	}
	options := map[string]any{
		"temperature": opts.Temperature,
		"num_ctx":     numCtx,
	}
	if opts.MaxTokens > 0 {
		options["num_predict"] = opts.MaxTokens
	}
	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream":  false,
		"options": options,
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
		Model           string `json:"model"`
		DoneReason      string `json:"done_reason"`
		PromptEvalCount int    `json:"prompt_eval_count"`
		EvalCount       int    `json:"eval_count"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("llm/ollama: parse response: %w", err)
	}

	// Map Ollama's done_reason to our StopReason vocabulary so the truncation
	// hint ("increase max_tokens") can fire. Ollama reports "length" when it
	// hits num_predict and "stop" on a natural end.
	stopReason := result.DoneReason
	if stopReason == "" {
		stopReason = "stop"
	}

	return &Response{
		Content:    result.Message.Content,
		Model:      result.Model,
		TokensUsed: result.PromptEvalCount + result.EvalCount,
		Duration:   duration,
		StopReason: stopReason,
	}, nil
}
