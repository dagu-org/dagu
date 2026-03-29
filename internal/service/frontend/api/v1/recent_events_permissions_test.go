// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/dagu-org/dagu/api/v1"
	"github.com/dagu-org/dagu/internal/cmn/config"
	"github.com/dagu-org/dagu/internal/service/eventfeed"
	"github.com/dagu-org/dagu/internal/service/frontend"
	apiv1 "github.com/dagu-org/dagu/internal/service/frontend/api/v1"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestRecentEventsRequiresAuthenticationAndAllowsAllRoles(t *testing.T) {
	t.Parallel()

	server := setupWebhookTestServer(t)
	adminToken := getWebhookAdminToken(t, server)

	users := []struct {
		username string
		password string
		role     api.UserRole
	}{
		{"manager-user", "manager1", api.UserRoleManager},
		{"developer-user", "developer1", api.UserRoleDeveloper},
		{"operator-user", "operator1", api.UserRoleOperator},
		{"viewer-user", "viewerpass1", api.UserRoleViewer},
	}
	for _, u := range users {
		server.Client().Post("/api/v1/users", api.CreateUserRequest{
			Username: u.username,
			Password: u.password,
			Role:     u.role,
		}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)
	}

	tokens := []string{adminToken}
	for _, u := range users {
		resp := server.Client().Post("/api/v1/auth/login", api.LoginRequest{
			Username: u.username,
			Password: u.password,
		}).ExpectStatus(http.StatusOK).Send(t)

		var login api.LoginResponse
		resp.Unmarshal(t, &login)
		tokens = append(tokens, login.Token)
	}

	server.Client().Get("/api/v1/recent-events").
		ExpectStatus(http.StatusUnauthorized).Send(t)

	for _, token := range tokens {
		server.Client().Get("/api/v1/recent-events").
			WithBearerToken(token).
			ExpectStatus(http.StatusOK).Send(t)
	}
}

func TestRecentEventsNotAuditLicensed(t *testing.T) {
	t.Parallel()

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
	adminToken := getWebhookAdminToken(t, server)

	server.Client().Get("/api/v1/recent-events").
		WithBearerToken(adminToken).
		ExpectStatus(http.StatusOK).Send(t)

	server.Client().Get("/api/v1/audit").
		WithBearerToken(adminToken).
		ExpectStatus(http.StatusForbidden).Send(t)
}

func TestApprovalActionsSucceedWhenRecentEventRecordingFails(t *testing.T) {
	t.Parallel()

	t.Run("Approve", func(t *testing.T) {
		t.Parallel()
		server, dagRunID := setupWaitingDAGWithFailingRecentEvents(t, "approve_fail_recent_event")

		resp := server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/approve_fail_recent_event/%s/steps/wait-step/approve", dagRunID),
			api.ApproveStepRequest{},
		).ExpectStatus(http.StatusOK).Send(t)

		var body api.ApproveDAGRunStep200JSONResponse
		resp.Unmarshal(t, &body)
		require.True(t, body.Resumed)

		waitForDagRunStatus(t, server, "approve_fail_recent_event", dagRunID, api.StatusSuccess)
	})

	t.Run("Reject", func(t *testing.T) {
		t.Parallel()
		server, dagRunID := setupWaitingDAGWithFailingRecentEvents(t, "reject_fail_recent_event")

		reason := "no"
		server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/reject_fail_recent_event/%s/steps/wait-step/reject", dagRunID),
			api.RejectStepRequest{Reason: &reason},
		).ExpectStatus(http.StatusOK).Send(t)

		waitForDagRunStatus(t, server, "reject_fail_recent_event", dagRunID, api.StatusRejected)
	})

	t.Run("PushBack", func(t *testing.T) {
		t.Parallel()
		server, dagRunID := setupWaitingDAGWithFailingRecentEvents(t, "push_back_fail_recent_event")

		resp := server.Client().Post(
			fmt.Sprintf("/api/v1/dag-runs/push_back_fail_recent_event/%s/steps/wait-step/push-back", dagRunID),
			api.PushBackStepRequest{},
		).ExpectStatus(http.StatusOK).Send(t)

		var body api.PushBackDAGRunStep200JSONResponse
		resp.Unmarshal(t, &body)
		require.True(t, body.Resumed)
		require.Equal(t, 1, body.ApprovalIteration)

		waitForDagRunStatus(t, server, "push_back_fail_recent_event", dagRunID, api.StatusWaiting)
	})
}

func setupWaitingDAGWithFailingRecentEvents(t *testing.T, dagName string) (test.Server, string) {
	t.Helper()

	failingSvc := eventfeed.New(&failingEventFeedStore{err: errors.New("boom")}, eventfeed.WithWriteTimeout(10*time.Millisecond))
	t.Cleanup(func() { _ = failingSvc.Close() })

	server := test.SetupServer(t,
		test.WithServerOptions(frontend.WithAPIOption(apiv1.WithEventFeedService(failingSvc))),
	)

	spec := `type: graph
steps:
  - name: wait-step
    command: "true"
    approval:
      prompt: "Please approve"
  - name: after-wait
    depends: [wait-step]
    command: "echo done"`
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &spec,
	}).ExpectStatus(http.StatusCreated).Send(t)

	startResp := server.Client().Post(fmt.Sprintf("/api/v1/dags/%s/start", dagName), api.ExecuteDAGJSONRequestBody{}).
		ExpectStatus(http.StatusOK).Send(t)

	var startBody api.ExecuteDAG200JSONResponse
	startResp.Unmarshal(t, &startBody)
	require.NotEmpty(t, startBody.DagRunId)

	waitForDagRunStatus(t, server, dagName, startBody.DagRunId, api.StatusWaiting)
	return server, startBody.DagRunId
}

func waitForDagRunStatus(t *testing.T, server test.Server, dagName, dagRunID string, expected api.Status) {
	t.Helper()

	require.Eventually(t, func() bool {
		resp := server.Client().Get(fmt.Sprintf("/api/v1/dags/%s/dag-runs/%s", dagName, dagRunID)).Send(t)
		if resp.Response.StatusCode() != http.StatusOK {
			return false
		}

		var body api.GetDAGDAGRunDetails200JSONResponse
		resp.Unmarshal(t, &body)
		return body.DagRun.Status == expected
	}, 15*time.Second, 100*time.Millisecond)
}

type failingEventFeedStore struct {
	err error
}

func (s *failingEventFeedStore) Append(_ context.Context, _ *eventfeed.Entry) error {
	return s.err
}

func (s *failingEventFeedStore) Query(context.Context, eventfeed.QueryFilter) (*eventfeed.QueryResult, error) {
	return &eventfeed.QueryResult{}, nil
}

func (s *failingEventFeedStore) Close() error { return nil }
