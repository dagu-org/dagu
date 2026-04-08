// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/cmd"
	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestServerCommand(t *testing.T) {
	t.Run("StartServer", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			require.Eventually(t, func() bool {
				return strings.Contains(th.LoggingOutput.String(), "Server is starting")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.Server(), test.CmdTest{
			Args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			ExpectedOut: []string{"Server is starting", port},
		})

	})
	t.Run("StartServerWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			require.Eventually(t, func() bool {
				return strings.Contains(th.LoggingOutput.String(), "54321")
			}, 5*time.Second, 50*time.Millisecond)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.Server(), test.CmdTest{
			Args:        []string{"server", "--config", test.TestdataPath(t, "cli/config_test.yaml")},
			ExpectedOut: []string{"54321"},
		})
	})
}

// findPort finds an available port.
func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}
