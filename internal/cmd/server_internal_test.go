// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/config"
	"github.com/dagucloud/dagu/v2/internal/eventstore"
	"github.com/dagucloud/dagu/v2/internal/license"
	"github.com/dagucloud/dagu/v2/internal/persis"
	"github.com/stretchr/testify/require"
)

func TestStopLocalAgentSessionCleanupHonorsShutdownContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	done := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		result <- stopLocalAgentSessionCleanup(ctx, func() {}, done, nil)
	}()

	select {
	case err := <-result:
		require.ErrorIs(t, err, context.Canceled)
	case <-time.After(100 * time.Millisecond):
		close(done)
		<-result
		t.Fatal("agent session cleanup did not honor the shutdown context")
	}
}

// The reporter runs whenever a license manager does: so that an
// administrator can choose a level later, and, when configuration fixes off,
// so that Dagu Console can be told once that the server no longer reports.
func TestStartCloudReport(t *testing.T) {
	newContext := func(report config.ReportLevel) *Context {
		cfg := &config.Config{Cloud: config.CloudConfig{Report: report}}
		cfg.Paths.DataDir = t.TempDir()
		return &Context{Context: t.Context(), Config: cfg, LicenseManager: license.NewTestManager()}
	}

	for _, level := range []config.ReportLevel{"", config.ReportOff, config.ReportHealth, config.ReportRuns} {
		t.Run("level "+string(level), func(t *testing.T) {
			ctx := newContext(level)
			reporter := startCloudReport(ctx, newCloudReportMonitoring(ctx))
			require.NotNil(t, reporter)
			reporter.Stop()
		})
	}

	t.Run("no license manager", func(t *testing.T) {
		ctx := newContext("")
		ctx.LicenseManager = nil
		monitoring := newCloudReportMonitoring(ctx)
		require.Nil(t, monitoring)
		require.Nil(t, startCloudReport(ctx, monitoring))
	})
}

// The event store and the run history are prepared unless configuration
// fixes a level below runs, because an administrator can choose runs at any
// time.
func TestCloudReportEvents(t *testing.T) {
	runs := persis.NewDAGRunRepository(nil, nil, persis.DAGRunRepositoryOptions{})
	newContext := func(report config.ReportLevel) *Context {
		cfg := &config.Config{Cloud: config.CloudConfig{Report: report}}
		cfg.Paths.DataDir = t.TempDir()
		return &Context{
			Context:     t.Context(),
			Config:      cfg,
			event:       eventstore.New(nil),
			Persistence: Persistence{DAGRunRepository: runs},
		}
	}

	for _, level := range []config.ReportLevel{config.ReportRuns, ""} {
		events := cloudReportEvents(newContext(level))
		require.NotNil(t, events.Reader)
		require.Equal(t, runs, events.Statuses)
	}
	require.Nil(t, cloudReportEvents(newContext(config.ReportHealth)).Reader)
	require.Nil(t, cloudReportEvents(newContext(config.ReportOff)).Reader)
}
