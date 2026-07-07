package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AnthropicProvider implements Provider for the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey string
	model  string
	client *http.Client
}

// NewAnthropicProvider creates an Anthropic provider.
func NewAnthropicProvider(apiKey, model string) *AnthropicProvider {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &AnthropicProvider{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *AnthropicProvider) Name() string { return "Anthropic" }

func (p *AnthropicProvider) Ping(ctx context.Context) error {
	_, err := p.Generate(ctx, "Respond with OK.", "ping", Options{Temperature: 0, MaxTokens: 10})
	return err
}

func (p *AnthropicProvider) Generate(ctx context.Context, systemPrompt, userPrompt string, opts Options) (*Response, error) {
	maxTokens := opts.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	body := map[string]any{
		"model":      p.model,
		"max_tokens": maxTokens,
		"system":     systemPrompt,
		"messages": []map[string]string{
			{"role": "user", "content": userPrompt},
		},
		"temperature": opts.Temperature,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm/anthropic: marshal request: %w", err)
	}

	start := time.Now()
	statusCode, respBody, err := doWithRetry(ctx, p.client, func() (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", p.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		return req, nil
	}, 3)
	if err != nil {
		return nil, fmt.Errorf("llm/anthropic: request failed: %w", err)
	}
	duration := time.Since(start)

	if statusCode != http.StatusOK {
		apiErr := &APIError{
			Provider:   "Anthropic",
			StatusCode: statusCode,
		}
		var errResp struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			apiErr.Code = errResp.Error.Type
			apiErr.Message = errResp.Error.Message
		} else {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		Model      string `json:"model"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("llm/anthropic: parse response: %w", err)
	}

	if len(result.Content) == 0 {
		return nil, fmt.Errorf("llm/anthropic: no content blocks in response")
	}

	// Concatenate all text content blocks (model may split across multiple).
	var textContent string
	for _, block := range result.Content {
		if block.Type == "text" || block.Type == "" {
			textContent += block.Text
		}
	}

	return &Response{
		Content:    textContent,
		Model:      result.Model,
		TokensUsed: result.Usage.InputTokens + result.Usage.OutputTokens,
		Duration:   duration,
		StopReason: result.StopReason,
	}, nil
}
