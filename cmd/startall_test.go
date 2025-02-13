package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"
)

func TestStartAllCommand(t *testing.T) {
	t.Run("StartAll", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, startAllCmd(), cmdTest{
			args:        []string{"start-all", fmt.Sprintf("--port=%s", findPort(t))},
			expectedOut: []string{"Serving"},
		})

	})
	t.Run("StartAllWithConfig", func(t *testing.T) {
		th := testSetup(t)
		go func() {
			time.Sleep(time.Millisecond * 500)
			th.Cancel()
		}()
		th.RunCommand(t, startAllCmd(), cmdTest{
			args:        []string{"start-all", "--config", test.TestdataPath(t, "cmd/config_startall.yaml")},
			expectedOut: []string{"54322", "dagu_test"},
		})
	})
}
