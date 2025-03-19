package digraph_test

import (
	"encoding/json"
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
		require.Regexp(t, `^/tmp/@dagu_testDag_[0-9a-f]+\.sock$`, dag.SockAddr())
	})
	t.Run("MaxUnixSocketLength", func(t *testing.T) {
		dag := &digraph.DAG{
			Location: "testdata/testDagVeryLongNameThatExceedsUnixSocketLengthMaximum-testDagVeryLongNameThatExceedsUnixSocketLengthMaximum.yml",
		}
		// 50 is the maximum length of a unix socket address
		require.LessOrEqual(t, 50, len(dag.SockAddr()))
		require.Equal(
			t,
			"/tmp/@dagu_testDagVeryLongNameThatExceedsUn_e959f2.sock",
			dag.SockAddr(),
		)
	})
}

func TestMashalJSON(t *testing.T) {
	th := test.Setup(t)
	t.Run("MarshalJSON", func(t *testing.T) {
		dag := th.DAG(t, filepath.Join("digraph", "default.yaml"))
		dat, err := json.Marshal(dag.DAG)
		require.NoError(t, err)
		println(string(dat))
	})
}
