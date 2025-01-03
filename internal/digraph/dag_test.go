package digraph

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/stretchr/testify/require"
)

var (
	testdataDir = filepath.Join(fileutil.MustGetwd(), "testdata")
)

func TestDAG_String(t *testing.T) {
	t.Run("DefaltConfig", func(t *testing.T) {
		filePath := filepath.Join(testdataDir, "default.yaml")
		dag, err := Load(context.Background(), filePath)
		require.NoError(t, err)

		ret := dag.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestDAG_SockAddr(t *testing.T) {
	t.Run("UnixSocketLocation", func(t *testing.T) {
		dag := &DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, dag.SockAddr())
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 108 is the maximum length of a unix socket address
		require.Greater(t, 108, len(dag.SockAddr()))
		require.Equal(
			t,
			"/tmp/@dagu-testDagVeryLongNameThatExceedsUnixSocketLengthMax-b92b711162d6012f025a76d0cf0b40c2.sock",
			dag.SockAddr(),
		)
	})
}
