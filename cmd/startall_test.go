package main_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/test"
)

func TestStartAllCommand(t *testing.T) {
	t.Run("StartAll", func(t *testing.T) {
		th := test.SetupCommandTest(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.CmdStartAll(), test.CmdTest{
			Args:        []string{"start-all", fmt.Sprintf("--port=%s", findPort(t))},
			ExpectedOut: []string{"Serving"},
		})

	})
	t.Run("StartAllWithConfig", func(t *testing.T) {
		th := test.SetupCommandTest(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, cmd.CmdStartAll(), test.CmdTest{
			Args:        []string{"start-all", "--config", test.TestdataPath(t, "cmd/config_startall.yaml")},
			ExpectedOut: []string{"54322", "dagu_test"},
		})
	})
}
