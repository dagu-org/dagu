package cli_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cli"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestServerCommand(t *testing.T) {
	t.Run("StartServer", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cli.Server(), test.CmdTest{
			Args:        []string{"server", fmt.Sprintf("--port=%s", port)},
			ExpectedOut: []string{"Server is starting", port},
		})

	})
	t.Run("StartServerWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cli.Server(), test.CmdTest{
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
