package api_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/api/v2"
	apiimpl "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
	"github.com/dagu-org/dagu/internal/test"
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

func TestValidateWebhookToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		provided string
		expected string
		want     bool
	}{
		{
			name:     "matching tokens",
			provided: "my-secret-token",
			expected: "my-secret-token",
			want:     true,
		},
		{
			name:     "non-matching tokens",
			provided: "wrong-token",
			expected: "my-secret-token",
			want:     false,
		},
		{
			name:     "empty provided",
			provided: "",
			expected: "my-secret-token",
			want:     false,
		},
		{
			name:     "empty expected",
			provided: "my-token",
			expected: "",
			want:     false,
		},
		{
			name:     "both empty",
			provided: "",
			expected: "",
			want:     false,
		},
		{
			name:     "tokens with special characters",
			provided: "token-with-$pecial-ch@rs!",
			expected: "token-with-$pecial-ch@rs!",
			want:     true,
		},
		{
			name:     "very long matching tokens",
			provided: "a-very-long-token-that-should-still-match-correctly-even-with-many-characters",
			expected: "a-very-long-token-that-should-still-match-correctly-even-with-many-characters",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := apiimpl.ValidateWebhookToken(tt.provided, tt.expected)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestWebhookAPI(t *testing.T) {
	server := test.SetupServer(t)

	t.Run("TriggerWebhookSuccess", func(t *testing.T) {
		spec := `
webhook:
  enabled: true
  token: my-secret-token-1234
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-test-dag",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		resp := server.Client().Post("/api/v2/webhooks/webhook-test-dag", api.WebhookRequest{
			Payload: &map[string]any{"event": "test"},
		}).WithHeader("Authorization", "Bearer my-secret-token-1234").
			ExpectStatus(http.StatusOK).Send(t)

		var webhookResp api.WebhookResponse
		resp.Unmarshal(t, &webhookResp)
		require.NotEmpty(t, webhookResp.DagRunId)

		_ = server.Client().Delete("/api/v2/dags/webhook-test-dag").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("TriggerWebhookInvalidToken", func(t *testing.T) {
		spec := `
webhook:
  enabled: true
  token: correct-token-12345
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-invalid-token",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		_ = server.Client().Post("/api/v2/webhooks/webhook-invalid-token", api.WebhookRequest{}).
			WithHeader("Authorization", "Bearer wrong-token-12345").
			ExpectStatus(http.StatusUnauthorized).Send(t)

		_ = server.Client().Delete("/api/v2/dags/webhook-invalid-token").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("TriggerWebhookNotEnabled", func(t *testing.T) {
		spec := `
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-not-enabled",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		_ = server.Client().Post("/api/v2/webhooks/webhook-not-enabled", api.WebhookRequest{}).
			WithHeader("Authorization", "Bearer any-token").
			ExpectStatus(http.StatusForbidden).Send(t)

		_ = server.Client().Delete("/api/v2/dags/webhook-not-enabled").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("TriggerWebhookNotFound", func(t *testing.T) {
		_ = server.Client().Post("/api/v2/webhooks/non-existent-dag", api.WebhookRequest{}).
			WithHeader("Authorization", "Bearer any-token").
			ExpectStatus(http.StatusNotFound).Send(t)
	})

	t.Run("TriggerWebhookIdempotency", func(t *testing.T) {
		spec := `
webhook:
  enabled: true
  token: idempotency-token-1
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-idempotency",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		dagRunID := "my-unique-run-id-12345"
		resp := server.Client().Post("/api/v2/webhooks/webhook-idempotency", api.WebhookRequest{
			DagRunId: &dagRunID,
			Payload:  &map[string]any{"event": "first"},
		}).WithHeader("Authorization", "Bearer idempotency-token-1").
			ExpectStatus(http.StatusOK).Send(t)

		var webhookResp api.WebhookResponse
		resp.Unmarshal(t, &webhookResp)
		require.Equal(t, dagRunID, webhookResp.DagRunId)

		// Second call with same dagRunId should return 409 Conflict
		_ = server.Client().Post("/api/v2/webhooks/webhook-idempotency", api.WebhookRequest{
			DagRunId: &dagRunID,
			Payload:  &map[string]any{"event": "second"},
		}).WithHeader("Authorization", "Bearer idempotency-token-1").
			ExpectStatus(http.StatusConflict).Send(t)

		_ = server.Client().Delete("/api/v2/dags/webhook-idempotency").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("TriggerWebhookMissingAuth", func(t *testing.T) {
		spec := `
webhook:
  enabled: true
  token: auth-token-123456
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-missing-auth",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// When Authorization header is missing (required by OpenAPI spec), returns 400 Bad Request
		_ = server.Client().Post("/api/v2/webhooks/webhook-missing-auth", api.WebhookRequest{}).
			ExpectStatus(http.StatusBadRequest).Send(t)

		_ = server.Client().Delete("/api/v2/dags/webhook-missing-auth").ExpectStatus(http.StatusNoContent).Send(t)
	})

	t.Run("TriggerWebhookPayloadTooLarge", func(t *testing.T) {
		spec := `
webhook:
  enabled: true
  token: large-payload-token1
steps:
  - name: test
    command: echo "hello"
`
		_ = server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
			Name: "webhook-large-payload",
			Spec: &spec,
		}).ExpectStatus(http.StatusCreated).Send(t)

		// Create a payload larger than 1MB
		largeData := strings.Repeat("x", 1024*1024+1)
		_ = server.Client().Post("/api/v2/webhooks/webhook-large-payload", api.WebhookRequest{
			Payload: &map[string]any{"data": largeData},
		}).WithHeader("Authorization", "Bearer large-payload-token1").
			ExpectStatus(http.StatusRequestEntityTooLarge).Send(t)

		_ = server.Client().Delete("/api/v2/dags/webhook-large-payload").ExpectStatus(http.StatusNoContent).Send(t)
	})
}
