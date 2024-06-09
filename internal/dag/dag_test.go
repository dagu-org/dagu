package dag

import (
	"os"
	"path"
	"testing"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var (
	testdataDir = path.Join(util.MustGetwd(), "testdata")
	testHomeDir = path.Join(util.MustGetwd(), "testdata/home")
)

func TestMain(m *testing.M) {
	changeHomeDir(testHomeDir)
	code := m.Run()
	os.Exit(code)
}

func changeHomeDir(homeDir string) {
	_ = os.Setenv("HOME", homeDir)
	_ = config.LoadConfig()
}

func TestDAG_String(t *testing.T) {
	t.Run("String representation of default.yaml", func(t *testing.T) {
		d, err := Load("", path.Join(testdataDir, "default.yaml"), "")
		require.NoError(t, err)

		ret := d.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestDAG_SockAddr(t *testing.T) {
	t.Run("Unix Socket", func(t *testing.T) {
		d := &DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, d.SockAddr())
	})
}
