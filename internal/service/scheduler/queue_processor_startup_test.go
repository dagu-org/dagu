// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"errors"
	osexec "os/exec"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/backoff"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newStartupTestProcessor(dagRunStore exec.DAGRunStore, procStore exec.ProcStore, cfg BackoffConfig) *QueueProcessor {
	return &QueueProcessor{
		dagRunStore:   dagRunStore,
		procStore:     procStore,
		quit:          make(chan struct{}),
		backoffConfig: cfg,
	}
}

func TestQueueProcessor_CheckStartupStatus_WithinGraceSkipsAttemptLookup(t *testing.T) {
	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	runRef := exec.NewDAGRunRef("test-dag", "run-1")

	procStore.On("IsRunAlive", mock.Anything, "test-queue", runRef).Return(false, nil).Once()

	p := newStartupTestProcessor(dagRunStore, procStore, BackoffConfig{
		StartupGracePeriod: time.Second,
	})

	started, err := p.checkStartupStatus(context.Background(), "test-queue", runRef, startupWaitState{
		launchedAt: time.Now(),
		execErrCh:  make(chan error, 1),
	})

	require.False(t, started)
	require.ErrorIs(t, err, errNotStarted)
	dagRunStore.AssertNotCalled(t, "FindAttempt", mock.Anything, mock.Anything)
	procStore.AssertExpectations(t)
}

func TestQueueProcessor_CheckStartupStatus_HeartbeatSkipsAttemptLookup(t *testing.T) {
	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	runRef := exec.NewDAGRunRef("test-dag", "run-1")

	procStore.On("IsRunAlive", mock.Anything, "test-queue", runRef).Return(true, nil).Once()

	p := newStartupTestProcessor(dagRunStore, procStore, BackoffConfig{
		StartupGracePeriod: time.Second,
	})

	started, err := p.checkStartupStatus(context.Background(), "test-queue", runRef, startupWaitState{
		launchedAt: time.Now(),
		execErrCh:  make(chan error, 1),
	})

	require.True(t, started)
	require.NoError(t, err)
	dagRunStore.AssertNotCalled(t, "FindAttempt", mock.Anything, mock.Anything)
	procStore.AssertExpectations(t)
}

func TestQueueProcessor_CheckStartupStatus_PreStartExecutionErrorIsPermanent(t *testing.T) {
	dagRunStore := &mockDAGRunStore{}
	procStore := &mockProcStore{}
	runRef := exec.NewDAGRunRef("test-dag", "run-1")
	execErrCh := make(chan error, 1)
	execErrCh <- errors.New("dispatch failed")

	p := newStartupTestProcessor(dagRunStore, procStore, BackoffConfig{
		StartupGracePeriod: time.Second,
	})

	started, err := p.checkStartupStatus(context.Background(), "test-queue", runRef, startupWaitState{
		launchedAt: time.Now(),
		execErrCh:  execErrCh,
	})

	require.False(t, started)
	require.ErrorIs(t, err, backoff.ErrPermanent)
	dagRunStore.AssertNotCalled(t, "FindAttempt", mock.Anything, mock.Anything)
	procStore.AssertNotCalled(t, "IsRunAlive", mock.Anything, mock.Anything, mock.Anything)
}

func TestQueueProcessor_CheckStartupStatus_AfterGraceFallsBackToStatus(t *testing.T) {
	testCases := []struct {
		name      string
		status    core.Status
		wantStart bool
		wantErr   error
	}{
		{name: "Queued", status: core.Queued, wantStart: false, wantErr: errNotStarted},
		{name: "Running", status: core.Running, wantStart: false, wantErr: errNotStarted},
		{name: "NotStarted", status: core.NotStarted, wantStart: true},
		{name: "Succeeded", status: core.Succeeded, wantStart: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			dagRunStore := &mockDAGRunStore{}
			procStore := &mockProcStore{}
			runRef := exec.NewDAGRunRef("test-dag", "run-1")
			attempt := &exec.MockDAGRunAttempt{
				Status: &exec.DAGRunStatus{Status: tc.status},
			}

			procStore.On("IsRunAlive", mock.Anything, "test-queue", runRef).Return(false, nil).Once()
			dagRunStore.On("FindAttempt", mock.Anything, runRef).Return(attempt, nil).Once()

			p := newStartupTestProcessor(dagRunStore, procStore, BackoffConfig{
				StartupGracePeriod: 50 * time.Millisecond,
			})

			started, err := p.checkStartupStatus(context.Background(), "test-queue", runRef, startupWaitState{
				launchedAt: time.Now().Add(-time.Second),
				execErrCh:  make(chan error, 1),
			})

			require.Equal(t, tc.wantStart, started)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}

			dagRunStore.AssertExpectations(t)
			procStore.AssertExpectations(t)
		})
	}
}

func TestIsPreStartExecutionFailure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "Nil", err: nil, want: false},
		{name: "ContextCanceled", err: context.Canceled, want: false},
		{name: "DeadlineExceeded", err: context.DeadlineExceeded, want: false},
		{name: "ExitError", err: &osexec.ExitError{}, want: false},
		{name: "DispatchFailure", err: errors.New("dispatch failed"), want: true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, isPreStartExecutionFailure(tc.err))
		})
	}
}
