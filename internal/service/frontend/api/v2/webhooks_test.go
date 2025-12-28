package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v2"
	"github.com/dagu-org/dagu/internal/common/config"
	apiimpl "github.com/dagu-org/dagu/internal/service/frontend/api/v2"
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
func setupWebhookTestServer(t *testing.T) test.Server {
	t.Helper()
	return test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
	}))
}

// getWebhookAdminToken authenticates as admin and returns the JWT token
func getWebhookAdminToken(t *testing.T, server test.Server) string {
	t.Helper()
	resp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
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
	server.Client().Post("/api/v2/dags", api.CreateNewDAGJSONRequestBody{
		Name: name,
		Spec: &spec,
	}).WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)
}

// TestWebhooks_ListEmpty tests listing webhooks when none exist
func TestWebhooks_ListEmpty(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	resp := server.Client().Get("/api/v2/webhooks").
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
	server.Client().Get("/api/v2/webhooks").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	server.Client().Post("/api/v2/dags/test-dag/webhook", nil).
		ExpectStatus(http.StatusUnauthorized).Send(t)
}

// TestWebhooks_RequiresAdmin tests that non-admin users cannot access webhook management endpoints
func TestWebhooks_RequiresAdmin(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	// Create a non-admin user
	server.Client().Post("/api/v2/users", api.CreateUserRequest{
		Username: "viewer-user",
		Password: "viewerpass1",
		Role:     api.UserRoleViewer,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	// Login as viewer
	viewerResp := server.Client().Post("/api/v2/auth/login", api.LoginRequest{
		Username: "viewer-user",
		Password: "viewerpass1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var viewerLogin api.LoginResponse
	viewerResp.Unmarshal(t, &viewerLogin)

	// Viewer should get forbidden for webhook management
	server.Client().Get("/api/v2/webhooks").
		WithBearerToken(viewerLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Post("/api/v2/dags/test-dag/webhook", nil).
		WithBearerToken(viewerLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)
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
	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
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
	listResp := server.Client().Get("/api/v2/webhooks").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var listResult api.WebhookListResponse
	listResp.Unmarshal(t, &listResult)
	assert.Len(t, listResult.Webhooks, 1)
	assert.Equal(t, dagName, listResult.Webhooks[0].DagName)

	// Get specific webhook
	getResp := server.Client().Get("/api/v2/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var getResult api.WebhookDetails
	getResp.Unmarshal(t, &getResult)
	assert.Equal(t, createResult.Webhook.Id, getResult.Id)
	assert.Equal(t, dagName, getResult.DagName)

	// Trigger the webhook
	triggerResp := server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.NotEmpty(t, triggerResult.DagRunId)
	assert.Equal(t, dagName, triggerResult.DagName)

	// Wait for LastUsedAt to be updated
	time.Sleep(100 * time.Millisecond)

	// Verify LastUsedAt is updated
	getResp2 := server.Client().Get("/api/v2/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusOK).Send(t)

	var getResult2 api.WebhookDetails
	getResp2.Unmarshal(t, &getResult2)
	require.NotNil(t, getResult2.LastUsedAt, "LastUsedAt should be set after trigger")

	// Delete webhook
	server.Client().Delete("/api/v2/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNoContent).Send(t)

	// Verify it's deleted
	server.Client().Get("/api/v2/dags/" + dagName + "/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNotFound).Send(t)

	// Verify webhook token no longer works
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
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

	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Disable the webhook
	enabled := false
	toggleResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook/toggle", api.WebhookToggleRequest{
		Enabled: enabled,
	}).WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	var toggleResult api.WebhookDetails
	toggleResp.Unmarshal(t, &toggleResult)
	assert.False(t, toggleResult.Enabled)

	// Try to trigger - should fail with forbidden
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusForbidden).Send(t)

	// Re-enable the webhook
	enabled = true
	server.Client().Post("/api/v2/dags/"+dagName+"/webhook/toggle", api.WebhookToggleRequest{
		Enabled: enabled,
	}).WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	// Trigger should work again
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
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

	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	oldToken := createResult.Token

	// Regenerate the token
	regenResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook/regenerate", nil).
		WithBearerToken(token).ExpectStatus(http.StatusOK).Send(t)

	var regenResult api.WebhookCreateResponse
	regenResp.Unmarshal(t, &regenResult)
	newToken := regenResult.Token

	assert.NotEqual(t, oldToken, newToken, "new token should be different")
	assert.Contains(t, newToken, "dagu_wh_", "new token should have webhook prefix")

	// Old token should no longer work
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(oldToken).
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// New token should work
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
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
	server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	// Try to create duplicate - should fail
	server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusConflict).Send(t)
}

// TestWebhooks_NotFound tests accessing webhooks for non-existent DAGs
func TestWebhooks_NotFound(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Get webhook for non-existent DAG
	server.Client().Get("/api/v2/dags/nonexistent-dag/webhook").
		WithBearerToken(token).
		ExpectStatus(http.StatusNotFound).Send(t)

	// Delete webhook for DAG without webhook
	dagName := "no_webhook_dag"
	createTestDAG(t, server, token, dagName)
	server.Client().Delete("/api/v2/dags/" + dagName + "/webhook").
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

	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
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
	triggerResp := server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{
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

	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Trigger with custom dag-run ID
	customID := "custom-dag-run-id-123"
	triggerResp := server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{
		DagRunId: &customID,
	}).WithBearerToken(webhookToken).ExpectStatus(http.StatusOK).Send(t)

	var triggerResult api.WebhookResponse
	triggerResp.Unmarshal(t, &triggerResult)
	assert.Equal(t, customID, triggerResult.DagRunId)

	// Wait for the DAG run to be recorded
	time.Sleep(100 * time.Millisecond)

	// Try to trigger with same dag-run ID - should get 409 Conflict
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{
		DagRunId: &customID,
	}).WithBearerToken(webhookToken).ExpectStatus(http.StatusConflict).Send(t)
}

// TestWebhooks_TriggerInvalidToken tests webhook trigger with invalid tokens
func TestWebhooks_TriggerInvalidToken(t *testing.T) {
	t.Parallel()
	server := setupWebhookTestServer(t)
	token := getWebhookAdminToken(t, server)

	// Create a DAG and webhook
	dagName := "webhook_invalid_token_test"
	createTestDAG(t, server, token, dagName)

	server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	// Try with no Authorization header
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// Try with wrong token format
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken("wrong_prefix_token").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	// Try with valid prefix but wrong token
	server.Client().Post("/api/v2/webhooks/"+dagName, api.WebhookRequest{}).
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

	createResp := server.Client().Post("/api/v2/dags/"+dagName+"/webhook", nil).
		WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var createResult api.WebhookCreateResponse
	createResp.Unmarshal(t, &createResult)
	webhookToken := createResult.Token

	// Try to trigger on non-existent DAG (token won't match)
	server.Client().Post("/api/v2/webhooks/nonexistent-dag", api.WebhookRequest{}).
		WithBearerToken(webhookToken).
		ExpectStatus(http.StatusUnauthorized).Send(t)
}
