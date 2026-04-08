// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmd"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestStartAllCommand(t *testing.T) {
	t.Run("StartAll", func(t *testing.T) {
		th := test.SetupCommand(t, test.WithCoordinatorEnabled())
		go func() {
			require.Eventually(t, func() bool {
				out := th.LoggingOutput.String()
				return strings.Contains(out, "Scheduler initialization") &&
					strings.Contains(out, "Coordinator initialization")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				fmt.Sprintf("--port=%s", findPort(t)),
				"--coordinator.host=0.0.0.0",
				fmt.Sprintf("--coordinator.port=%s", findPort(t)),
			},
			ExpectedOut: []string{"Server initialization", "Scheduler initialization", "Coordinator initialization", "Scheduler stopped"},
		})

	})
	t.Run("StartAllWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			require.Eventually(t, func() bool {
				out := th.LoggingOutput.String()
				return strings.Contains(out, "Coordinator initialization")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.StartAll(), test.CmdTest{
			Args: []string{
				"start-all",
				"--config", test.TestdataPath(t, "cli/config_startall.yaml"),
				fmt.Sprintf("--coordinator.port=%s", findPort(t)),
			},
			ExpectedOut: []string{"54322", "dagu_test", "Coordinator initialization"},
		})
	})
}
