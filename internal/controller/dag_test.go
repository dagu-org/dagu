package controller_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/controller"
)

func TestLoadConfig(t *testing.T) {
	file := testConfig("controller_config_error.yaml")
	dag, err := controller.FromConfig(file)
	require.Error(t, err)
	require.NotNil(t, dag)
	require.Error(t, dag.Error)
	require.Equal(t, file, dag.Config.ConfigPath)
}
