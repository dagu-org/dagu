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

func TestControllerRuntimeActionsRequireReadyController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newControllerMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					ControllerStatus: &exec.ControllerStatusInfo{
						State:   exec.ControllerStatusStateUnavailable,
						Message: "controller is not ready",
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	_, err := api.StartController(ctx, openapi.StartControllerRequestObject{
		Name: "software_dev",
		Body: &openapi.ControllerStartRequest{Instruction: ptrOf("Ship the feature")},
	})
	require.Error(t, err)
	apiErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, 409, apiErr.HTTPStatus)
	require.Contains(t, apiErr.Message, "controller is not ready")
}

func TestControllerListAndDetailExposeControllerStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newControllerMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					ControllerStatus: &exec.ControllerStatusInfo{
						State: exec.ControllerStatusStateReady,
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	listResp, err := api.ListController(ctx, openapi.ListControllerRequestObject{})
	require.NoError(t, err)
	listOK, ok := listResp.(openapi.ListController200JSONResponse)
	require.True(t, ok)
	require.Len(t, listOK.Controller, 1)
	require.NotNil(t, listOK.Controller[0].ControllerStatus)
	require.Equal(t, openapi.ControllerStatusStateReady, listOK.Controller[0].ControllerStatus.State)

	detailResp, err := api.GetController(ctx, openapi.GetControllerRequestObject{Name: "software_dev"})
	require.NoError(t, err)
	detailOK, ok := detailResp.(openapi.GetController200JSONResponse)
	require.True(t, ok)
	require.NotNil(t, detailOK.ControllerStatus)
	require.Equal(t, openapi.ControllerStatusStateReady, detailOK.ControllerStatus.State)
}
