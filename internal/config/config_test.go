package config_test

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagman/internal/config"
	"github.com/yohamta/dagman/internal/settings"
	"github.com/yohamta/dagman/internal/utils"
)

var (
	testDir     = path.Join(utils.MustGetwd(), "../../tests/testdata")
	testHomeDir = path.Join(utils.MustGetwd(), "../../tests/config")
	testConfig  = path.Join(testDir, "config_load.yaml")
	testEnv     = []string{}
)

func TestMain(m *testing.M) {
	settings.InitTest(testHomeDir)
	testEnv = []string{
		fmt.Sprintf("LOG_DIR=%s", path.Join(testHomeDir, "/logs")),
		fmt.Sprintf("PATH=%s", os.ExpandEnv("${PATH}")),
	}
	code := m.Run()
	os.Exit(code)
}

func TestAssertDefinition(t *testing.T) {
	loader := config.NewConfigLoader()

	_, err := loader.Load(path.Join(testDir, "config_err_no_name.yaml"), "")
	require.Equal(t, err, fmt.Errorf("DAG name must be specified."))

	_, err = loader.Load(path.Join(testDir, "config_err_no_steps.yaml"), "")
	require.Equal(t, err, fmt.Errorf("at least one step must be specified."))
}

func TestAssertStepDefinition(t *testing.T) {
	loader := config.NewConfigLoader()

	_, err := loader.Load(path.Join(testDir, "config_err_step_no_name.yaml"), "")
	require.Equal(t, err, fmt.Errorf("step name must be specified."))

	_, err = loader.Load(path.Join(testDir, "config_err_step_no_command.yaml"), "")
	require.Equal(t, err, fmt.Errorf("step command must be specified."))
}
