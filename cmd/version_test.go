package cmd

import (
	"testing"

	"github.com/dagu-dev/dagu/internal/constants"
)

func TestVersionCommand(t *testing.T) {
	constants.Version = "1.2.3"
	testRunCommand(t, versionCmd(), cmdTest{
		args:        []string{"version"},
		expectedOut: []string{"1.2.3"}})
}
