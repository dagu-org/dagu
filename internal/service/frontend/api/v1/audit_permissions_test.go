package api_test

import (
	"net/http"
	"testing"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
)

func setupAuditTestServer(t *testing.T) test.Server {
	t.Helper()
	return setupWebhookTestServer(t, func(cfg *config.Config) {
		cfg.Server.Audit.Enabled = true
	})
}

func TestAudit_RequiresManagerOrAbove(t *testing.T) {
	t.Parallel()
	server := setupAuditTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	// Create users for each role below manager.
	for _, u := range []struct {
		username string
		password string
		role     api.UserRole
	}{
		{"manager-user", "manager1", api.UserRoleManager},
		{"developer-user", "developer1", api.UserRoleDeveloper},
		{"operator-user", "operator1", api.UserRoleOperator},
		{"viewer-user", "viewerpass1", api.UserRoleViewer},
	} {
		server.Client().Post("/api/v1/users", api.CreateUserRequest{
			Username: u.username,
			Password: u.password,
			Role:     u.role,
		}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)
	}

	login := func(username, password string) string {
		resp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: username,
			Password: password,
		}).ExpectStatus(http.StatusOK).Send(t)
		var result api.LoginResponse
		resp.Unmarshal(t, &result)
		return result.Token
	}

	managerToken := login("manager-user", "manager1")
	developerToken := login("developer-user", "developer1")
	operatorToken := login("operator-user", "operator1")
	viewerToken := login("viewer-user", "viewerpass1")

	// Manager can access audit endpoint.
	server.Client().Get("/api/v1/audit").
		WithBearerToken(managerToken).
		ExpectStatus(http.StatusOK).Send(t)

	// Developer, operator, and viewer are forbidden.
	server.Client().Get("/api/v1/audit").
		WithBearerToken(developerToken).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(operatorToken).
		ExpectStatus(http.StatusForbidden).Send(t)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(viewerToken).
		ExpectStatus(http.StatusForbidden).Send(t)
}
