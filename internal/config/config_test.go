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

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.Error(t, err)
	}
}

func TestLoadEnv(t *testing.T) {
	for _, c := range []struct {
		val, key, want string
	}{
		{
			`env: 
  VAR: "` + "`echo 1`" + `"
`,
			"VAR", "1",
		},
		{
			`env: 
  "1": "123"
`,
			"1", "123",
		},
		{
			`env: 
  - "FOO": "BAR"
  - "FOO": "${FOO}:BAZ"
  - "FOO": "${FOO}:BAR"
  - "FOO": "${FOO}:FOO"
`,
			"FOO", "BAR:BAZ:BAR:FOO",
		},
	} {
		l := &Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		d, err := l.unmarshalData([]byte(c.val))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		require.Equal(t, c.want, os.Getenv(c.key))
	}
}

func TestParseParameter(t *testing.T) {
	for _, test := range []struct {
		Params string
		Env    string
		Want   map[string]string
	}{
		{
			Params: "P1=foo P2=${FOO} P3=`/bin/echo 1 X=` bar",
			Env:    "FOO: BAR",
			Want: map[string]string{
				"P1": "foo",
				"P2": "BAR",
				"P3": "1",
				"X":  "",
				"1":  "P1=foo",
				"2":  "P2=BAR",
				"3":  "P3=1",
				"4":  "X=",
				"5":  "bar",
			},
		},
	} {
		l := &Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		d, err := l.unmarshalData([]byte(fmt.Sprintf(`
env:
  - %s
params: %s
  	`, test.Env, test.Params)))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		for k, v := range test.Want {
			require.Equal(t, v, os.Getenv(k))
		}
	}
}

func TestTags(t *testing.T) {
	for _, test := range []struct {
		Tags string
		Want []string
	}{
		{
			Tags: "Daily, Monthly",
			Want: []string{"daily", "monthly"},
		},
	} {
		l := &Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		d, err := l.unmarshalData([]byte(fmt.Sprintf(`
tags: %s
  	`, test.Tags)))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		cfg, err := b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		require.Equal(t, test.Want, cfg.Tags)
	}
}
