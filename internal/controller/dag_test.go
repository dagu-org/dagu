package controller_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
)

func TestLoadConfig(t *testing.T) {
	file := testDAG("controller_config_error.yaml")
	dr := controller.NewDAGReader()
	dag, err := dr.ReadDAG(file, false)
	require.Error(t, err)
	require.NotNil(t, dag)
	require.Error(t, dag.Error)
	require.Equal(t, file, dag.DAG.ConfigPath)
}
