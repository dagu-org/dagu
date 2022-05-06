package config

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
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
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	_, err := l.Load(path.Join(testDir, "config_err_no_name.yaml"), "")
	require.Equal(t, err, fmt.Errorf("DAG name must be specified"))

	_, err = l.Load(path.Join(testDir, "config_err_no_steps.yaml"), "")
	require.Equal(t, err, fmt.Errorf("at least one step must be specified"))
}

func TestAssertStepDefinition(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	_, err := l.Load(path.Join(testDir, "config_err_step_no_name.yaml"), "")
	require.Equal(t, err, fmt.Errorf("step name must be specified"))

	_, err = l.Load(path.Join(testDir, "config_err_step_no_command.yaml"), "")
	require.Equal(t, err, fmt.Errorf("step command must be specified"))
}

func TestConfigReadClone(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	cfg, err := l.Load(path.Join(testDir, "config_default.yaml"), "")
	require.NoError(t, err)

	require.Contains(t, cfg.String(), "test DAG")

	require.Equal(t, cfg.Name, "test DAG")

	cfg2 := cfg.Clone()
	require.Equal(t, cfg, cfg2)
}

func TestConfigLoadHeadOnly(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	cfg, err := l.LoadHeadOnly(path.Join(testDir, "config_default.yaml"))
	require.NoError(t, err)

	require.Equal(t, cfg.Name, "test DAG")
	require.True(t, len(cfg.Steps) == 0)
}

func TestLoadInvalidConfigError(t *testing.T) {
	for _, c := range []string{
		`env: 
  VAR: "` + "`ech 1`" + `"
`,
		`logDir: "` + "`ech foo`" + `"`,
		`params: "` + "`ech foo`" + `"`,
	} {
		l := &Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		d, err := l.unmarshalData([]byte(c))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		_, err = buildFromDefinition(def, nil, nil)
		require.Error(t, err)
	}
}
