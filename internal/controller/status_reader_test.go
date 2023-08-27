package controller_test

import (
	"github.com/yohamta/dagu/internal/persistence/jsondb"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
)

func TestLoadConfig(t *testing.T) {
	var (
		file = testDAG("invalid_dag.yaml")
		dr   = controller.NewDAGStatusReader(jsondb.New())
	)

	dag, err := dr.ReadStatus(file, false)
	require.Error(t, err)
	require.NotNil(t, dag)

	// contains error message
	require.Error(t, dag.Error)
}

func TestReadAll(t *testing.T) {
	dr := controller.NewDAGStatusReader(jsondb.New())
	dags, _, err := dr.ReadAllStatus(testdataDir)
	require.NoError(t, err)
	require.Greater(t, len(dags), 0)

	pattern := path.Join(testdataDir, "*.yaml")
	matches, err := filepath.Glob(pattern)
	require.NoError(t, err)
	if len(matches) != len(dags) {
		t.Fatalf("unexpected number of dags: %d", len(dags))
	}
}

func TestReadDAGStatus(t *testing.T) {
	var (
		file = testDAG("read_status.yaml")
		dr   = controller.NewDAGStatusReader(jsondb.New())
	)

	_, err := dr.ReadStatus(file, false)
	require.NoError(t, err)
}
