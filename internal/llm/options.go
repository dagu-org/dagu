package llm

import "time"

// Option is a functional option for configuring an LLM provider.
type Option func(*Config)

// WithAPIKey sets the API key for the provider.
func WithAPIKey(apiKey string) Option {
	return func(c *Config) {
		c.APIKey = apiKey
	}
}

// WithBaseURL sets the base URL for the provider.
func WithBaseURL(baseURL string) Option {
	return func(c *Config) {
		c.BaseURL = baseURL
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithMaxRetries sets the maximum number of retry attempts.
func WithMaxRetries(maxRetries int) Option {
	return func(c *Config) {
		c.MaxRetries = maxRetries
	}
}

// WithBackoff sets the backoff configuration for retries.
func WithBackoff(initial, max time.Duration, multiplier float64) Option {
	return func(c *Config) {
		c.InitialInterval = initial
		c.MaxInterval = max
		c.Multiplier = multiplier
	}
}

// ApplyOptions applies the given options to a Config.
func ApplyOptions(cfg *Config, opts ...Option) {
	for _, opt := range opts {
		opt(cfg)
	}
}

// NewConfig creates a new Config with the given options applied.
func NewConfig(opts ...Option) Config {
	cfg := DefaultConfig()
	ApplyOptions(&cfg, opts...)
	return cfg
}

// RequestOption is a functional option for configuring a ChatRequest.
type RequestOption func(*ChatRequest)

// WithTemperature sets the temperature for the request.
func WithTemperature(temp float64) RequestOption {
	return func(r *ChatRequest) {
		r.Temperature = &temp
	}
}

// WithMaxTokens sets the maximum tokens for the request.
func WithMaxTokens(tokens int) RequestOption {
	return func(r *ChatRequest) {
		r.MaxTokens = &tokens
	}
}

// WithTopP sets the top_p (nucleus sampling) parameter.
func WithTopP(topP float64) RequestOption {
	return func(r *ChatRequest) {
		r.TopP = &topP
	}
}

// WithStop sets the stop sequences for the request.
func WithStop(stop ...string) RequestOption {
	return func(r *ChatRequest) {
		r.Stop = stop
	}
}

// ApplyRequestOptions applies the given options to a ChatRequest.
func ApplyRequestOptions(req *ChatRequest, opts ...RequestOption) {
	for _, opt := range opts {
		opt(req)
	}
}

// NewChatRequest creates a new ChatRequest with the given model, messages, and options.
func NewChatRequest(model string, messages []Message, opts ...RequestOption) *ChatRequest {
	req := &ChatRequest{
		Model:    model,
		Messages: messages,
	}
	ApplyRequestOptions(req, opts...)
	return req
}
