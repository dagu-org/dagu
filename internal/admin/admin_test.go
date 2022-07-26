package admin

import (
	"os"
	"path"
	"testing"

	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var testsDir = path.Join(utils.MustGetwd(), "../../tests/admin/")
var testDAGsDir string

func TestMain(m *testing.M) {
	testDAGsDir = utils.MustTempDir("dagu-admin-test")
	os.Setenv("HOST", "localhost")
	settings.ChangeHomeDir(testsDir)
	code := m.Run()
	_ = os.RemoveAll(testDAGsDir)
	os.Exit(code)
}
