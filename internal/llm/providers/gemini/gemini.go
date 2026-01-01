// Package gemini provides an LLM provider implementation for Google's Gemini API.
package gemini

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/llm"
)

const (
	providerName = "gemini"
	// Endpoint format: /models/{model}:generateContent
	generateContentPath = "/models/%s:generateContent"
	streamContentPath   = "/models/%s:streamGenerateContent"
)

func init() {
	llm.RegisterProvider(llm.ProviderGemini, New)
}

// Provider implements the llm.Provider interface for Google Gemini.
type Provider struct {
	config     llm.Config
	httpClient *http.Client
}

// New creates a new Gemini provider.
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
	body, err := p.buildRequestBody(req)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(generateContentPath, req.Model)
	respBody, err := p.doRequest(ctx, endpoint, body)
	if err != nil {
		return nil, err
	}
	defer func() { _ = respBody.Close() }()

	var resp generateContentResponse
	if err := json.NewDecoder(respBody).Decode(&resp); err != nil {
		return nil, llm.WrapError(providerName, fmt.Errorf("failed to decode response: %w", err))
	}

	// Extract content from response
	var content string
	var finishReason string
	if len(resp.Candidates) > 0 {
		candidate := resp.Candidates[0]
		finishReason = candidate.FinishReason
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				content += part.Text
			}
		}
	}

	// Calculate usage
	var usage llm.Usage
	if resp.UsageMetadata != nil {
		usage = llm.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
	}

	return &llm.ChatResponse{
		Content:      content,
		FinishReason: finishReason,
		Usage:        usage,
	}, nil
}

// ChatStream sends messages and streams the response.
func (p *Provider) ChatStream(ctx context.Context, req *llm.ChatRequest) (<-chan llm.StreamEvent, error) {
	body, err := p.buildRequestBody(req)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf(streamContentPath, req.Model)
	respBody, err := p.doRequest(ctx, endpoint+"?alt=sse", body)
	if err != nil {
		return nil, err
	}

	events := make(chan llm.StreamEvent)
	go p.streamResponse(ctx, respBody, events)

	return events, nil
}

func (p *Provider) buildRequestBody(req *llm.ChatRequest) ([]byte, error) {
	// Separate system instruction from other messages
	var sysInstr *systemInstruction
	contents := make([]content, 0, len(req.Messages))

	for _, m := range req.Messages {
		switch m.Role {
		case llm.RoleSystem:
			if sysInstr == nil {
				sysInstr = &systemInstruction{}
			}
			sysInstr.Parts = append(sysInstr.Parts, part{Text: m.Content})

		case llm.RoleUser:
			contents = append(contents, content{
				Role:  "user",
				Parts: []part{{Text: m.Content}},
			})

		case llm.RoleAssistant:
			contents = append(contents, content{
				Role:  "model", // Gemini uses "model" instead of "assistant"
				Parts: []part{{Text: m.Content}},
			})

		case llm.RoleTool:
			// Gemini uses "function" role for function/tool results.
			// For basic chat, we include tool results as user messages.
			contents = append(contents, content{
				Role:  "user",
				Parts: []part{{Text: m.Content}},
			})
		}
	}

	geminiReq := generateContentRequest{
		Contents: contents,
	}

	if sysInstr != nil {
		geminiReq.SystemInstruction = sysInstr
	}

	// Build generation config
	genConfig := &generationConfig{}
	hasConfig := false

	if req.Temperature != nil {
		genConfig.Temperature = req.Temperature
		hasConfig = true
	}
	if req.MaxTokens != nil {
		genConfig.MaxOutputTokens = req.MaxTokens
		hasConfig = true
	}
	if req.TopP != nil {
		genConfig.TopP = req.TopP
		hasConfig = true
	}
	if len(req.Stop) > 0 {
		genConfig.StopSequences = req.Stop
		hasConfig = true
	}

	if hasConfig {
		geminiReq.GenerationConfig = genConfig
	}

	return json.Marshal(geminiReq)
}

func (p *Provider) doRequest(ctx context.Context, endpoint string, body []byte) (io.ReadCloser, error) {
	// Add API key to URL
	url := p.config.BaseURL + endpoint
	if strings.Contains(url, "?") {
		url += "&key=" + p.config.APIKey
	} else {
		url += "?key=" + p.config.APIKey
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, llm.WrapError(providerName, fmt.Errorf("failed to create request: %w", err))
	}

	httpReq.Header.Set("Content-Type", "application/json")

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

		// Gemini streaming uses SSE format with "data: " prefix
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")

		var chunk generateContentResponse
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		// Capture usage if present
		if chunk.UsageMetadata != nil {
			usage = &llm.Usage{
				PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
				CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
			}
		}

		// Extract text content
		if len(chunk.Candidates) > 0 {
			candidate := chunk.Candidates[0]
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					events <- llm.StreamEvent{Delta: part.Text}
				}
			}

			// Check for completion
			if candidate.FinishReason != "" && candidate.FinishReason != "UNSPECIFIED" {
				events <- llm.StreamEvent{Done: true, Usage: usage}
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		events <- llm.StreamEvent{Error: llm.WrapError(providerName, err), Done: true}
		return
	}

	// Signal completion
	events <- llm.StreamEvent{Done: true, Usage: usage}
}

// API request/response types

type part struct {
	Text string `json:"text,omitempty"`
}

type content struct {
	Role  string `json:"role"`
	Parts []part `json:"parts"`
}

type systemInstruction struct {
	Parts []part `json:"parts"`
}

type generationConfig struct {
	Temperature     *float64 `json:"temperature,omitempty"`
	MaxOutputTokens *int     `json:"maxOutputTokens,omitempty"`
	TopP            *float64 `json:"topP,omitempty"`
	StopSequences   []string `json:"stopSequences,omitempty"`
}

type generateContentRequest struct {
	Contents          []content          `json:"contents"`
	SystemInstruction *systemInstruction `json:"systemInstruction,omitempty"`
	GenerationConfig  *generationConfig  `json:"generationConfig,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []part `json:"parts"`
			Role  string `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		Index         int    `json:"index"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata *struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

type errorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error"`
}
