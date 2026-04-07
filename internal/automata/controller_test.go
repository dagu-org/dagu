// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package automata

import (
	"context"
	"errors"
	"testing"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/core/exec"
	"github.com/dagucloud/dagu/internal/service/eventstore"
	"github.com/stretchr/testify/require"
)

type failingSoulStore struct {
	err error
}

func (*failingSoulStore) Create(context.Context, *agent.Soul) error {
	return nil
}

func (s *failingSoulStore) GetByID(context.Context, string) (*agent.Soul, error) {
	return nil, s.err
}

func (*failingSoulStore) List(context.Context) ([]*agent.Soul, error) {
	return nil, nil
}

func (*failingSoulStore) Search(context.Context, agent.SearchSoulsOptions) (*exec.PaginatedResult[agent.SoulMetadata], error) {
	return nil, nil
}

func (*failingSoulStore) Update(context.Context, *agent.Soul) error {
	return nil
}

func (*failingSoulStore) Delete(context.Context, string) error {
	return nil
}

func TestStartTurnPersistsRuntimeOptionErrors(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	svc, _, store := newTestServiceWithEventStore(t)
	svc.soulStore = &failingSoulStore{err: errors.New("soul lookup failed")}

	require.NoError(t, svc.PutSpec(ctx, "software_dev", `goal: Complete the assigned software work
allowed_dags:
  names:
    - build-app
agent:
  soul: "missing"
`))

	def, err := svc.GetDefinition(ctx, "software_dev")
	require.NoError(t, err)
	state, err := svc.ensureState(ctx, def)
	require.NoError(t, err)
	queueTurnMessage(state, "kickoff", "start working", svc.clock())
	require.NoError(t, svc.saveState(ctx, def.Name, state))

	require.NoError(t, svc.flushPendingTurnMessages(ctx, def, state))

	reloaded, err := svc.loadState(ctx, def.Name)
	require.NoError(t, err)
	require.Equal(t, "soul lookup failed", reloaded.LastError)
	require.Len(t, reloaded.PendingTurnMessages, 1)
	require.Len(t, store.events, 1)
	require.Equal(t, eventstore.TypeAutomataError, store.events[0].Type)
}
