// Package openrouter provides an LLM provider implementation for OpenRouter's API.
// OpenRouter is a unified API that routes requests to various LLM providers.
package openrouter

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dagu-org/dagu/internal/common/backoff"
	"github.com/dagu-org/dagu/internal/llm"
)

const (
	providerName        = "openrouter"
	defaultChatEndpoint = "/chat/completions"
	streamPrefix        = "data: "
	streamDoneMarker    = "[DONE]"
)

func init() {
	llm.RegisterProvider(llm.ProviderOpenRouter, New)
}

// Provider implements the llm.Provider interface for OpenRouter.
type Provider struct {
	config           llm.Config
	httpClient       *http.Client
	streamHttpClient *http.Client
	// Optional metadata for OpenRouter
	SiteURL  string
	SiteName string
}

// New creates a new OpenRouter provider.
func New(cfg llm.Config) (llm.Provider, error) {
	if cfg.APIKey == "" {
		return nil, llm.ErrNoAPIKey
	}

	return &Provider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		streamHttpClient: &http.Client{},
		SiteURL:          "https://github.com/dagu-org/dagu",
		SiteName:         "Dagu Workflow Engine",
	}, nil
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return providerName
}

// Chat sends messages and returns the complete response.
func (p *Provider) Chat(ctx context.Context, req *llm.ChatRequest) (*llm.ChatResponse, error) {
	body, err := p.buildRequestBody(req, false)
	if err != nil {
		return nil, err
	}

	respBody, err := p.doRequest(ctx, body, false)
	if err != nil {
		return nil, err
	}
	defer func() { _ = respBody.Close() }()

	var resp chatCompletionResponse
	if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
		return nil, llm.WrapError(providerName, fmt.Errorf("failed to decode response: %w", err))
	}

	if len(resp.Choices) == 0 {
		return nil, llm.WrapError(providerName, fmt.Errorf("no choices in response"))
	}

	return &llm.ChatResponse{
		Content:      resp.Choices[0].Message.Content,
		FinishReason: resp.Choices[0].FinishReason,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}, nil
}

// ChatStream sends messages and streams the response.
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	body, err := p.buildRequestBody(req, true)
	if err != nil {
		return nil, err
	}

	respBody, err := p.doRequest(ctx, body, true)
	if err != nil {
		return nil, err
	}

	events := make(chan llm.StreamEvent)
	go p.streamResponse(ctx, respBody, events)

	return events, nil
}

func (p *Provider) buildRequestBody(req *llm.ChatRequest, stream bool) ([]byte, error) {
	messages := make([]message, len(req.Messages))
	for i, m := range req.Messages {
		messages[i] = message{
			Role:    string(m.Role),
			Content: m.Content,
		}
		if m.Name != "" {
			messages[i].Name = m.Name
		}
		if m.ToolCallID != "" {
			messages[i].ToolCallID = m.ToolCallID
		}
	}

	chatReq := chatCompletionRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.MaxTokens != nil {
		chatReq.MaxTokens = req.MaxTokens
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		chatReq.Stop = req.Stop
	}

	// Add reasoning configuration if enabled
	// OpenRouter normalizes reasoning across different providers
	if req.Thinking != nil && req.Thinking.Enabled {
		reasoning := &reasoningRequest{}
		if req.Thinking.Effort != "" {
			reasoning.Effort = string(req.Thinking.Effort)
		} else {
			reasoning.Effort = string(llm.ThinkingEffortMedium)
		}
		if req.Thinking.BudgetTokens != nil {
			reasoning.MaxTokens = req.Thinking.BudgetTokens
		}
		chatReq.Reasoning = reasoning
	}

	return json.Marshal(chatReq)
}

func (p *Provider) doRequest(ctx context.Context, body []byte, streaming bool) (io.ReadCloser, error) {
	url := p.config.BaseURL + defaultChatEndpoint

	client := p.httpClient
	if streaming {
		client = p.streamHttpClient
	}

	var respBody io.ReadCloser

	policy := &backoff.ExponentialBackoffPolicy{
		InitialInterval: p.config.InitialInterval,
		BackoffFactor:   p.config.Multiplier,
		MaxInterval:     p.config.MaxInterval,
		MaxRetries:      p.config.MaxRetries,
	}

	err := backoff.Retry(ctx, func(ctx context.Context) error {
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			return backoff.PermanentError(llm.WrapError(providerName, fmt.Errorf("failed to create request: %w", err)))
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

		// OpenRouter-specific headers
		if p.SiteURL != "" {
			httpReq.Header.Set("HTTP-Referer", p.SiteURL)
		}
		if p.SiteName != "" {
			httpReq.Header.Set("X-Title", p.SiteName)
		}

		resp, err := client.Do(httpReq)
		if err != nil {
			return err // Retriable
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			respBody = resp.Body
			return nil
		}

		// Read error response
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		apiErr := p.parseErrorResponse(resp.StatusCode, errBody)
		if !apiErr.Retryable {
			return backoff.PermanentError(apiErr)
		}
		return apiErr
	}, policy, nil)

	if err != nil {
		return nil, err
	}
	return respBody, nil
}

func (p *Provider) parseErrorResponse(statusCode int, body []byte) *llm.APIError {
	var errResp errorResponse
	message := string(body)
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error.Message != "" {
		message = errResp.Error.Message
	}

	return llm.NewAPIError(providerName, statusCode, message)
}

func (p *Provider) streamResponse(ctx context.Context, body io.ReadCloser, events chan<- llm.StreamEvent) {
	defer close(events)
	defer func() { _ = body.Close() }()

	scanner := bufio.NewScanner(body)
	var usage *llm.Usage

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			events <- llm.StreamEvent{Error: ctx.Err(), Done: true}
			return
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, streamPrefix) {
			continue
		}

		data := strings.TrimPrefix(line, streamPrefix)
		if data == streamDoneMarker {
			events <- llm.StreamEvent{Done: true, Usage: usage}
			return
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Capture usage if present
		if chunk.Usage != nil {
			usage = &llm.Usage{
				PromptTokens:     chunk.Usage.PromptTokens,
				CompletionTokens: chunk.Usage.CompletionTokens,
				TotalTokens:      chunk.Usage.TotalTokens,
			}
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			events <- llm.StreamEvent{Delta: chunk.Choices[0].Delta.Content}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- llm.StreamEvent{Error: llm.WrapError(providerName, err), Done: true}
		return
	}

	// If we get here without [DONE], still signal completion
	events <- llm.StreamEvent{Done: true, Usage: usage}
}

// API request/response types (OpenAI-compatible)

type message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type chatCompletionRequest struct {
	Model       string            `json:"model"`
	Messages    []message         `json:"messages"`
	Temperature *float64          `json:"temperature,omitempty"`
	MaxTokens   *int              `json:"max_tokens,omitempty"`
	TopP        *float64          `json:"top_p,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
	Reasoning   *reasoningRequest `json:"reasoning,omitempty"`
}

// reasoningRequest represents OpenRouter's unified reasoning configuration.
type reasoningRequest struct {
	Effort    string `json:"effort,omitempty"`
	MaxTokens *int   `json:"max_tokens,omitempty"`
}

type chatCompletionResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int     `json:"index"`
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

type streamChunk struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason,omitempty"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
}

type errorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}
