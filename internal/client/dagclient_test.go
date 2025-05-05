package client_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
)

func TestDAGClient(t *testing.T) {
	t.Parallel()

	th := test.Setup(t)

	t.Run("Update", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient

		// valid DAG
		validDAG := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		// Update Error: the DAG does not exist
		err := cli.UpdateDAG(ctx, "non-existing-dag", validDAG)
		require.Error(t, err)

		// create a new DAG file
		id, err := cli.CreateDAG(ctx, "new-dag-file")
		require.NoError(t, err)

		// Update the DAG
		err = cli.UpdateDAG(ctx, id, validDAG)
		require.NoError(t, err)

		// Check the content of the DAG file
		spec, err := cli.GetDAGSpec(ctx, id)
		require.NoError(t, err)
		require.Equal(t, validDAG, spec)
	})
	t.Run("Remove", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient

		spec := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
		id, err := cli.CreateDAG(ctx, "test")
		require.NoError(t, err)
		err = cli.UpdateDAG(ctx, id, spec)
		require.NoError(t, err)

		// check file
		newSpec, err := cli.GetDAGSpec(ctx, id)
		require.NoError(t, err)
		require.Equal(t, spec, newSpec)

		// delete
		err = cli.DeleteDAG(ctx, id)
		require.NoError(t, err)
	})
	t.Run("Create", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient

		id, err := cli.CreateDAG(ctx, "test-dag")
		require.NoError(t, err)

		// Check if the new DAG is actually created.
		filePath := filepath.Join(th.Config.Paths.DAGsDir, id+".yaml")
		dag, err := digraph.Load(ctx, filePath)
		require.NoError(t, err)
		require.Equal(t, "test-dag", dag.Name)
	})
	t.Run("Rename", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient

		// Create a DAG to rename.
		id, err := cli.CreateDAG(ctx, "old_name")
		require.NoError(t, err)
		_, err = cli.GetDAGStatus(ctx, filepath.Join(th.Config.Paths.DAGsDir, id+".yaml"))
		require.NoError(t, err)

		// Rename the file.
		err = cli.MoveDAG(ctx, id, id+"_renamed")

		// Check if the file is renamed.
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(th.Config.Paths.DAGsDir, id+"_renamed.yaml"))
	})
	t.Run("TestClient_Empty", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient
		dag := th.DAG(t, filepath.Join("client", "empty_status.yaml"))

		_, err := cli.GetDAGStatus(ctx, dag.Location)
		require.NoError(t, err)
	})
	t.Run("TestClient_All", func(t *testing.T) {
		th := test.Setup(t)

		ctx := th.Context
		cli := th.DAGClient

		// Create a DAG
		_, err := cli.CreateDAG(ctx, "test-dag1")
		require.NoError(t, err)

		_, err = cli.CreateDAG(ctx, "test-dag2")
		require.NoError(t, err)

		// Get all statuses.
		result, errList, err := cli.ListDAGs(ctx)
		require.NoError(t, err)
		require.Empty(t, errList)
		require.Equal(t, 2, len(result.Items))
	})
	t.Run("InvalidDAGName", func(t *testing.T) {
		ctx := th.Context
		cli := th.DAGClient

		dagStatus, err := cli.GetDAGStatus(ctx, "invalid-dag-name")
		require.Error(t, err)
		require.NotNil(t, dagStatus)

		// Check the status contains error.
		require.Error(t, dagStatus.Error)
	})
}

func TestClient_GetTagList(t *testing.T) {
	th := test.Setup(t)

	ctx := th.Context
	cli := th.DAGClient

	// Create DAG List
	for i := 0; i < 40; i++ {
		spec := ""
		id, err := cli.CreateDAG(ctx, "1test-dag-pagination"+fmt.Sprintf("%d", i))
		require.NoError(t, err)
		if i%2 == 0 {
			spec = "tags: tag1,tag2\nsteps:\n  - name: step1\n    command: echo hello\n"
		} else {
			spec = "tags: tag2,tag3\nsteps:\n  - name: step1\n    command: echo hello\n"
		}
		if err = cli.UpdateDAG(ctx, id, spec); err != nil {
			t.Fatal(err)
		}
	}

	tags, errs, err := cli.GetTagList(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, len(errs))
	require.Equal(t, 3, len(tags))

	mapTags := make(map[string]bool)
	for _, tag := range tags {
		mapTags[tag] = true
	}

	require.True(t, mapTags["tag1"])
	require.True(t, mapTags["tag2"])
	require.True(t, mapTags["tag3"])
}
