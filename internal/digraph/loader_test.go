package digraph_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	t.Run(("WithName"), func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "loader_test.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG, digraph.WithName("testDAG"))
		require.NoError(t, err)
		require.Equal(t, "testDAG", dag.Name)
	})
	t.Run("InvalidPath", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "not_existing_file.yaml"))
		_, err := digraph.Load(context.Background(), testDAG)
		require.Error(t, err)
	})
	t.Run("UnknownField", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "err_decode.yaml"))
		_, err := digraph.Load(context.Background(), testDAG)
		require.Error(t, err)
		require.Contains(t, err.Error(), "has invalid keys: invalidKey")
	})
	t.Run("InvalidYAML", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "err_parse.yaml"))
		_, err := digraph.Load(context.Background(), testDAG)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalidyaml")
	})
	t.Run("MetadataOnly", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "loader_test.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG, digraph.OnlyMetadata())
		require.NoError(t, err)
		require.Empty(t, dag.Steps)
		// Check if the metadata is loaded correctly
		require.Equal(t, "loader_test", dag.Name)
		require.Len(t, dag.Steps, 0)
	})
	t.Run("DefaultConfig", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "default.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG)

		require.NoError(t, err)

		// DAG level
		assert.Equal(t, "", dag.LogDir)
		assert.Equal(t, testDAG, dag.Location)
		assert.Equal(t, "default", dag.Name)
		assert.Equal(t, time.Second*60, dag.MaxCleanUpTime)
		assert.Equal(t, 30, dag.HistRetentionDays)

		// Step level
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "1", dag.Steps[0].Name, "1")
		assert.Equal(t, "true", dag.Steps[0].Command, "true")
		assert.Equal(t, filepath.Dir(testDAG), dag.Steps[0].Dir)
	})
	t.Run("OverrideConfig", func(t *testing.T) {
		t.Parallel()

		// Base config has the following values:
		// MailOn: {Failure: true, Success: false}
		base := test.TestdataPath(t, filepath.Join("digraph", "base.yaml"))
		// Overwrite the base config with the following values:
		// MailOn: {Failure: false, Success: true}
		testDAG := test.TestdataPath(t, filepath.Join("digraph", "override.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG, digraph.WithBaseConfig(base))
		require.NoError(t, err)

		// Check if the MailOn values are overwritten
		assert.False(t, dag.MailOn.Failure)
		assert.True(t, dag.MailOn.Success)
	})
}

func TestLoadBaseConfig(t *testing.T) {
	t.Parallel()

	t.Run("LoadBaseConfigFile", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "base.yaml"))
		dag, err := digraph.LoadBaseConfig(digraph.BuildContext{}, testDAG)
		require.NotNil(t, dag)
		require.NoError(t, err)
	})
	t.Run("InheritBaseConfig", func(t *testing.T) {
		t.Parallel()

		baseDAG := test.TestdataPath(t, filepath.Join("digraph", "inherit_base.yaml"))
		testDAG := test.TestdataPath(t, filepath.Join("digraph", "inherit_child.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG, digraph.WithBaseConfig(baseDAG))
		require.NotNil(t, dag)
		require.NoError(t, err)

		// Check if fields are inherited correctly
		assert.Equal(t, "/base/logs", dag.LogDir)
	})
}

func TestLoadYAML(t *testing.T) {
	t.Parallel()
	const testDAG = `
name: test DAG
steps:
  - name: "1"
    command: "true"
`
	t.Run("ValidYAMLData", func(t *testing.T) {
		t.Parallel()

		ret, err := digraph.LoadYAMLWithOpts(context.Background(), []byte(testDAG), digraph.BuildOpts{})
		require.NoError(t, err)
		require.Equal(t, "test DAG", ret.Name)

		step := ret.Steps[0]
		require.Equal(t, "1", step.Name)
		require.Equal(t, "true", step.Command)
	})
	t.Run("InvalidYAMLData", func(t *testing.T) {
		t.Parallel()

		_, err := digraph.LoadYAMLWithOpts(context.Background(), []byte(`invalid`), digraph.BuildOpts{})
		require.Error(t, err)
	})
}

func TestLoadYAMLWithNameOption(t *testing.T) {
	t.Parallel()
	const testDAG = `
steps:
  - name: "1"
    command: "true"
`

	ret, err := digraph.LoadYAMLWithOpts(context.Background(), []byte(testDAG), digraph.BuildOpts{
		Name: "testDAG",
	})
	require.NoError(t, err)
	require.Equal(t, "testDAG", ret.Name)

	step := ret.Steps[0]
	require.Equal(t, "1", step.Name)
	require.Equal(t, "true", step.Command)
}
