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
	lease, err := manager.Acquire()
	require.NoError(t, err)
	_, err = manager.Acquire()
	require.ErrorIs(t, err, ErrMaxSessionsReached)

	lease.Release()

	_, err = manager.Acquire()
	require.NoError(t, err)
}

func TestManager_LeaseActivateRequiresActiveReservation(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	lease, err := manager.Acquire()
	require.NoError(t, err)

	lease.Release()

	err = lease.Activate(&Connection{ID: "conn-1"})
	require.ErrorIs(t, err, ErrReservationInactive)
}

func TestManager_LeaseDoubleReleaseIsNoOp(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	lease, err := manager.Acquire()
	require.NoError(t, err)

	lease.Release()
	lease.Release()

	_, err = manager.Acquire()
	require.NoError(t, err)
}

func TestManager_LeaseReleaseAfterActivationIsNoOp(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	lease, err := manager.Acquire()
	require.NoError(t, err)
	require.NoError(t, lease.Activate(&Connection{ID: "conn-1"}))

	lease.Release()
	lease.Release()

	_, err = manager.Acquire()
	require.NoError(t, err)
}

func TestManager_ActivateFailsWhenManagerIsShuttingDown(t *testing.T) {
	t.Parallel()

	manager := NewManager(context.Background(), 1)
	lease, err := manager.Acquire()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Shutdown(ctx))

	err = lease.Activate(&Connection{ID: "conn-1"})
	require.ErrorIs(t, err, ErrManagerShuttingDown)

	_, err = manager.Acquire()
	require.ErrorIs(t, err, ErrManagerShuttingDown)
}

func TestManager_ShutdownWaitsForActiveSessionsOnly(t *testing.T) {
	t.Parallel()

	t.Run("ActiveSessions", func(t *testing.T) {
		manager := NewManager(context.Background(), 1)
		lease, err := manager.Acquire()
		require.NoError(t, err)
		require.NoError(t, lease.Activate(&Connection{ID: "conn-1"}))

		go func() {
			time.Sleep(100 * time.Millisecond)
			lease.Release()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		require.NoError(t, manager.Shutdown(ctx))
		assert.GreaterOrEqual(t, time.Since(start), 100*time.Millisecond)
	})

	t.Run("PendingReservations", func(t *testing.T) {
		manager := NewManager(context.Background(), 1)
		lease, err := manager.Acquire()
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		start := time.Now()
		require.NoError(t, manager.Shutdown(ctx))
		assert.Less(t, time.Since(start), 100*time.Millisecond)

		lease.Release()
	})
}
