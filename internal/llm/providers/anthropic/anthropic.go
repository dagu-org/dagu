// Package anthropic provides an LLM provider implementation for Anthropic's Claude API.
package anthropic

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	providerName        = "anthropic"
	defaultMessagesPath = "/v1/messages"
	anthropicAPIVersion = "2023-06-01"
	streamPrefix        = "data: "
)

func init() {
	llm.RegisterProvider(llm.ProviderAnthropic, New)
}

// Provider implements the llm.Provider interface for Anthropic Claude.
type Provider struct {
	config     llm.Config
	httpClient *http.Client
}

// New creates a new Anthropic provider.
func New(cfg llm.Config) (llm.Provider, error) {
	if cfg.APIKey == "" {
		return nil, llm.ErrNoAPIKey
	}

	return &Provider{
		config: cfg,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
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

	respBody, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = respBody.Close() }()

	var resp messagesResponse
	if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
		return nil, llm.WrapError(providerName, fmt.Errorf("failed to decode response: %w", err))
	}

	// Extract content from response
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return &llm.ChatResponse{
		Content:      content,
		FinishReason: resp.StopReason,
		Usage: llm.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}, nil
}

// ChatStream sends messages and streams the response.
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	body, err := p.buildRequestBody(req, true)
	if err != nil {
		return nil, err
	}

	respBody, err := p.doRequest(ctx, body)
	if err != nil {
		return nil, err
	}

	events := make(chan llm.StreamEvent)
	go p.streamResponse(ctx, respBody, events)

	return events, nil
}

func (p *Provider) buildRequestBody(req *llm.ChatRequest, stream bool) ([]byte, error) {
	// Anthropic separates system message from other messages
	var systemContent string
	messages := make([]message, 0, len(req.Messages))

	for _, m := range req.Messages {
		switch m.Role {
		case llm.RoleSystem:
			// Concatenate system messages
			if systemContent != "" {
				systemContent += "\n\n"
			}
			systemContent += m.Content
		case llm.RoleUser, llm.RoleAssistant:
			messages = append(messages, message{
				Role:    string(m.Role),
				Content: m.Content,
			})
		case llm.RoleTool:
			// Tool results in Anthropic are sent as user messages with tool_result content blocks.
			// For basic chat, we include tool results as user messages.
			messages = append(messages, message{
				Role:    "user",
				Content: m.Content,
			})
		}
	}

	// Anthropic requires at least one user message
	if len(messages) == 0 {
		return nil, llm.WrapError(providerName, fmt.Errorf("at least one user message is required"))
	}

	chatReq := messagesRequest{
		Model:    req.Model,
		Messages: messages,
		Stream:   stream,
	}

	// Set system message if present
	if systemContent != "" {
		chatReq.System = systemContent
	}

	// Set max_tokens (required by Anthropic)
	if req.MaxTokens != nil {
		chatReq.MaxTokens = *req.MaxTokens
	} else {
		// Default to 4096 if not specified
		chatReq.MaxTokens = 4096
	}

	if req.Temperature != nil {
		chatReq.Temperature = req.Temperature
	}
	if req.TopP != nil {
		chatReq.TopP = req.TopP
	}
	if len(req.Stop) > 0 {
		chatReq.StopSequences = req.Stop
	}

	return json.Marshal(chatReq)
}

func (p *Provider) doRequest(ctx context.Context, body []byte) (io.ReadCloser, error) {
	url := p.config.BaseURL + defaultMessagesPath

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, llm.WrapError(providerName, fmt.Errorf("failed to create request: %w", err))
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	var resp *http.Response
	var lastErr error

	// Retry loop with backoff
	interval := p.config.InitialInterval
	for attempt := 0; attempt <= p.config.MaxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(interval):
			}
			interval = time.Duration(float64(interval) * p.config.Multiplier)
			if interval > p.config.MaxInterval {
				interval = p.config.MaxInterval
			}
			// Recreate request body reader
			httpReq.Body = io.NopCloser(bytes.NewReader(body))
		}

		resp, lastErr = p.httpClient.Do(httpReq)
		if lastErr != nil {
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp.Body, nil
		}

		// Read error response
		errBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		apiErr := p.parseErrorResponse(resp.StatusCode, errBody)
		if !apiErr.Retryable {
			return nil, apiErr
		}
		lastErr = apiErr
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, llm.WrapError(providerName, fmt.Errorf("request failed after %d retries", p.config.MaxRetries))
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

		// Anthropic uses event/data format
		if !strings.HasPrefix(line, streamPrefix) {
			continue
		}

		data := strings.TrimPrefix(line, streamPrefix)

		var event streamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta != nil && event.Delta.Type == "text_delta" {
				events <- llm.StreamEvent{Delta: event.Delta.Text}
			}

		case "message_delta":
			// Message completed, may contain usage
			if event.Usage != nil {
				usage = &llm.Usage{
					PromptTokens:     event.Usage.InputTokens,
					CompletionTokens: event.Usage.OutputTokens,
					TotalTokens:      event.Usage.InputTokens + event.Usage.OutputTokens,
				}
			}

		case "message_start":
			// May contain initial usage (input tokens)
			if event.Message != nil && event.Message.Usage != nil {
				usage = &llm.Usage{
					PromptTokens: event.Message.Usage.InputTokens,
				}
			}

		case "message_stop":
			events <- llm.StreamEvent{Done: true, Usage: usage}
			return

		case "error":
			errMsg := "unknown streaming error"
			if event.Error != nil {
				errMsg = event.Error.Message
			}
			events <- llm.StreamEvent{
				Error: llm.WrapError(providerName, errors.New(errMsg)),
				Done:  true,
			}
			return
		}
	}

	if err := scanner.Err(); err != nil {
		events <- llm.StreamEvent{Error: llm.WrapError(providerName, err), Done: true}
		return
	}

	// If we get here without message_stop, still signal completion
	events <- llm.StreamEvent{Done: true, Usage: usage}
}

// API request/response types

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type messagesRequest struct {
	Model         string    `json:"model"`
	Messages      []message `json:"messages"`
	System        string    `json:"system,omitempty"`
	MaxTokens     int       `json:"max_tokens"`
	Temperature   *float64  `json:"temperature,omitempty"`
	TopP          *float64  `json:"top_p,omitempty"`
	StopSequences []string  `json:"stop_sequences,omitempty"`
	Stream        bool      `json:"stream,omitempty"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type messagesResponse struct {
	ID           string         `json:"id"`
	Type         string         `json:"type"`
	Role         string         `json:"role"`
	Content      []contentBlock `json:"content"`
	Model        string         `json:"model"`
	StopReason   string         `json:"stop_reason"`
	StopSequence string         `json:"stop_sequence,omitempty"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type streamEvent struct {
	Type    string `json:"type"`
	Index   int    `json:"index,omitempty"`
	Message *struct {
		ID    string `json:"id"`
		Type  string `json:"type"`
		Role  string `json:"role"`
		Usage *struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage,omitempty"`
	} `json:"message,omitempty"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text,omitempty"`
	} `json:"delta,omitempty"`
	Usage *struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type errorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}
