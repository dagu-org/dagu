package digraph_test

import (
	"path/filepath"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestDAG(t *testing.T) {
	th := test.Setup(t)
	t.Run("String", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("digraph", "default.yaml"))
		ret := dag.String()
		require.Contains(t, ret, "Name: default")
	})
}

func TestUnixSocket(t *testing.T) {
	t.Run("Location", func(t *testing.T) {
		dag := &digraph.DAG{Location: "testdata/testDag.yml"}
		require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, dag.SockAddr())
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &digraph.DAG{
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
