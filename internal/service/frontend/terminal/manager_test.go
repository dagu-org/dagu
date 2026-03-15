// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package terminal

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManager_ReleasePendingFreesReservation(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	require.NoError(t, manager.Acquire())
	require.ErrorIs(t, manager.Acquire(), ErrMaxSessionsReached)

	manager.ReleasePending()

	require.NoError(t, manager.Acquire())
}

func TestManager_RegisterRequiresReservation(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	err := manager.Register(&Connection{ID: "conn-1"})
	require.ErrorIs(t, err, ErrReservationRequired)
}

func TestManager_ReleaseSessionUnknownIDIsNoOp(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	require.NoError(t, manager.Acquire())
	require.NoError(t, manager.Register(&Connection{ID: "conn-1"}))

	manager.ReleaseSession("missing")

	require.ErrorIs(t, manager.Acquire(), ErrMaxSessionsReached)

	manager.ReleaseSession("conn-1")
	require.NoError(t, manager.Acquire())
}

func TestManager_ShutdownWaitsForActiveSessionsOnly(t *testing.T) {
	t.Parallel()

	t.Run("ActiveSessions", func(t *testing.T) {
		manager := NewManager(context.Background(), 1)
		require.NoError(t, manager.Acquire())
		require.NoError(t, manager.Register(&Connection{ID: "conn-1"}))

		go func() {
			time.Sleep(100 * time.Millisecond)
			manager.ReleaseSession("conn-1")
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		require.NoError(t, manager.Shutdown(ctx))
		assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
	})

	t.Run("PendingReservations", func(t *testing.T) {
		manager := NewManager(context.Background(), 1)
		require.NoError(t, manager.Acquire())

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		require.NoError(t, manager.Shutdown(ctx))
		assert.Less(t, time.Since(start), 100*time.Millisecond)
	})
}
