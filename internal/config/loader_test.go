package config_test

import (
	"fmt"
	"path"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
)

func TestLoadConfig(t *testing.T) {
	loader := config.NewConfigLoader()
	cfg, err := loader.Load(testConfig, "")
	require.NoError(t, err)

	steps := []*config.Step{
		{
			Name:      "1",
			Dir:       testHomeDir,
			Command:   "true",
			Args:      []string{},
			Variables: testEnv,
			Preconditions: []*config.Condition{
				{
					Condition: "`echo test`",
					Expected:  "test",
				},
			},
			MailOnError: true,
			ContinueOn: config.ContinueOn{
				Failure: true,
				Skipped: true,
			},
			RetryPolicy: &config.RetryPolicy{
				Limit: 2,
			},
		},
		{
			Name:          "2",
			Dir:           testDir,
			Command:       "false",
			Args:          []string{},
			Variables:     testEnv,
			Preconditions: []*config.Condition{},
			ContinueOn: config.ContinueOn{
				Failure: true,
				Skipped: false,
			},
			Depends: []string{
				"1",
			},
		},
	}

	makeTestStepFunc := func(name string) *config.Step {
		return &config.Step{
			Name:          name,
			Dir:           testDir,
			Command:       fmt.Sprintf("%s.sh", name),
			Args:          []string{},
			Variables:     testEnv,
			Preconditions: []*config.Condition{},
		}
	}

	stepm := map[string]*config.Step{}
	for _, name := range []string{
		constants.OnExit,
		constants.OnSuccess,
		constants.OnFailure,
		constants.OnCancel,
	} {
		stepm[name] = makeTestStepFunc(name)
	}

	want := &config.Config{
		ConfigPath:        testConfig,
		Name:              "test job",
		Description:       "this is a test job.",
		Env:               testEnv,
		LogDir:            path.Join(testHomeDir, "/logs"),
		HistRetentionDays: 3,
		MailOn: config.MailOn{
			Failure: true,
			Success: true,
		},
		DelaySec:      time.Second * 1,
		MaxActiveRuns: 1,
		Params:        []string{"param1", "param2"},
		DefaultParams: "param1 param2",
		Smtp: &config.SmtpConfig{
			Host: "smtp.host",
			Port: "25",
		},
		ErrorMail: &config.MailConfig{
			From:   "system@mail.com",
			To:     "error@mail.com",
			Prefix: "[ERROR]",
		},
		InfoMail: &config.MailConfig{
			From:   "system@mail.com",
			To:     "info@mail.com",
			Prefix: "[INFO]",
		},
		Preconditions: []*config.Condition{
			{
				Condition: "`echo 1`",
				Expected:  "1",
			},
		},
		Steps: steps,
		HandlerOn: config.HandlerOn{
			Exit:    stepm[constants.OnExit],
			Success: stepm[constants.OnSuccess],
			Failure: stepm[constants.OnFailure],
			Cancel:  stepm[constants.OnCancel],
		},
	}
	assert.Equal(t, cfg, want)
}

func TestLoadGlobalConfig(t *testing.T) {
	loader := config.NewConfigLoader()
	cfg, err := loader.LoadGlobalConfig()
	require.NotNil(t, cfg)
	require.NoError(t, err)

	sort.Slice(cfg.Env, func(i, j int) bool {
		return strings.Compare(cfg.Env[i], cfg.Env[j]) < 0
	})

	want := &config.Config{
		Env:               testEnv,
		LogDir:            path.Join(testHomeDir, "/logs"),
		HistRetentionDays: 7,
		Params:            []string{},
		Steps:             []*config.Step{},
		Smtp: &config.SmtpConfig{
			Host: "smtp.host",
			Port: "25",
		},
		ErrorMail: &config.MailConfig{
			From:   "system@mail.com",
			To:     "error@mail.com",
			Prefix: "[ERROR]",
		},
		InfoMail: &config.MailConfig{
			From:   "system@mail.com",
			To:     "info@mail.com",
			Prefix: "[INFO]",
		},
		Preconditions: []*config.Condition{},
	}
	assert.Equal(t, cfg, want)
}
