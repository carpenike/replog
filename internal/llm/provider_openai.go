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

// OpenAIProvider implements Provider for OpenAI and OpenAI-compatible APIs.
type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// NewOpenAIProvider creates an OpenAI provider.
// If baseURL is empty, it defaults to the official OpenAI API.
func NewOpenAIProvider(apiKey, model, baseURL string) *OpenAIProvider {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIProvider{
		apiKey:  apiKey,
		model:   model,
		baseURL: baseURL,
		client:  &http.Client{
			Timeout: 5 * time.Minute,
		},
	}
}

func (p *OpenAIProvider) Name() string { return "OpenAI" }

func (p *OpenAIProvider) Ping(ctx context.Context) error {
	_, err := p.Generate(ctx, "Respond with OK.", "ping", Options{Temperature: 0, MaxTokens: 10})
	return err
}

func (p *OpenAIProvider) Generate(ctx context.Context, systemPrompt, userPrompt string, opts Options) (*Response, error) {
	body := map[string]any{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"temperature": opts.Temperature,
	}
	if opts.MaxTokens > 0 {
		body["max_tokens"] = opts.MaxTokens
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("llm/openai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", p.baseURL+"/chat/completions", bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("llm/openai: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	start := time.Now()
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("llm/openai: request failed: %w", err)
	}
	defer resp.Body.Close()
	duration := time.Since(start)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm/openai: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{
			Provider:   "OpenAI",
			StatusCode: resp.StatusCode,
		}
		var errResp struct {
			Error struct {
				Type    string `json:"type"`
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		}
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error.Message != "" {
			apiErr.Code = errResp.Error.Code
			if apiErr.Code == "" {
				apiErr.Code = errResp.Error.Type
			}
			apiErr.Message = errResp.Error.Message
		} else {
			apiErr.Message = string(respBody)
		}
		return nil, apiErr
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Model string `json:"model"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("llm/openai: parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("llm/openai: no choices in response")
	}

	return &Response{
		Content:    result.Choices[0].Message.Content,
		Model:      result.Model,
		TokensUsed: result.Usage.TotalTokens,
		Duration:   duration,
		StopReason: result.Choices[0].FinishReason,
	}, nil
}
