package admin

import (
	"os"
	"path"
	"testing"

	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/utils"
)

var testdataDir = path.Join(utils.MustGetwd(), "testdata")
var testHomeDir string

func TestMain(m *testing.M) {
	testHomeDir = utils.MustTempDir("dagu-admin-test")
	os.Setenv("HOST", "localhost")
	changeHomeDir(testdataDir)
	code := m.Run()
	_ = os.RemoveAll(testHomeDir)
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	os.Setenv("HOME", homeDir)
	_ = config.LoadConfig(homeDir)
}
