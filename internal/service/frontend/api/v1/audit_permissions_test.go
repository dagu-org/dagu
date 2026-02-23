package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/license"
	"github.com/dagu-org/dagu/internal/service/frontend"
	"github.com/dagu-org/dagu/internal/test"
)

// setupAuditTestServer creates a test server with audit enabled but NO license
// manager so it can be used to verify license gating.
func setupAuditTestServer(t *testing.T) test.Server {
	t.Helper()
	server := test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		cfg.Server.Audit.Enabled = true
	}))
	server.Client().Post("/api/v1/auth/setup", api.SetupRequest{
		Username: "admin",
		Password: "adminpass",
	}).ExpectStatus(http.StatusOK).Send(t)
	return server
}

func TestAudit_RequiresManagerOrAbove(t *testing.T) {
	t.Parallel()
	server := setupLicensedAuditTestServer(t)
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

// setupLicensedAuditTestServer creates a test server with audit enabled and a
// license manager that has both "audit" and "rbac" features.
func setupLicensedAuditTestServer(t *testing.T) test.Server {
	t.Helper()
	lm := license.NewTestManager(license.FeatureAudit, license.FeatureRBAC)
	return setupWebhookTestServer(t, func(cfg *config.Config) {
		cfg.Server.Audit.Enabled = true
	}, test.WithServerOptions(frontend.WithLicenseManager(lm)))
}

func TestAudit_RequiresLicense(t *testing.T) {
	t.Parallel()
	// Server with audit enabled but NO license manager — community mode.
	server := setupAuditTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(adminToken).
		ExpectStatus(http.StatusForbidden).Send(t)
}
