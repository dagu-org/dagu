package cmd

import (
	"testing"
	"time"

	"github.com/dagu-dev/dagu/internal/test"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		setup := test.Setup(t)
		defer setup.Cleanup()

		go func() {
			testRunCommand(t, schedulerCmd(), cmdTest{
				args:        []string{"scheduler"},
				expectedOut: []string{"starting dagu scheduler"},
			})
		}()

		time.Sleep(time.Millisecond * 500)
	})
}
