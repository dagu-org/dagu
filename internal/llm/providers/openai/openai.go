// Package openai provides an LLM provider implementation for OpenAI's API.
package openai

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	providerName        = "openai"
	defaultChatEndpoint = "/chat/completions"
	streamPrefix        = "data: "
	streamDoneMarker    = "[DONE]"
)

func init() {
	llm.RegisterProvider(llm.ProviderOpenAI, New)
}

// Provider implements the llm.Provider interface for OpenAI.
var _ llm.Provider = (*Provider)(nil)

type Provider struct {
	config     llm.Config
	httpClient *llm.HTTPClient
}

// New creates a new OpenAI provider.
func New(cfg llm.Config) (llm.Provider, error) {
	if cfg.APIKey == "" {
		return nil, llm.ErrNoAPIKey
	}

	return &Provider{
		config:     cfg,
		httpClient: llm.NewHTTPClient(cfg),
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

	respBody, err := p.doRequest(ctx, body)
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
	if req.Thinking != nil && req.Thinking.Enabled {
		effort := req.Thinking.Effort
		if effort == "" {
			effort = llm.ThinkingEffortMedium
		}
		chatReq.Reasoning = &reasoningRequest{Effort: string(effort)}

		// For reasoning models, use max_completion_tokens instead of max_tokens
		if req.MaxTokens != nil {
			chatReq.MaxCompletionTokens = req.MaxTokens
			chatReq.MaxTokens = nil
		}

		// Reasoning models don't support temperature and topP
		chatReq.Temperature = nil
		chatReq.TopP = nil
	}

	// Include usage in stream for final event
	if stream {
		chatReq.StreamOptions = &streamOptions{IncludeUsage: true}
	}

	return json.Marshal(chatReq)
}

func (p *Provider) doRequest(ctx context.Context, body []byte) (io.ReadCloser, error) {
	return p.httpClient.Do(ctx, p.config.BaseURL+defaultChatEndpoint, body, p.authHeaders())
}

func (p *Provider) authHeaders() map[string]string {
	return map[string]string{
		"Authorization": "Bearer " + p.config.APIKey,
	}
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

// API request/response types

type message struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	Name       string `json:"name,omitempty"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type chatCompletionRequest struct {
	Model               string            `json:"model"`
	Messages            []message         `json:"messages"`
	Temperature         *float64          `json:"temperature,omitempty"`
	MaxTokens           *int              `json:"max_tokens,omitempty"`
	MaxCompletionTokens *int              `json:"max_completion_tokens,omitempty"`
	TopP                *float64          `json:"top_p,omitempty"`
	Stop                []string          `json:"stop,omitempty"`
	Stream              bool              `json:"stream,omitempty"`
	StreamOptions       *streamOptions    `json:"stream_options,omitempty"`
	Reasoning           *reasoningRequest `json:"reasoning,omitempty"`
}

// reasoningRequest represents OpenAI's reasoning configuration for o1/o3/GPT-5 models.
type reasoningRequest struct {
	Effort string `json:"effort"`
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
