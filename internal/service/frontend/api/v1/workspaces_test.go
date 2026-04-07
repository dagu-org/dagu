// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api_test

import (
	"net/http"
	"testing"

	api "github.com/dagucloud/dagu/api/v1"
	"github.com/stretchr/testify/require"
)

func TestCreateWorkspaceRejectsHyphenatedName(t *testing.T) {
	server := setupBuiltinAuthServer(t)
	token := getAdminToken(t, server)

	resp := server.Client().Post("/api/v1/workspaces", api.CreateWorkspaceRequest{
		Name: "ops-team",
	}).WithBearerToken(token).ExpectStatus(http.StatusBadRequest).Send(t)

	var errResp api.Error
	resp.Unmarshal(t, &errResp)
	require.Equal(t, api.ErrorCodeBadRequest, errResp.Code)
	require.Contains(t, errResp.Message, "letters, numbers, and underscores")
}

func TestUpdateWorkspaceRejectsHyphenatedName(t *testing.T) {
	server := setupBuiltinAuthServer(t)
	token := getAdminToken(t, server)

	createResp := server.Client().Post("/api/v1/workspaces", api.CreateWorkspaceRequest{
		Name: "ops_team",
	}).WithBearerToken(token).ExpectStatus(http.StatusCreated).Send(t)

	var created api.WorkspaceResponse
	createResp.Unmarshal(t, &created)

	invalidName := "ops-team"
	resp := server.Client().Patch("/api/v1/workspaces/"+created.Id, api.UpdateWorkspaceRequest{
		Name: &invalidName,
	}).WithBearerToken(token).ExpectStatus(http.StatusBadRequest).Send(t)

	var errResp api.Error
	resp.Unmarshal(t, &errResp)
	require.Equal(t, api.ErrorCodeBadRequest, errResp.Code)
	require.Contains(t, errResp.Message, "letters, numbers, and underscores")
}
