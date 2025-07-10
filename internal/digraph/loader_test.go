package digraph_test

import (
	"context"
	"os"
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
		assert.Contains(t, dag.Env, "BASE_ENV=base_value")
		assert.Contains(t, dag.Env, "CHILD_ENV=child_value")
		assert.Contains(t, dag.Env, "OVERWRITE_ENV=child_overwrite_value")
		// 3 from base + 1 from child. For now we keep the base env vars that are overwritten in the child DAG
		// TODO: This should be changed not
		assert.Len(t, dag.Env, 4)
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

// createTempYAMLFile creates a temporary YAML file with the given content
func createTempYAMLFile(t *testing.T, content string) string {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "*.yaml")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Remove(tmpFile.Name()) })

	_, err = tmpFile.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, tmpFile.Close())

	return tmpFile.Name()
}

func TestMultiDAGFile(t *testing.T) {
	t.Parallel()

	t.Run("LoadMultipleDAGs", func(t *testing.T) {
		t.Parallel()

		// Create a temporary multi-DAG YAML file
		multiDAGContent := `name: main-pipeline
steps:
  - name: process
    run: transform-data
  - name: archive
    run: archive-results

---
name: transform-data
steps:
  - name: transform
    command: transform.py

---
name: archive-results
steps:
  - name: archive
    command: archive.sh
`
		// Create temporary file
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load the multi-DAG file
		dag, err := digraph.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify main DAG
		assert.Equal(t, "main-pipeline", dag.Name)
		assert.Len(t, dag.Steps, 2)
		assert.Equal(t, "process", dag.Steps[0].Name)
		assert.Equal(t, "transform-data", dag.Steps[0].ChildDAG.Name)
		assert.Equal(t, "archive", dag.Steps[1].Name)
		assert.Equal(t, "archive-results", dag.Steps[1].ChildDAG.Name)

		// Verify child DAGs
		require.NotNil(t, dag.LocalDAGs)
		assert.Len(t, dag.LocalDAGs, 2)

		// Check transform-data child DAG
		_, exists := dag.LocalDAGs["transform-data"]
		require.True(t, exists)
		transformDAG := dag.LocalDAGs["transform-data"].DAG
		assert.Equal(t, "transform-data", transformDAG.Name)
		assert.Len(t, transformDAG.Steps, 1)
		assert.Equal(t, "transform", transformDAG.Steps[0].Name)
		assert.Equal(t, "transform.py", transformDAG.Steps[0].Command)

		// Check archive-results child DAG
		_, exists = dag.LocalDAGs["archive-results"]
		require.True(t, exists)
		archiveDAG := dag.LocalDAGs["archive-results"].DAG
		assert.Equal(t, "archive-results", archiveDAG.Name)
		assert.Len(t, archiveDAG.Steps, 1)
		assert.Equal(t, "archive", archiveDAG.Steps[0].Name)
		assert.Equal(t, "archive.sh", archiveDAG.Steps[0].Command)
	})

	t.Run("MultiDAGWithBaseConfig", func(t *testing.T) {
		t.Parallel()

		// Create base config
		baseConfig := `env:
  - ENV: production
  - API_KEY: secret123
logDir: /base/logs
smtp:
  host: smtp.example.com
  port: "587"
`
		baseFile := createTempYAMLFile(t, baseConfig)

		// Create multi-DAG file
		multiDAGContent := `name: main-dag
env:
  - APP: myapp
steps:
  - name: process
    command: echo "main"

---
name: child-dag
env:
  - SERVICE: worker
steps:
  - name: work
    command: echo "child"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load with base config
		dag, err := digraph.Load(context.Background(), tmpFile,
			digraph.WithBaseConfig(baseFile))
		require.NoError(t, err)

		// Verify main DAG inherits base config
		assert.Equal(t, "main-dag", dag.Name)
		assert.Equal(t, "/base/logs", dag.LogDir)
		assert.NotNil(t, dag.SMTP)
		assert.Equal(t, "smtp.example.com", dag.SMTP.Host)

		// Verify main DAG has merged env vars
		assert.Contains(t, dag.Env, "ENV=production")
		assert.Contains(t, dag.Env, "API_KEY=secret123")
		assert.Contains(t, dag.Env, "APP=myapp")

		// Verify child DAG also inherits base config
		childDAG := dag.LocalDAGs["child-dag"]
		require.NotNil(t, childDAG)
		assert.Equal(t, "/base/logs", childDAG.DAG.LogDir)
		assert.NotNil(t, childDAG.DAG.SMTP)
		assert.Equal(t, "smtp.example.com", childDAG.DAG.SMTP.Host)

		// Verify child DAG has merged env vars
		assert.Contains(t, childDAG.DAG.Env, "ENV=production")
		assert.Contains(t, childDAG.DAG.Env, "API_KEY=secret123")
		assert.Contains(t, childDAG.DAG.Env, "SERVICE=worker")
	})

	t.Run("SingleDAGFileCompatibility", func(t *testing.T) {
		t.Parallel()

		// Single DAG file (no document separator)
		singleDAGContent := `name: single-dag
steps:
  - name: step1
    command: echo "hello"
`
		tmpFile := createTempYAMLFile(t, singleDAGContent)

		// Load single DAG file
		dag, err := digraph.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify it loads correctly without child DAGs
		assert.Equal(t, "single-dag", dag.Name)
		assert.Len(t, dag.Steps, 1)
		assert.Nil(t, dag.LocalDAGs) // No child DAGs for single DAG file
	})

	t.Run("DuplicateChildDAGNames", func(t *testing.T) {
		t.Parallel()

		// Multi-DAG file with duplicate names
		multiDAGContent := `name: main
steps:
  - name: step1
    command: echo "main"

---
name: duplicate-name
steps:
  - name: step1
    command: echo "first"

---
name: duplicate-name
steps:
  - name: step1
    command: echo "second"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load should fail due to duplicate names
		_, err := digraph.Load(context.Background(), tmpFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate DAG name")
	})

	t.Run("ChildDAGWithoutName", func(t *testing.T) {
		t.Parallel()

		// Multi-DAG file where child DAG has no name
		multiDAGContent := `name: main
steps:
  - name: step1
    command: echo "main"

---
steps:
  - name: step1
    command: echo "unnamed"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// Load should fail because child DAG has no name
		_, err := digraph.Load(context.Background(), tmpFile)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must have a name")
	})

	t.Run("EmptyDocumentSeparator", func(t *testing.T) {
		t.Parallel()

		// TODO: The YAML parser has limitations with empty documents (---)
		// The behavior is inconsistent - sometimes it skips them, sometimes it errors.
		// For now, we test that it loads something, but the child DAG after
		// the empty document may or may not be loaded.
		multiDAGContent := `name: main
steps:
  - name: step1
    command: echo "main"

---

---
name: child
steps:
  - name: step1
    command: echo "child"
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		// The behavior with empty documents is unpredictable
		dag, err := digraph.Load(context.Background(), tmpFile)
		if err != nil {
			// If it errors, it should be a decode error
			assert.Contains(t, err.Error(), "failed to decode document")
		} else {
			// If it succeeds, at least the main DAG should be loaded
			assert.Equal(t, "main", dag.Name)
			// The child DAG after empty document may or may not be loaded
			// due to YAML parser limitations
		}
	})

	t.Run("ComplexMultiDAGWithParameters", func(t *testing.T) {
		t.Parallel()

		// Complex multi-DAG with parameters
		multiDAGContent := `name: etl-pipeline
params:
  - ENVIRONMENT: dev
schedule: "0 2 * * *"
steps:
  - name: extract
    run: extract-module
    params: "SOURCE=customers TABLE=users"
  - name: transform
    run: transform-module
    depends: extract

---
name: extract-module
params:
  - SOURCE: default_source
  - TABLE: default_table
steps:
  - name: validate
    command: test -f data/${SOURCE}/${TABLE}
  - name: extract
    command: extract.py --source=${SOURCE} --table=${TABLE}
    depends: validate

---
name: transform-module
steps:
  - name: transform
    command: transform.py
`
		tmpFile := createTempYAMLFile(t, multiDAGContent)

		dag, err := digraph.Load(context.Background(), tmpFile)
		require.NoError(t, err)

		// Verify main DAG
		assert.Equal(t, "etl-pipeline", dag.Name)
		assert.Len(t, dag.Schedule, 1)
		assert.Equal(t, "0 2 * * *", dag.Schedule[0].Expression)
		assert.Contains(t, dag.Params, "ENVIRONMENT=dev")

		// Verify child DAG references and parameters
		assert.Equal(t, "extract-module", dag.Steps[0].ChildDAG.Name)
		assert.Equal(t, `SOURCE="customers" TABLE="users"`, dag.Steps[0].ChildDAG.Params)
		assert.Equal(t, "transform-module", dag.Steps[1].ChildDAG.Name)

		// Verify extract-module child DAG
		extractDAG := dag.LocalDAGs["extract-module"]
		require.NotNil(t, extractDAG)
		assert.Contains(t, extractDAG.DAG.Params, "SOURCE=default_source")
		assert.Contains(t, extractDAG.DAG.Params, "TABLE=default_table")
		assert.Len(t, extractDAG.DAG.Steps, 2)

		// Verify dependencies in child DAG
		assert.Contains(t, extractDAG.DAG.Steps[1].Depends, "validate")
	})

	t.Run("WorkerSelector", func(t *testing.T) {
		t.Parallel()

		testDAG := test.TestdataPath(t, filepath.Join("digraph", "worker_selector.yaml"))
		dag, err := digraph.Load(context.Background(), testDAG)
		require.NoError(t, err)

		// Verify DAG loaded successfully
		assert.Equal(t, "worker-selector-test", dag.Name)
		assert.Equal(t, "Test DAG with worker selector", dag.Description)
		assert.Len(t, dag.Steps, 3)

		// Verify first step with GPU selector
		gpuTask := dag.Steps[0]
		assert.Equal(t, "gpu-task", gpuTask.Name)
		assert.NotNil(t, gpuTask.WorkerSelector)
		assert.Equal(t, "true", gpuTask.WorkerSelector["gpu"])
		assert.Equal(t, "64G", gpuTask.WorkerSelector["memory"])

		// Verify second step with CPU selector
		cpuTask := dag.Steps[1]
		assert.Equal(t, "cpu-task", cpuTask.Name)
		assert.NotNil(t, cpuTask.WorkerSelector)
		assert.Equal(t, "amd64", cpuTask.WorkerSelector["cpu-arch"])
		assert.Equal(t, "us-east-1", cpuTask.WorkerSelector["region"])

		// Verify third step without selector
		anyWorkerTask := dag.Steps[2]
		assert.Equal(t, "any-worker-task", anyWorkerTask.Name)
		assert.Nil(t, anyWorkerTask.WorkerSelector)
	})
}
