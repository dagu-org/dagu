package scheduler

import (
	"os"
	"testing"

	"github.com/daguflow/dagu/internal/util"
	"go.uber.org/goleak"
)

var testHomeDir string

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
	tempDir := util.MustTempDir("runner_test")
	err := os.Setenv("HOME", tempDir)
	if err != nil {
		panic(err)
	}
	testHomeDir = tempDir
	code := m.Run()
	_ = os.RemoveAll(tempDir)
	os.Exit(code)
}
