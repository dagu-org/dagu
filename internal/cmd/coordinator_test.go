package cmd_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestCoordinatorCommand(t *testing.T) {
	t.Run("StartCoordinator", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args:        []string{"coordinator", fmt.Sprintf("--coordinator.port=%s", port)},
			ExpectedOut: []string{"Coordinator initialization", port},
		})
	})

	t.Run("StartCoordinatorWithHost", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args:        []string{"coordinator", "--coordinator.host=0.0.0.0", fmt.Sprintf("--coordinator.port=%s", port)},
			ExpectedOut: []string{"Coordinator initialization", "0.0.0.0", port},
		})
	})

	t.Run("StartCoordinatorWithConfig", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args:        []string{"coordinator", "--config", test.TestdataPath(t, "cmd/config_test.yaml")},
			ExpectedOut: []string{"Coordinator initialization", "9876"},
		})
	})

	t.Run("StartCoordinatorWithSigningKey", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args:        []string{"coordinator", fmt.Sprintf("--coordinator.port=%s", port), "--coordinator.signing-key=test-secret-key"},
			ExpectedOut: []string{"Coordinator initialization", port},
		})
	})

	t.Run("StartCoordinatorWithTLS", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args: []string{
				"coordinator",
				fmt.Sprintf("--coordinator.port=%s", port),
				"--coordinator.tls-cert=/path/to/cert.pem",
				"--coordinator.tls-key=/path/to/key.pem",
			},
			ExpectedOut: []string{"Coordinator initialization", port},
		})
	})

	t.Run("StartCoordinatorWithMutualTLS", func(t *testing.T) {
		th := test.SetupCommand(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		port := findPort(t)
		th.RunCommand(t, cmd.CmdCoordinator(), test.CmdTest{
			Args: []string{
				"coordinator",
				fmt.Sprintf("--coordinator.port=%s", port),
				"--coordinator.tls-cert=/path/to/cert.pem",
				"--coordinator.tls-key=/path/to/key.pem",
				"--coordinator.tls-ca=/path/to/ca.pem",
			},
			ExpectedOut: []string{"Coordinator initialization", port},
		})
	})
}
