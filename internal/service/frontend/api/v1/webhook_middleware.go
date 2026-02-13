package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
)

// contextKey is a private type for context keys in this package.
type contextKey string

const (
	// rawBodyContextKey stores the raw request body bytes in context.
	rawBodyContextKey contextKey = "webhook_raw_body"
)

// WebhookRawBodyMiddleware is a chi middleware that captures the raw request
// body into the request context before the generated strict handler consumes it.
// This allows the webhook handler to fall back to the full raw body when the
// structured "payload" field is not present in the request.
//
// The middleware only activates for POST requests to webhook trigger paths
// (paths containing "/webhooks/" with a fileName segment after it).
func WebhookRawBodyMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost || !isWebhookTriggerPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			if r.Body == nil || r.Body == http.NoBody {
				next.ServeHTTP(w, r)
				return
			}

			// Read the body with the same size limit used for payload validation.
			rawBody, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookPayloadSize+1))
			if err != nil {
				http.Error(w, "failed to read request body", http.StatusBadRequest)
				return
			}

			// Replace r.Body so the generated code can still read it.
			r.Body = io.NopCloser(bytes.NewReader(rawBody))

			ctx := context.WithValue(r.Context(), rawBodyContextKey, rawBody)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// rawBodyFromContext retrieves the raw request body bytes stored by
// WebhookRawBodyMiddleware. Returns nil if not present.
func rawBodyFromContext(ctx context.Context) []byte {
	raw, _ := ctx.Value(rawBodyContextKey).([]byte)
	return raw
}

// withRawBody returns a context with the raw body attached (for testing).
func withRawBody(ctx context.Context, body []byte) context.Context {
	return context.WithValue(ctx, rawBodyContextKey, body)
}

// isWebhookTriggerPath checks if the URL path matches the webhook trigger
// endpoint pattern (POST /api/v1/webhooks/{fileName}). It looks for "/webhooks/"
// in the path with a non-empty segment after it.
func isWebhookTriggerPath(urlPath string) bool {
	path := strings.TrimRight(urlPath, "/")
	idx := strings.Index(path, "/webhooks/")
	if idx < 0 {
		return false
	}
	after := path[idx+len("/webhooks/"):]
	return after != ""
}
