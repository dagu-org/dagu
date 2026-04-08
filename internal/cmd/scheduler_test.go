// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmd"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			require.Eventually(t, func() bool {
				return strings.Contains(th.LoggingOutput.String(), "Scheduler started")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()

		th.RunCommand(t, cmd.Scheduler(), test.CmdTest{
			Args:        []string{"scheduler"},
			ExpectedOut: []string{"Scheduler started", "Scheduler stopped"},
		})
	})
	t.Run("StartSchedulerWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			require.Eventually(t, func() bool {
				return strings.Contains(th.LoggingOutput.String(), "dagu_test")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()

		th.RunCommand(t, cmd.Scheduler(), test.CmdTest{
			Args:        []string{"scheduler", "--config", test.TestdataPath(t, "cli/config_test.yaml")},
			ExpectedOut: []string{"dagu_test"},
		})
	})
}
