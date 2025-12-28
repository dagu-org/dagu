package api_test

import (
	"testing"

	apiimpl "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
	"github.com/stretchr/testify/require"
)

func TestExtractWebhookToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		authHeader string
		want       string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer my-secret-token",
			want:       "my-secret-token",
		},
		{
			name:       "empty header",
			authHeader: "",
			want:       "",
		},
		{
			name:       "no bearer prefix",
			authHeader: "my-secret-token",
			want:       "",
		},
		{
			name:       "wrong prefix",
			authHeader: "Basic my-secret-token",
			want:       "",
		},
		{
			name:       "bearer with extra spaces",
			authHeader: "Bearer  token-with-space",
			want:       " token-with-space",
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer my-token",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiimpl.ExtractWebhookToken(tt.authHeader)
			require.Equal(t, tt.want, got)
		})
	}
}

// Note: Webhook API integration tests require builtin auth mode with webhook store.
// These tests are covered in the integration test suite where the full auth
// infrastructure is available. The webhook management endpoints (create, delete,
// regenerate, toggle) require admin authentication, and the trigger endpoint
// requires a valid webhook token from the store.
