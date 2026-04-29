// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"context"
	"testing"

	openapi "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/stretchr/testify/require"
)

type stubServiceRegistry struct {
	members map[exec.ServiceName][]exec.HostInfo
}

func (s *stubServiceRegistry) Register(context.Context, exec.ServiceName, exec.HostInfo) error {
	return nil
}

func (s *stubServiceRegistry) Unregister(context.Context) {}

func (s *stubServiceRegistry) GetServiceMembers(_ context.Context, serviceName exec.ServiceName) ([]exec.HostInfo, error) {
	return append([]exec.HostInfo(nil), s.members[serviceName]...), nil
}

func (s *stubServiceRegistry) UpdateStatus(context.Context, exec.ServiceName, exec.ServiceStatus) error {
	return nil
}

func TestAutopilotRuntimeActionsRequireReadyController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newAutopilotMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					AutopilotController: &exec.AutopilotControllerInfo{
						State:   exec.AutopilotControllerStateUnavailable,
						Message: "controller is not ready",
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	_, err := api.StartAutopilot(ctx, openapi.StartAutopilotRequestObject{
		Name: "software_dev",
		Body: &openapi.AutopilotStartRequest{Instruction: ptrOf("Ship the feature")},
	})
	require.Error(t, err)
	apiErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, 409, apiErr.HTTPStatus)
	require.Contains(t, apiErr.Message, "controller is not ready")
}

func TestAutopilotListAndDetailExposeControllerStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newAutopilotMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					AutopilotController: &exec.AutopilotControllerInfo{
						State: exec.AutopilotControllerStateReady,
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	listResp, err := api.ListAutopilot(ctx, openapi.ListAutopilotRequestObject{})
	require.NoError(t, err)
	listOK, ok := listResp.(openapi.ListAutopilot200JSONResponse)
	require.True(t, ok)
	require.Len(t, listOK.Autopilot, 1)
	require.NotNil(t, listOK.Autopilot[0].AutopilotController)
	require.Equal(t, openapi.AutopilotControllerStatusStateReady, listOK.Autopilot[0].AutopilotController.State)

	detailResp, err := api.GetAutopilot(ctx, openapi.GetAutopilotRequestObject{Name: "software_dev"})
	require.NoError(t, err)
	detailOK, ok := detailResp.(openapi.GetAutopilot200JSONResponse)
	require.True(t, ok)
	require.NotNil(t, detailOK.AutopilotController)
	require.Equal(t, openapi.AutopilotControllerStatusStateReady, detailOK.AutopilotController.State)
}
