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

func TestAutomataRuntimeActionsRequireReadyController(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newAutomataMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					AutomataController: &exec.AutomataControllerInfo{
						State:   exec.AutomataControllerStateUnavailable,
						Message: "controller is not ready",
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	_, err := api.StartAutomata(ctx, openapi.StartAutomataRequestObject{
		Name: "software_dev",
		Body: &openapi.AutomataStartRequest{Instruction: ptrOf("Ship the feature")},
	})
	require.Error(t, err)
	apiErr, ok := err.(*Error)
	require.True(t, ok)
	require.Equal(t, 409, apiErr.HTTPStatus)
	require.Contains(t, apiErr.Message, "controller is not ready")
}

func TestAutomataListAndDetailExposeControllerStatus(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	api, svc, _ := newAutomataMemoryAPI(t)
	api.serviceRegistry = &stubServiceRegistry{
		members: map[exec.ServiceName][]exec.HostInfo{
			exec.ServiceNameScheduler: {
				{
					ID:     "scheduler-1",
					Host:   "localhost",
					Status: exec.ServiceStatusActive,
					AutomataController: &exec.AutomataControllerInfo{
						State: exec.AutomataControllerStateReady,
					},
				},
			},
		},
	}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", "goal: Complete the assigned software work\nallowed_dags:\n  names:\n    - build-app\n"))

	listResp, err := api.ListAutomata(ctx, openapi.ListAutomataRequestObject{})
	require.NoError(t, err)
	listOK, ok := listResp.(openapi.ListAutomata200JSONResponse)
	require.True(t, ok)
	require.Len(t, listOK.Automata, 1)
	require.NotNil(t, listOK.Automata[0].AutomataController)
	require.Equal(t, openapi.AutomataControllerStatusStateReady, listOK.Automata[0].AutomataController.State)

	detailResp, err := api.GetAutomata(ctx, openapi.GetAutomataRequestObject{Name: "software_dev"})
	require.NoError(t, err)
	detailOK, ok := detailResp.(openapi.GetAutomata200JSONResponse)
	require.True(t, ok)
	require.NotNil(t, detailOK.AutomataController)
	require.Equal(t, openapi.AutomataControllerStatusStateReady, detailOK.AutomataController.State)
}
