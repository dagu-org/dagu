package dag

import (
	"os"
	"path"
	"testing"

	"github.com/dagu-dev/dagu/internal/util"
	"github.com/stretchr/testify/require"
)

var (
	testdataDir = path.Join(util.MustGetwd(), "testdata")
	testHomeDir = path.Join(util.MustGetwd(), "testdata/home")
)

func TestMain(m *testing.M) {
	err := os.Setenv("HOME", testHomeDir)
	if err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestDAG_String(t *testing.T) {
	t.Run("String representation of default.yaml", func(t *testing.T) {
		loader := NewLoader()
		dg, err := loader.Load("", path.Join(testdataDir, "default.yaml"), "")
		require.NoError(t, err)

		ret := dg.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestDAG_SockAddr(t *testing.T) {
	t.Run("Unix Socket", func(t *testing.T) {
		d := &DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, d.SockAddr())
	})
	t.Run("Unix Socket", func(t *testing.T) {
		d := &DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 108 is the maximum length of a unix socket address
		require.Greater(t, 108, len(d.SockAddr()))
		require.Equal(
			t,
			"/tmp/@dagu-testDagVeryLongNameThatExceedsUnixSocketLengthMax-b92b711162d6012f025a76d0cf0b40c2.sock",
			d.SockAddr(),
		)
	})
}
