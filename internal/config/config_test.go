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
	settings.ChangeHomeDir(testHomeDir)
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

	_, err := l.Load(path.Join(testDir, "config_err_no_steps.yaml"), "")
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

	cfg2 := cfg.Clone()
	require.Equal(t, cfg, cfg2)
}

func TestConfigLoadHeadOnly(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	cfg, err := l.LoadHeadOnly(path.Join(testDir, "config_default.yaml"))
	require.NoError(t, err)

	require.Equal(t, cfg.Name, "config_default")
	require.True(t, len(cfg.Steps) == 0)
}

func TestLoadInvalidConfigError(t *testing.T) {
	for _, c := range []string{
		`env: 
  VAR: "` + "`ech 1`" + `"
`,
		`logDir: "` + "`ech foo`" + `"`,
		`params: "` + "`ech foo`" + `"`,
		`schedule: "` + "1" + `"`,
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
			Params: "x",
			Want: map[string]string{
				"1": "x",
			},
		},
		{
			Params: "x y",
			Want: map[string]string{
				"1": "x",
				"2": "y",
			},
		},
		{
			Params: "x yy zzz",
			Want: map[string]string{
				"1": "x",
				"2": "yy",
				"3": "zzz",
			},
		},
		{
			Params: "x $1",
			Want: map[string]string{
				"1": "x",
				"2": "x",
			},
		},
		{
			Params: "first P1=foo P2=${FOO} P3=`/bin/echo ${P2}` X=bar Y=${P1} Z=\"A B C\"",
			Env:    "FOO: BAR",
			Want: map[string]string{
				"P1": "foo",
				"P2": "BAR",
				"P3": "BAR",
				"X":  "bar",
				"Y":  "foo",
				"Z":  "A B C",
				"1":  "first",
				"2":  "P1=foo",
				"3":  "P2=BAR",
				"4":  "P3=BAR",
				"5":  "X=bar",
				"6":  "Y=foo",
				"7":  "Z=A B C",
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

func TestExpandEnv(t *testing.T) {
	b := &builder{}
	os.Setenv("FOO", "BAR")
	require.Equal(t, b.expandEnv("${FOO}"), "BAR")

	b.noEval = true
	require.Equal(t, b.expandEnv("${FOO}"), "${FOO}")
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

		require.True(t, cfg.HasTag("daily"))
	}
}

func TestSchedule(t *testing.T) {
	for _, test := range []struct {
		Def  string
		Err  bool
		Want int
	}{
		{
			Def:  "schedule: \"*/5 * * * *\"",
			Want: 1,
		},
		{
			Def: `schedule:
  - "*/5 * * * *"
  - "* * * * *"`,
			Want: 2,
		},
		{
			Def: `schedule:
  - true 
  - "* * * * *"`,
			Err: true,
		},
	} {
		l := &Loader{
			HomeDir: utils.MustGetUserHomeDir(),
		}
		d, err := l.unmarshalData([]byte(test.Def))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		cfg, err := b.buildFromDefinition(def, nil)

		if test.Err {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Equal(t, test.Want, len(cfg.Schedule))
		}

	}
}

func TestSockAddr(t *testing.T) {
	cfg := &Config{ConfigPath: "testdata/testDag.yml"}
	require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, cfg.SockAddr())
}

func TestOverwriteGlobalConfig(t *testing.T) {
	l := &Loader{HomeDir: utils.MustGetUserHomeDir()}

	cfg, err := l.Load(path.Join(testDir, "config_overwrite.yaml"), "")
	require.NoError(t, err)

	require.Equal(t, &MailOn{Failure: false, Success: false}, cfg.MailOn)
	require.Equal(t, cfg.HistRetentionDays, 7)

	cfg, err = l.Load(path.Join(testDir, "config_no_overwrite.yaml"), "")
	require.NoError(t, err)

	require.Equal(t, &MailOn{Failure: true, Success: false}, cfg.MailOn)
	require.Equal(t, cfg.HistRetentionDays, 30)
}
