package admin

import (
	"os"
	"path"
	"testing"

	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var testsDir = path.Join(utils.MustGetwd(), "../../tests/admin/")

func TestMain(m *testing.M) {
	os.Setenv("HOST", "localhost")
	settings.InitTest(testsDir)
	code := m.Run()
	os.Exit(code)
}
