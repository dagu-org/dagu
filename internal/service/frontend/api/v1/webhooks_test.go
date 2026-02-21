package api_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	apiimpl "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
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
			t.Parallel()
			got := apiimpl.ExtractWebhookToken(tt.authHeader)
			require.Equal(t, tt.want, got)
		})
	}
}

// setupWebhookTestServer creates a test server with builtin auth enabled
func setupWebhookTestServer(t *testing.T, extraMutators ...func(*config.Config)) test.Server {
	t.Helper()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		for _, m := range extraMutators {
			m(cfg)
		}
	}))

	// Create admin via setup endpoint
	server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	return server
}

// getWebhookAdminToken authenticates as admin and returns the JWT token
func getWebhookAdminToken(t *testing.T, server test.Server) string {
	t.Helper()
	resp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)

	var loginResult api.LoginResponse
	resp.Unmarshal(t, &loginResult)
	require.NotEmpty(t, loginResult.Token)
	return loginResult.Token
}

// createTestDAG creates a simple DAG for webhook testing
func createTestDAG(t *testing.T, server test.Server, token, name string) {
	t.Helper()
	spec := `
steps:
  - name: test
    command: echo hello
`
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: name,
		Spec: &spec,
	}).WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)
}

// TestWebhooks_ListEmpty tests listing webhooks when none exist
func TestWebhooks_ListEmpty(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	resp := server.Client().Get("/api/v1/webhooks").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var result api.WebhookListResponse
	resp.Unmarshal(t, &result)
	assert.Empty(t, result.Webhooks, "expected no webhooks")
}

// TestWebhooks_RequiresAuth tests that webhook management endpoints require authentication
func TestWebhooks_RequiresAuth(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)

	// Without auth - should fail
	server.Client().Get("/api/v1/webhooks").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	server.Client().Post("/api/v1/dags/test-dag/webhook", nil).
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestWebhooks_RequiresDeveloperOrAbove tests that webhook management requires developer or above.
func TestWebhooks_RequiresDeveloperOrAbove(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	// Create non-developer users.
	server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "operator-user",
		Password: "operator1",
		Role:     api.UserRoleOperator,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "viewer-user",
		Password: "viewerpass1",
		Role:     api.UserRoleViewer,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	// Create developer user.
	server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "developer-user",
		Password: "developer1",
		Role:     api.UserRoleDeveloper,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	// Login as operator.
	operatorResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "operator-user",
		Password: "operator1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var operatorLogin api.LoginResponse
	operatorResp.Unmarshal(t, &operatorLogin)

	// Login as viewer.
	viewerResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "viewer-user",
		Password: "viewerpass1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var viewerLogin api.LoginResponse
	viewerResp.Unmarshal(t, &viewerLogin)

	// Login as developer.
	developerResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "developer-user",
		Password: "developer1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var developerLogin api.LoginResponse
	developerResp.Unmarshal(t, &developerLogin)

	// Operator and viewer should get forbidden for webhook management.
	server.Client().Get("/api/v1/webhooks").
		WithBearerToken(operatorLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Post("/api/v1/dags/test-dag/webhook", nil).
		WithBearerToken(operatorLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Get("/api/v1/webhooks").
		WithBearerToken(viewerLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Post("/api/v1/dags/test-dag/webhook", nil).
		WithBearerToken(viewerLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)

	// Developer can access webhook management endpoints.
	server.Client().Get("/api/v1/webhooks").
		WithBearerToken(developerLogin.Token).
		ExpectStatus(http.StatusOK).Send(t)

	// Developer can also create webhooks.
	dagName := "webhook_dev_access_test"
	createTestDAG(t, server, adminToken, dagName)
	server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(developerLogin.Token).
		ExpectStatus(http.StatusCreated).Send(t)
}

// TestWebhooks_CRUD tests the full CRUD lifecycle of webhooks
func TestWebhooks_CRUD(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG first
	dagName := "webhook_crud_test"
	createTestDAG(t, server, token, dagName)

	// Create a webhook
	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)

	assert.NotEmpty(t, createResult.Token, "expected full token to be returned")
	assert.NotEmpty(t, createResult.Webhook.Id, "expected webhook ID")
	assert.Equal(t, dagName, createResult.Webhook.DagName)
	assert.True(t, createResult.Webhook.Enabled)
	assert.NotEmpty(t, createResult.Webhook.TokenPrefix, "expected token prefix")
	assert.Contains(t, createResult.Token, "dagu_wh_", "token should have webhook prefix")

	webhookToken := createResult.Token

	// List webhooks
	listResp := server.Client().Get("/api/v1/webhooks").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var listResult api.WebhookListResponse
	listResp.Unmarshal(t, &listResult)
	assert.Len(t, listResult.Webhooks, 1)
	assert.Equal(t, dagName, listResult.Webhooks[0].DagName)

	// Get specific webhook
	getResp := server.Client().Get("/api/v1/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var getResult api.WebhookDetails
	getResp.Unmarshal(t, &getResult)
	assert.Equal(t, createResult.Webhook.Id, getResult.Id)
	assert.Equal(t, dagName, getResult.DagName)

	// Trigger the webhook
	triggerResp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.NotEmpty(t, triggerResult.DagRunId)
	assert.Equal(t, dagName, triggerResult.DagName)

	// Verify LastUsedAt is updated
	require.Eventually(t, func() bool {
		resp := server.Client().Get("/api/v1/dags/" + dagName + "/webhook").
			WithBearerToken(token).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}
		var result api.WebhookDetails
		resp.Unmarshal(t, &result)
		return result.LastUsedAt != nil
	}, 5*time.Second, 100*time.Millisecond, "LastUsedAt should be set after trigger")

	// Delete webhook
	server.Client().Delete("/api/v1/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNoContent).Send(t)

	// Verify it's deleted
	server.Client().Get("/api/v1/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNotFound).Send(t)

	// Verify webhook token no longer works
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestWebhooks_Toggle tests enabling and disabling webhooks
func TestWebhooks_Toggle(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_toggle_test"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Disable the webhook
	enabled := false
	toggleResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook/toggle", api.WebhookToggleRequest{
		Enabled: enabled,
	}).WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	var toggleResult api.WebhookDetails
	toggleResp.Unmarshal(t, &toggleResult)
	assert.False(t, toggleResult.Enabled)

	// Try to trigger - should fail with forbidden
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusForbidden).Send(t)

	// Re-enable the webhook
	enabled = true
	server.Client().Post("/api/v1/dags/"+dagName+"/webhook/toggle", api.WebhookToggleRequest{
		Enabled: enabled,
	}).WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	// Trigger should work again
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestWebhooks_RegenerateToken tests token regeneration
func TestWebhooks_RegenerateToken(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_regen_test"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	oldToken := createResult.Token

	// Regenerate the token
	regenResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook/regenerate", nil).
		WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	var regenResult api.WebhookCreateResponse
	regenResp.Unmarshal(t, &regenResult)
	newToken := regenResult.Token

	assert.NotEqual(t, oldToken, newToken, "new token should be different")
	assert.Contains(t, newToken, "dagu_wh_", "new token should have webhook prefix")

	// Old token should no longer work
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(oldToken).
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// New token should work
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(newToken).
		ExpectStatus(http.StatusOK).Send(t)
}

// TestWebhooks_DuplicateCreate tests that creating duplicate webhooks fails
func TestWebhooks_DuplicateCreate(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG
	dagName := "webhook_dup_test"
	createTestDAG(t, server, token, dagName)

	// Create first webhook
	server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	// Try to create duplicate - should fail
	server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusConflict).Send(t)
}

// TestWebhooks_NotFound tests accessing webhooks for non-existent DAGs
func TestWebhooks_NotFound(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Get webhook for non-existent DAG
	server.Client().Get("/api/v1/dags/nonexistent-dag/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNotFound).Send(t)

	// Delete webhook for DAG without webhook
	dagName := "no_webhook_dag"
	createTestDAG(t, server, token, dagName)
	server.Client().Delete("/api/v1/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNotFound).Send(t)
}

// TestWebhooks_TriggerWithPayload tests triggering webhooks with payload
func TestWebhooks_TriggerWithPayload(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_payload_test"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Trigger with payload
	payload := map[string]any{
		"key":    "value",
		"number": 42,
		"nested": map[string]any{
			"inner": "data",
		},
	}
	triggerResp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{
		Payload: &payload,
	}).WithBearerToken(webhookToken).ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.NotEmpty(t, triggerResult.DagRunId)
}

// TestWebhooks_TriggerWithDagRunID tests idempotency with dag-run ID
func TestWebhooks_TriggerWithDagRunID(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_idempotent_test"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Trigger with custom dag-run ID
	customID := "custom-dag-run-id-123"
	triggerResp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{
		DagRunId: &customID,
	}).WithBearerToken(webhookToken).ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.Equal(t, customID, triggerResult.DagRunId)

	// Wait for the DAG run to be recorded, then verify duplicate returns 409 Conflict
	require.Eventually(t, func() bool {
		resp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{
			DagRunId: &customID,
		}).WithBearerToken(webhookToken).Send(t)
		return resp.Response.StatusCode() == http.StatusConflict
	}, 5*time.Second, 100*time.Millisecond, "duplicate dag-run ID should return 409 Conflict")
}

// TestWebhooks_TriggerInvalidToken tests webhook trigger with invalid tokens
func TestWebhooks_TriggerInvalidToken(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_invalid_token_test"
	createTestDAG(t, server, token, dagName)

	server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	// Try with no Authorization header
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// Try with wrong token format
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken("wrong_prefix_token").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// Try with valid prefix but wrong token
	server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken("dagu_wh_invalidtoken12345678901234567890").
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestWebhooks_TriggerNonExistentDAG tests triggering webhook for non-existent DAG
func TestWebhooks_TriggerNonExistentDAG(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)

	// Create a valid webhook token for a real DAG, then try on non-existent
	token := getWebhookAdminToken(t, server)
	dagName := "webhook_existing_dag"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Try to trigger on non-existent DAG (token won't match)
	server.Client().Post("/api/v1/webhooks/nonexistent-dag", api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestIsWebhookTriggerPath tests the webhook trigger path matching helper.
func TestIsWebhookTriggerPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"webhook trigger", "/api/v1/webhooks/my-dag", true},
		{"webhook trigger with base path", "/base/api/v1/webhooks/my-dag", true},
		{"webhook list (no segment)", "/api/v1/webhooks", false},
		{"webhook list trailing slash", "/api/v1/webhooks/", false},
		{"non-webhook path", "/api/v1/dags", false},
		{"dag webhook management", "/api/v1/dags/my-dag/webhook", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := apiimpl.IsWebhookTriggerPath(tt.path)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestMarshalWebhookPayload_RawBodyFallback tests the raw body fallback behavior.
func TestMarshalWebhookPayload_RawBodyFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		body    *api.WebhookRequest
		rawBody []byte
		want    string
	}{
		{
			name:    "structured payload takes precedence",
			body:    &api.WebhookRequest{Payload: &map[string]any{"key": "val"}},
			rawBody: []byte(`{"event":"push"}`),
			want:    `{"key":"val"}`,
		},
		{
			name:    "falls back to raw body when payload is nil",
			body:    &api.WebhookRequest{},
			rawBody: []byte(`{"event":"push","repo":"foo"}`),
			want:    `{"event":"push","repo":"foo"}`,
		},
		{
			name:    "raw body with dagRunId is passed through",
			body:    &api.WebhookRequest{},
			rawBody: []byte(`{"dagRunId":"abc","event":"push"}`),
			want:    `{"dagRunId":"abc","event":"push"}`,
		},
		{
			name:    "nil body falls back to raw body",
			body:    nil,
			rawBody: []byte(`{"event":"push"}`),
			want:    `{"event":"push"}`,
		},
		{
			name:    "no raw body returns empty object",
			body:    &api.WebhookRequest{},
			rawBody: nil,
			want:    "{}",
		},
		{
			name:    "invalid JSON raw body returns empty object",
			body:    &api.WebhookRequest{},
			rawBody: []byte(`not-json`),
			want:    "{}",
		},
		{
			name:    "empty raw body returns empty object",
			body:    &api.WebhookRequest{},
			rawBody: []byte{},
			want:    "{}",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			if tt.rawBody != nil {
				ctx = apiimpl.WithRawBody(ctx, tt.rawBody)
			}
			got, err := apiimpl.MarshalWebhookPayload(ctx, tt.body)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestWebhooks_TriggerWithArbitraryPayload tests triggering a webhook with
// an arbitrary JSON body (no "payload" wrapper), simulating external services.
func TestWebhooks_TriggerWithArbitraryPayload(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	dagName := "webhook_arbitrary_payload_test"
	createTestDAG(t, server, token, dagName)

	createResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Send an arbitrary JSON body WITHOUT the "payload" wrapper.
	// This simulates what external services like GitHub would send.
	arbitraryBody := map[string]any{
		"event": "push",
		"repo":  "foo/bar",
		"ref":   "refs/heads/main",
	}
	triggerResp := server.Client().Post("/api/v1/webhooks/"+dagName, arbitraryBody).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.NotEmpty(t, triggerResult.DagRunId)
	assert.Equal(t, dagName, triggerResult.DagName)
}
