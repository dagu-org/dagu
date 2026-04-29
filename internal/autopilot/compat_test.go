// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package autopilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceReadsLegacyAutomataDefinitionAndState(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t)
	ctx := context.Background()

	require.NoError(t, os.MkdirAll(svc.legacyDefinitionsDir, 0o750))
	require.NoError(t, os.WriteFile(
		filepath.Join(svc.legacyDefinitionsDir, "legacy_ops.yaml"),
		[]byte(autopilotSpec("build-app")),
		0o600,
	))

	state := newInitialState()
	state.State = StatePaused
	data, err := json.Marshal(state)
	require.NoError(t, err)
	legacyStateDir := filepath.Join(svc.legacyStateDir, "legacy_ops")
	require.NoError(t, os.MkdirAll(legacyStateDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(legacyStateDir, "state.json"), data, 0o600))

	def, err := svc.GetDefinition(ctx, "legacy_ops")
	require.NoError(t, err)
	assert.Equal(t, "legacy_ops", def.Name)

	loadedState, err := svc.loadState(ctx, "legacy_ops")
	require.NoError(t, err)
	require.NotNil(t, loadedState)
	assert.Equal(t, StatePaused, loadedState.State)
}

func TestPutSpecUpdatesLegacyAutomataDefinitionInPlace(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t)
	ctx := context.Background()

	legacyPath := filepath.Join(svc.legacyDefinitionsDir, "legacy_ops.yaml")
	require.NoError(t, os.MkdirAll(svc.legacyDefinitionsDir, 0o750))
	require.NoError(t, os.WriteFile(legacyPath, []byte(autopilotSpec("build-app")), 0o600))

	require.NoError(t, svc.PutSpec(ctx, "legacy_ops", autopilotSpecWithModel("build-app", "model-2")))

	_, err := os.Stat(legacyPath)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(svc.definitionsDir, "legacy_ops.yaml"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}
