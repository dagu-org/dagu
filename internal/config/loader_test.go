package config

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/utils"
)

func TestLoadConfig(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	_, err := l.Load(testConfig, "")
	require.NoError(t, err)
}

func TestLoadGlobalConfig(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	cfg, err := l.loadGlobalConfig(
		path.Join(l.HomeDir, ".dagu/config.yaml"),
		&BuildConfigOptions{},
	)
	require.NotNil(t, cfg)
	require.NoError(t, err)
}

func TestLoadGlobalConfigError(t *testing.T) {
	for _, path := range []string{
		path.Join(testDir, "config_err_decode.yaml"),
		path.Join(testDir, "config_err_parse.yaml"),
	} {
		l := &Loader{HomeDir: utils.MustGetUserHomeDir()}
		_, err := l.loadGlobalConfig(path, &BuildConfigOptions{})
		require.Error(t, err)
	}
}

func TestLoadDeafult(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	cfg, err := l.Load(path.Join(testDir, "config_default.yaml"), "")
	require.NoError(t, err)

	assert.Equal(t, time.Second*60, cfg.MaxCleanUpTime)
	assert.Equal(t, 7, cfg.HistRetentionDays)
}

func TestLoadData(t *testing.T) {
	dat := `name: test DAG
steps:
  - name: "1"
    command: "true"
`
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	ret, err := l.LoadData([]byte(dat))
	require.NoError(t, err)
	require.Equal(t, ret.Name, "test DAG")

	step := ret.Steps[0]
	require.Equal(t, step.Name, "1")
	require.Equal(t, step.Command, "true")

	// error
	dat = `invalidyaml`
	_, err = l.LoadData([]byte(dat))
	require.Error(t, err)

	// error
	dat = `invalidkey: test DAG`
	_, err = l.LoadData([]byte(dat))
	require.Error(t, err)

	// error
	dat = `name: test DAG`
	_, err = l.LoadData([]byte(dat))
	require.Error(t, err)
}

func TestLoadErrorFileNotExist(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	_, err := l.Load(path.Join(testDir, "not_existing_file.yaml"), "")
	require.Error(t, err)
}

func TestGlobalConfigNotExist(t *testing.T) {
	l := &Loader{
		HomeDir: "/not_existing_dir",
	}

	file := path.Join(testDir, "config_default.yaml")
	_, err := l.Load(file, "")
	require.NoError(t, err)
}

func TestDecodeError(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	file := path.Join(testDir, "config_err_decode.yaml")
	_, err := l.Load(file, "")
	require.Error(t, err)
}
