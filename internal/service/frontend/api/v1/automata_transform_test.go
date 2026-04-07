// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package api

import (
	"testing"
	"time"

	openapi "github.com/dagucloud/dagu/api/v1"
	"github.com/dagucloud/dagu/internal/automata"
	"github.com/stretchr/testify/require"
)

func TestToAPIAutomataSummaryIncludesDerivedStatusFields(t *testing.T) {
	t.Parallel()

	summary := automata.Summary{
		Name:          "queue_worker",
		Kind:          automata.AutomataKindService,
		Nickname:      "Queue Butler",
		IconURL:       "https://cdn.example.com/queue-butler.png",
		Goal:          "Handle inbound work continuously.",
		State:         automata.StateIdle,
		DisplayStatus: automata.DisplayStatusRunning,
		Busy:          false,
		NeedsInput:    true,
	}

	resp := toAPIAutomataSummary(summary)
	require.Equal(t, openapi.AutomataKindService, resp.Kind)
	require.NotNil(t, resp.Nickname)
	require.Equal(t, "Queue Butler", *resp.Nickname)
	require.NotNil(t, resp.IconUrl)
	require.Equal(t, "https://cdn.example.com/queue-butler.png", *resp.IconUrl)
	require.NotNil(t, resp.DisplayStatus)
	require.Equal(t, openapi.AutomataDisplayStatusRunning, *resp.DisplayStatus)
	require.NotNil(t, resp.Busy)
	require.False(t, *resp.Busy)
	require.NotNil(t, resp.NeedsInput)
	require.True(t, *resp.NeedsInput)
}

func TestToAPIAutomataStateDerivesServiceDisplayFields(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.April, 2, 12, 0, 0, 0, time.UTC)
	def := &automata.Definition{
		Name:     "queue_worker",
		Kind:     automata.AutomataKindService,
		Nickname: "Queue Butler",
		IconURL:  "https://cdn.example.com/queue-butler.png",
		Goal:     "Handle inbound work continuously.",
	}
	state := &automata.State{
		State:       automata.StateIdle,
		ActivatedAt: now,
		ActivatedBy: "tester",
	}

	resp := toAPIAutomataState(def, state)
	require.NotNil(t, resp.DisplayStatus)
	require.Equal(t, openapi.AutomataDisplayStatusRunning, *resp.DisplayStatus)
	require.NotNil(t, resp.Busy)
	require.False(t, *resp.Busy)
	require.NotNil(t, resp.NeedsInput)
	require.False(t, *resp.NeedsInput)
	require.NotNil(t, resp.ActivatedAt)
	require.Equal(t, now, *resp.ActivatedAt)
	require.NotNil(t, resp.ActivatedBy)
	require.Equal(t, "tester", *resp.ActivatedBy)

	defResp := toAPIAutomataDefinition(def)
	require.NotNil(t, defResp.Nickname)
	require.Equal(t, "Queue Butler", *defResp.Nickname)
	require.NotNil(t, defResp.IconUrl)
	require.Equal(t, "https://cdn.example.com/queue-butler.png", *defResp.IconUrl)
}
