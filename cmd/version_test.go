package cmd

import (
	"testing"

	"github.com/yohamta/dagu/internal/constants"
)

func TestVersionCommand(t *testing.T) {
	constants.Version = "1.2.3"
	testRunCommand(t, versionCommand(), cmdTest{
		args:        []string{"version"},
		expectedOut: []string{"1.2.3"}})
}
