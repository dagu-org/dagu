// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec070_profiles_test

import (
	"net/http"
	"testing"

	api "github.com/dagucloud/dagu/v2/api/v1"
	"github.com/dagucloud/dagu/v2/internal/test"
	"github.com/stretchr/testify/require"
)

// A webhook caller selects a runtime profile with the X-Dagu-Profile header,
// restricted to the profiles ConfigureDAGWebhookProfileSelection allowed for
// that webhook.
func TestWebhookProfileSelectionApplies(t *testing.T) {
	t.Parallel()

	server, adminToken := setupBuiltinAuthServer(t)

	const dagName = "webhook-profile-dag"
	spec := "steps:\n  - command: echo \"$WEBHOOK_VAR\"\n"
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &spec,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	server.Client().Post("/api/v1/profiles", api.CreateRuntimeProfileRequest{Name: "webhookprof"}).
		WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)
	setVariable(t, server, adminToken, "webhookprof", "WEBHOOK_VAR", "from-webhook-profile")

	createHookResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)
	var hookCreate api.WebhookCreateResponse
	createHookResp.Unmarshal(t, &hookCreate)

	server.Client().Put("/api/v1/dags/"+dagName+"/webhook/profile-selection",
		api.WebhookProfileSelectionRequest{AllowedProfiles: []api.RuntimeProfileName{"webhookprof"}}).
		WithBearerToken(adminToken).ExpectStatus(http.StatusOK).Send(t)

	triggerResp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(hookCreate.Token).
		WithHeader("X-Dagu-Profile", "webhookprof").
		ExpectStatus(http.StatusOK).Send(t)
	var trigger api.WebhookResponse
	triggerResp.Unmarshal(t, &trigger)

	test.ProcessQueuedInlineRun(t, server, dagName)

	getResp := server.Client().Get("/api/v1/dag-runs/" + dagName + "/" + string(trigger.DagRunId)).
		WithBearerToken(adminToken).ExpectStatus(http.StatusOK).Send(t)
	var details struct {
		DAGRunDetails struct {
			StatusLabel string `json:"statusLabel"`
			ProfileName string `json:"profileName"`
		} `json:"dagRunDetails"`
	}
	getResp.Unmarshal(t, &details)
	require.Equal(t, "succeeded", details.DAGRunDetails.StatusLabel, "body: %s", getResp.Body)
	require.Equal(t, "webhookprof", details.DAGRunDetails.ProfileName)
}

// A webhook caller naming a profile outside the configured allow-list is
// rejected before any DAG-run is created.
func TestWebhookProfileSelectionRejectsDisallowedProfile(t *testing.T) {
	t.Parallel()

	server, adminToken := setupBuiltinAuthServer(t)

	const dagName = "webhook-profile-reject-dag"
	spec := "steps:\n  - command: echo hi\n"
	server.Client().Post("/api/v1/dags", api.CreateNewDAGJSONRequestBody{
		Name: dagName,
		Spec: &spec,
	}).WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	server.Client().Post("/api/v1/profiles", api.CreateRuntimeProfileRequest{Name: "allowedprof"}).
		WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)

	createHookResp := server.Client().Post("/api/v1/dags/"+dagName+"/webhook", nil).
		WithBearerToken(adminToken).ExpectStatus(http.StatusCreated).Send(t)
	var hookCreate api.WebhookCreateResponse
	createHookResp.Unmarshal(t, &hookCreate)

	server.Client().Put("/api/v1/dags/"+dagName+"/webhook/profile-selection",
		api.WebhookProfileSelectionRequest{AllowedProfiles: []api.RuntimeProfileName{"allowedprof"}}).
		WithBearerToken(adminToken).ExpectStatus(http.StatusOK).Send(t)

	rejectResp := server.Client().Post("/api/v1/webhooks/"+dagName, api.WebhookRequest{}).
		WithBearerToken(hookCreate.Token).
		WithHeader("X-Dagu-Profile", "not-allowed").
		ExpectStatus(http.StatusForbidden).Send(t)
	require.Contains(t, rejectResp.Body, "runtime profile selection is not allowed for this webhook")
}
