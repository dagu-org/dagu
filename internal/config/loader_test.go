package config

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/utils"
)

func TestLoadConfig(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	cfg, err := l.Load(testConfig, "")
	require.NoError(t, err)

	steps := []*Step{
		{
			Name:      "1",
			Dir:       testHomeDir,
			Command:   "true",
			Args:      []string{},
			Variables: testEnv,
			Preconditions: []*Condition{
				{
					Condition: "`echo test`",
					Expected:  "test",
				},
			},
			MailOnError: true,
			ContinueOn: ContinueOn{
				Failure: true,
				Skipped: true,
			},
			RetryPolicy: &RetryPolicy{
				Limit: 2,
			},
			RepeatPolicy: RepeatPolicy{
				Repeat:   true,
				Interval: time.Second * 10,
			},
		},
		{
			Name:          "2",
			Dir:           testDir,
			Command:       "false",
			Args:          []string{},
			Variables:     testEnv,
			Preconditions: []*Condition{},
			ContinueOn: ContinueOn{
				Failure: true,
				Skipped: false,
			},
			Depends: []string{
				"1",
			},
		},
	}

	makeTestStepFunc := func(name string) *Step {
		return &Step{
			Name:          name,
			Dir:           testDir,
			Command:       fmt.Sprintf("%s.sh", name),
			Args:          []string{},
			Variables:     testEnv,
			Preconditions: []*Condition{},
		}
	}

	stepm := map[string]*Step{}
	for _, name := range []string{
		constants.OnExit,
		constants.OnSuccess,
		constants.OnFailure,
		constants.OnCancel,
	} {
		stepm[name] = makeTestStepFunc(name)
	}

	want := &Config{
		ConfigPath:        testConfig,
		Name:              "test DAG",
		Description:       "this is a test DAG.",
		Env:               testEnv,
		LogDir:            path.Join(testHomeDir, "/logs"),
		HistRetentionDays: 3,
		MailOn: MailOn{
			Failure: true,
			Success: true,
		},
		Delay:         time.Second * 1,
		MaxActiveRuns: 1,
		Params:        []string{"param1", "param2"},
		DefaultParams: "param1 param2",
		Smtp: &SmtpConfig{
			Host: "smtp.host",
			Port: "25",
		},
		ErrorMail: &MailConfig{
			From:   "system@mail.com",
			To:     "error@mail.com",
			Prefix: "[ERROR]",
		},
		InfoMail: &MailConfig{
			From:   "system@mail.com",
			To:     "info@mail.com",
			Prefix: "[INFO]",
		},
		Preconditions: []*Condition{
			{
				Condition: "`echo 1`",
				Expected:  "1",
			},
		},
		Steps: steps,
		HandlerOn: HandlerOn{
			Exit:    stepm[constants.OnExit],
			Success: stepm[constants.OnSuccess],
			Failure: stepm[constants.OnFailure],
			Cancel:  stepm[constants.OnCancel],
		},
		MaxCleanUpTime: time.Second * 500,
	}
	assert.Equal(t, want, cfg)
}

func TestLoadGlobalConfig(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	cfg, err := l.loadGlobalConfig(
		path.Join(l.HomeDir, ".dagu/config.yaml"),
	)
	require.NotNil(t, cfg)
	require.NoError(t, err)

	sort.Slice(cfg.Env, func(i, j int) bool {
		return strings.Compare(cfg.Env[i], cfg.Env[j]) < 0
	})

	want := &Config{
		Env:               testEnv,
		LogDir:            path.Join(testHomeDir, "/logs"),
		HistRetentionDays: 7,
		Params:            []string{},
		Steps:             []*Step{},
		Smtp: &SmtpConfig{
			Host: "smtp.host",
			Port: "25",
		},
		ErrorMail: &MailConfig{
			From:   "system@mail.com",
			To:     "error@mail.com",
			Prefix: "[ERROR]",
		},
		InfoMail: &MailConfig{
			From:   "system@mail.com",
			To:     "info@mail.com",
			Prefix: "[INFO]",
		},
		Preconditions: []*Condition{},
	}
	assert.Equal(t, want, cfg)
}

func TestLoadDeafult(t *testing.T) {
	l := &Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}
	cfg, err := l.Load(path.Join(testDir, "config_default.yaml"), "")
	require.NoError(t, err)

	assert.Equal(t, time.Minute*5, cfg.MaxCleanUpTime)
	assert.Equal(t, 7, cfg.HistRetentionDays)
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
