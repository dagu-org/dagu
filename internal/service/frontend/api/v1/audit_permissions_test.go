package api_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/test"
)

func setupAuditTestServer(t *testing.T) test.Server {
	t.Helper()
	return test.SetupServer(t, test.WithConfigMutator(func(cfg *config.Config) {
		cfg.Server.Auth.Mode = config.AuthModeBuiltin
		cfg.Server.Auth.Builtin.Admin.Username = "admin"
		cfg.Server.Auth.Builtin.Admin.Password = "adminpass"
		cfg.Server.Auth.Builtin.Token.Secret = "jwt-secret-key"
		cfg.Server.Auth.Builtin.Token.TTL = 24 * time.Hour
		cfg.Server.Audit.Enabled = true
	}))
}

func TestAudit_RequiresManagerOrAbove(t *testing.T) {
	t.Parallel()
	server := setupAuditTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "manager-user",
		Password: "manager1",
		Role:     api.UserRoleManager,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	server.Client().Post("/api/v1/users", api.CreateUserRequest{
		Username: "developer-user",
		Password: "developer1",
		Role:     api.UserRoleDeveloper,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	managerResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "manager-user",
		Password: "manager1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var managerLogin api.LoginResponse
	managerResp.Unmarshal(t, &managerLogin)

	developerResp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
		Username: "developer-user",
		Password: "developer1",
	}).ExpectStatus(http.StatusOK).Send(t)

	var developerLogin api.LoginResponse
	developerResp.Unmarshal(t, &developerLogin)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(managerLogin.Token).
		ExpectStatus(http.StatusOK).Send(t)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(developerLogin.Token).
		ExpectStatus(http.StatusForbidden).Send(t)
}
