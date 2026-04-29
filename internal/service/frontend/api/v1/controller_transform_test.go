// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"testing"
	"time"

	openapi "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/controller"
	"github.com/stretchr/testify/require"
)

func TestToAPIControllerSummaryIncludesDerivedStatusFields(t *testing.T) {
	t.Parallel()

	summary := controller.Summary{
		Name:          "queue_worker",
		Kind:          controller.ControllerKindWorkflow,
		Nickname:      "Queue Butler",
		IconURL:       "https://cdn.example.com/queue-butler.png",
		Goal:          "Handle inbound work continuously.",
		ClonedFrom:    "software_dev",
		ResetOnFinish: true,
		State:         controller.StateIdle,
		DisplayStatus: controller.DisplayStatusRunning,
		Busy:          false,
		NeedsInput:    true,
	}

	resp := toAPIControllerSummary(summary)
	require.Equal(t, openapi.ControllerKindWorkflow, resp.Kind)
	require.NotNil(t, resp.Nickname)
	require.Equal(t, "Queue Butler", *resp.Nickname)
	require.NotNil(t, resp.IconUrl)
	require.Equal(t, "https://cdn.example.com/queue-butler.png", *resp.IconUrl)
	require.NotNil(t, resp.ClonedFrom)
	require.Equal(t, "software_dev", *resp.ClonedFrom)
	require.NotNil(t, resp.ResetOnFinish)
	require.True(t, *resp.ResetOnFinish)
	require.NotNil(t, resp.DisplayStatus)
	require.Equal(t, openapi.ControllerDisplayStatusRunning, *resp.DisplayStatus)
	require.NotNil(t, resp.Busy)
	require.False(t, *resp.Busy)
	require.NotNil(t, resp.NeedsInput)
	require.True(t, *resp.NeedsInput)
}

func TestToAPIControllerStateDerivesDisplayFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)
	def := &controller.Definition{
		Name:       "queue_worker",
		Kind:       controller.ControllerKindWorkflow,
		Nickname:   "Queue Butler",
		IconURL:    "https://cdn.example.com/queue-butler.png",
		Goal:       "Handle inbound work continuously.",
		ClonedFrom: "software_dev",
		Trigger: controller.Trigger{
			Type: controller.TriggerModeCron,
			Schedules: controller.CronScheduleList{
				{Expression: "0 * * * *"},
			},
			Prompt: "Handle inbound work continuously.",
		},
	}
	state := &controller.State{
		State:       controller.StateWaiting,
		ActivatedAt: now,
		ActivatedBy: "tester",
	}

	resp := toAPIControllerState(def, state)
	require.NotNil(t, resp.DisplayStatus)
	require.Equal(t, openapi.ControllerDisplayStatusRunning, *resp.DisplayStatus)
	require.NotNil(t, resp.Busy)
	require.False(t, *resp.Busy)
	require.NotNil(t, resp.NeedsInput)
	require.False(t, *resp.NeedsInput)
	require.NotNil(t, resp.ActivatedAt)
	require.Equal(t, now, *resp.ActivatedAt)
	require.NotNil(t, resp.ActivatedBy)
	require.Equal(t, "tester", *resp.ActivatedBy)

	defResp := toAPIControllerDefinition(def)
	require.NotNil(t, defResp.Nickname)
	require.Equal(t, "Queue Butler", *defResp.Nickname)
	require.NotNil(t, defResp.IconUrl)
	require.Equal(t, "https://cdn.example.com/queue-butler.png", *defResp.IconUrl)
	require.NotNil(t, defResp.ClonedFrom)
	require.Equal(t, "software_dev", *defResp.ClonedFrom)
	require.Equal(t, openapi.ControllerTriggerTypeCron, defResp.Trigger.Type)
	require.NotNil(t, defResp.Trigger.Schedules)
	require.Equal(t, []string{"0 * * * *"}, *defResp.Trigger.Schedules)
	require.NotNil(t, defResp.Trigger.Prompt)
	require.Equal(t, "Handle inbound work continuously.", *defResp.Trigger.Prompt)
}
