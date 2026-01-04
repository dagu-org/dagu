package llm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfigOptions(t *testing.T) {
	t.Parallel()

	cfg := NewConfig(
		WithAPIKey("test-key"),
		WithBaseURL("https://example.com"),
		WithTimeout(5*time.Minute),
		WithMaxRetries(5),
		WithBackoff(2*time.Second, 1*time.Minute, 3.0),
	)

	assert.Equal(t, "test-key", cfg.APIKey)
	assert.Equal(t, "https://example.com", cfg.BaseURL)
	assert.Equal(t, 5*time.Minute, cfg.Timeout)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 2*time.Second, cfg.InitialInterval)
	assert.Equal(t, 1*time.Minute, cfg.MaxInterval)
	assert.Equal(t, 3.0, cfg.Multiplier)
}

func TestRequestOptions(t *testing.T) {
	t.Parallel()

	req := NewChatRequest("gpt-4", []Message{{Role: RoleUser, Content: "hi"}},
		WithTemperature(0.7),
		WithMaxTokens(100),
		WithTopP(0.9),
		WithStop("END", "STOP"),
	)

	assert.Equal(t, "gpt-4", req.Model)
	assert.Len(t, req.Messages, 1)
	assert.Equal(t, 0.7, *req.Temperature)
	assert.Equal(t, 100, *req.MaxTokens)
	assert.Equal(t, 0.9, *req.TopP)
	assert.Equal(t, []string{"END", "STOP"}, req.Stop)
}
