package cmd

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/constants"
)

func TestVersionCommand(t *testing.T) {
	t.Run("[Success] Version command should run", func(t *testing.T) {
		constants.Version = "1.2.3"
		testRunCommand(t, versionCmd(), cmdTest{
			args:        []string{"version"},
			expectedOut: []string{"1.2.3"}})
	})
}
