package llm

import (
	"context"
	"io"

	"github.com/go-resty/resty/v2"
)

// HTTPClient performs HTTP requests with retry logic using resty.
type HTTPClient struct {
	client *resty.Client
}

// NewHTTPClient creates a new HTTP client with the given configuration.
func NewHTTPClient(cfg Config) *HTTPClient {
	client := resty.New().
		SetTimeout(cfg.Timeout).
		SetRetryCount(cfg.MaxRetries).
		SetRetryWaitTime(cfg.InitialInterval).
		SetRetryMaxWaitTime(cfg.MaxInterval).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true // Retry on network errors
			}
			// Retry on rate limit and server errors
			code := r.StatusCode()
			return code == 429 || (code >= 500 && code <= 504)
		})

	return &HTTPClient{client: client}
}

// Do performs an HTTP POST request with the configured retry policy.
// Returns the response body as an io.ReadCloser for streaming support.
func (c *HTTPClient) Do(ctx context.Context, url string, body []byte, headers map[string]string) (io.ReadCloser, error) {
	req := c.client.R().
		SetContext(ctx).
		SetHeaders(headers).
		SetBody(body).
		SetDoNotParseResponse(true) // Return raw response for streaming

	resp, err := req.Post(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode() >= 200 && resp.StatusCode() < 300 {
		return resp.RawBody(), nil
	}

	// Read error body and close
	defer func() { _ = resp.RawBody().Close() }()
	errBody, _ := io.ReadAll(resp.RawBody())

	return nil, NewAPIError("llm", resp.StatusCode(), string(errBody))
}
