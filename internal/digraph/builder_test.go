package digraph

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testHelper struct {
	t         *testing.T
	buildOpts buildOpts
	*DAG
}

func (th *testHelper) AssertEnv(t *testing.T, key, val string) {
	th.t.Helper()

	for _, env := range th.Env {
		if env == key+"="+val {
			return
		}
	}
	t.Errorf("expected env %s=%s not found", key, val)
}

func (th *testHelper) AssertParam(t *testing.T, params ...string) {
	th.t.Helper()

	assert.Len(t, th.Params, len(params), "expected %d params, got %d", len(params), len(th.Params))
	for i, p := range params {
		assert.Equal(t, p, th.Params[i])
	}
}

type testOption func(*testHelper)

func withBuildOpts(opts buildOpts) testOption {
	return func(th *testHelper) {
		th.buildOpts = opts
	}
}

func loadTestYAML(t *testing.T, inputFile string, opts ...testOption) *testHelper {
	t.Helper()
	ctx := context.Background()

	th := &testHelper{t: t}

	for _, opt := range opts {
		opt(th)
	}

	dag, err := loadYAML(ctx, readTestFile(t, inputFile), th.buildOpts)
	require.NoError(t, err, "failed to load YAML %s", inputFile)

	th.DAG = dag
	return th
}

func loadTestYAMLError(t *testing.T, inputFile string, expectedErr error, opts ...testOption) {
	t.Helper()
	ctx := context.Background()

	th := &testHelper{t: t}

	for _, opt := range opts {
		opt(th)
	}

	_, err := loadYAML(ctx, readTestFile(t, inputFile), th.buildOpts)
	assert.Error(t, err, "expected error %v, got nil", expectedErr)
	if errs, ok := err.(*errorList); ok && len(*errs) > 0 {
		// check if the error is in the list of errors
		found := false
		for _, e := range *errs {
			if errors.Is(e, expectedErr) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error %v, got %v", expectedErr, err)
		}
	} else if !errors.Is(err, expectedErr) {
		t.Errorf("expected error %v, got %v", expectedErr, err)
	}
}

func TestBuildDAGError(t *testing.T) {
	t.Parallel()
	t.Run("NoName", func(t *testing.T) {
		loadTestYAMLError(t, "invalid_no_name.yaml", errStepNameRequired)
	})
	t.Run("InvalidEnv", func(t *testing.T) {
		loadTestYAMLError(t, "invalid_env.yaml", errInvalidEnvValue)
	})
	t.Run("InvalidParams", func(t *testing.T) {
		loadTestYAMLError(t, "invalid_params.yaml", errInvalidParamValue)
	})
	t.Run("InvalidSchedule", func(t *testing.T) {
		loadTestYAMLError(t, "invalid_schedule.yaml", errInvalidSchedule)
	})
}

func TestBuildStepError(t *testing.T) {
	t.Parallel()
	t.Run("NoCommand", func(t *testing.T) {
		loadTestYAMLError(t, "invalid_no_command.yaml", errStepCommandIsRequired)
	})
}

func TestBuildDAG(t *testing.T) {
	t.Parallel()
	t.Run("ValidEnv", func(t *testing.T) {
		th := loadTestYAML(t, "valid_env.yaml")
		th.AssertEnv(t, "FOO", "123")
	})
	t.Run("ValidEnvWithSubstitution", func(t *testing.T) {
		th := loadTestYAML(t, "valid_env_substitution.yaml")
		// find the env key in the map
		var found bool
		for _, env := range th.Env {
			if env == "VAR=123" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected env key not found")
	})
	t.Run("ValidEnvWithSubstitutionAndEnv", func(t *testing.T) {
		th := loadTestYAML(t, "valid_env_substitution_and_env.yaml")
		// find the env key in the map
		var found bool
		for _, env := range th.Env {
			if env == "FOO=BAR:BAZ:BAR:FOO" {
				found = true
				break
			}
		}
		assert.True(t, found, "expected env key not found")
	})
	t.Run("ValidSchedule", func(t *testing.T) {
		th := loadTestYAML(t, "valid_schedule.yaml")
		assert.Len(t, th.Schedule, 1)
		assert.Equal(t, "0 1 * * *", th.Schedule[0].Expression)

		assert.Len(t, th.StopSchedule, 1)
		assert.Equal(t, "0 2 * * *", th.StopSchedule[0].Expression)

		assert.Len(t, th.RestartSchedule, 1)
		assert.Equal(t, "0 12 * * *", th.RestartSchedule[0].Expression)
	})
	t.Run("ScheduleInList", func(t *testing.T) {
		th := loadTestYAML(t, "schedule_in_list.yaml")
		assert.Len(t, th.Schedule, 2)
		assert.Equal(t, "0 1 * * *", th.Schedule[0].Expression)
		assert.Equal(t, "0 18 * * *", th.Schedule[1].Expression)
	})
	t.Run("ScheduleWithMultipleValues", func(t *testing.T) {
		th := loadTestYAML(t, "schedule_with_multiple_values.yaml")
		assert.Len(t, th.Schedule, 2)
		assert.Equal(t, "0 1 * * *", th.Schedule[0].Expression)
		assert.Equal(t, "0 18 * * *", th.Schedule[1].Expression)

		assert.Len(t, th.StopSchedule, 2)
		assert.Equal(t, "0 2 * * *", th.StopSchedule[0].Expression)
		assert.Equal(t, "0 20 * * *", th.StopSchedule[1].Expression)

		assert.Len(t, th.RestartSchedule, 2)
		assert.Equal(t, "0 12 * * *", th.RestartSchedule[0].Expression)
		assert.Equal(t, "0 22 * * *", th.RestartSchedule[1].Expression)
	})
	t.Run("SkipIfSuccessful", func(t *testing.T) {
		th := loadTestYAML(t, "skip_if_successful.yaml")
		assert.True(t, th.SkipIfSuccessful)
	})
	t.Run("ParamsWithSubstitution", func(t *testing.T) {
		th := loadTestYAML(t, "params_with_substitution.yaml")
		th.AssertParam(t, "x", "x")
	})
	t.Run("ParamsWithQuotedValues", func(t *testing.T) {
		th := loadTestYAML(t, "params_with_quoted_values.yaml")
		th.AssertParam(t, "x=a b c", "y=d e f")
	})
	t.Run("ParamsAsMap", func(t *testing.T) {
		th := loadTestYAML(t, "params_as_map.yaml")
		th.AssertParam(t,
			"FOO=foo",
			"BAR=bar",
			"BAZ=baz",
		)
	})
	t.Run("ParamsAsMapOverride", func(t *testing.T) {
		th := loadTestYAML(t, "params_as_map.yaml", withBuildOpts(
			buildOpts{
				parameters: "FOO=X BAZ=Y",
			},
		))
		th.AssertParam(t,
			"FOO=X",
			"BAR=bar",
			"BAZ=Y",
		)
	})
	t.Run("ParamsWithComplexValues", func(t *testing.T) {
		th := loadTestYAML(t, "params_with_complex_values.yaml")
		th.AssertParam(t,
			"first",
			"P1=foo",
			"P2=TEXT",
			"P3=BAR",
			"X=bar",
			"Y=foo",
			"Z=A B C",
		)
	})
	t.Run("mailOn", func(t *testing.T) {
		th := loadTestYAML(t, "valid_mail_on.yaml")
		assert.True(t, th.MailOn.Failure)
		assert.True(t, th.MailOn.Success)
	})
	t.Run("ValidTags", func(t *testing.T) {
		th := loadTestYAML(t, "valid_tags.yaml")
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("ValidTagsList", func(t *testing.T) {
		th := loadTestYAML(t, "valid_tags_list.yaml")
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("LogDir", func(t *testing.T) {
		th := loadTestYAML(t, "valid_log_dir.yaml")
		assert.Equal(t, "/tmp/logs", th.LogDir)
	})
	t.Run("MailConfig", func(t *testing.T) {
		th := loadTestYAML(t, "valid_mail_config.yaml")
		assert.Equal(t, "smtp.example.com", th.SMTP.Host)
		assert.Equal(t, "587", th.SMTP.Port)
		assert.Equal(t, "user@example.com", th.SMTP.Username)
		assert.Equal(t, "password", th.SMTP.Password)

		assert.Equal(t, "error@example.com", th.ErrorMail.From)
		assert.Equal(t, "admin@example.com", th.ErrorMail.To)
		assert.Equal(t, "[ERROR]", th.ErrorMail.Prefix)
		assert.True(t, th.ErrorMail.AttachLogs)

		assert.Equal(t, "info@example.com", th.InfoMail.From)
		assert.Equal(t, "user@example.com", th.InfoMail.To)
		assert.Equal(t, "[INFO]", th.InfoMail.Prefix)
		assert.True(t, th.InfoMail.AttachLogs)
	})
	t.Run("MaxHistRetentionDays", func(t *testing.T) {
		th := loadTestYAML(t, "hist_retention_days.yaml")
		assert.Equal(t, 365, th.HistRetentionDays)
	})
	t.Run("CleanUpTime", func(t *testing.T) {
		th := loadTestYAML(t, "max_cleanup_time.yaml")
		assert.Equal(t, time.Duration(10*time.Second), th.MaxCleanUpTime)
	})
	t.Run("Preconditions", func(t *testing.T) {
		th := loadTestYAML(t, "preconditions.yaml")
		assert.Len(t, th.Preconditions, 1)
		assert.Equal(t, Condition{Condition: "test -f file.txt", Expected: "true"}, th.Preconditions[0])
	})
	t.Run("MaxActiveRuns", func(t *testing.T) {
		th := loadTestYAML(t, "max_active_runs.yaml")
		assert.Equal(t, 3, th.MaxActiveRuns)
	})
}

func TestBuildStep(t *testing.T) {
	t.Parallel()
	t.Run("ValidCommand", func(t *testing.T) {
		th := loadTestYAML(t, "valid_command.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "echo 1", th.Steps[0].CmdWithArgs)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step 1", th.Steps[0].Name)
	})
	t.Run("ValidCommandInArray", func(t *testing.T) {
		th := loadTestYAML(t, "valid_command_in_array.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step 1", th.Steps[0].Name)
	})
	t.Run("ValidCommandInList", func(t *testing.T) {
		th := loadTestYAML(t, "valid_command_in_list.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step 1", th.Steps[0].Name)
	})
	t.Run("HTTPExecutor", func(t *testing.T) {
		th := loadTestYAML(t, "http_executor.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
	})
	t.Run("HTTPExecutorWithConfig", func(t *testing.T) {
		th := loadTestYAML(t, "http_executor_with_config.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, map[string]any{
			"key": "value",
			"map": map[string]any{
				"foo": "bar",
			},
		}, th.Steps[0].ExecutorConfig.Config)
	})
	t.Run("SubWorkflow", func(t *testing.T) {
		th := loadTestYAML(t, "subworkflow.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "subworkflow", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "run", th.Steps[0].Command)
		assert.Equal(t, []string{
			"sub_dag",
			"param1=value1 param2=value2",
		}, th.Steps[0].Args)
	})
	t.Run("ContinueOn", func(t *testing.T) {
		th := loadTestYAML(t, "continue_on.yaml")
		assert.Len(t, th.Steps, 1)
		assert.True(t, th.Steps[0].ContinueOn.Failure)
		assert.True(t, th.Steps[0].ContinueOn.Skipped)
	})
	t.Run("RetryPolicy", func(t *testing.T) {
		th := loadTestYAML(t, "retry_policy.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 3, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 10*time.Second, th.Steps[0].RetryPolicy.Interval)
	})
	t.Run("RepeatPolicy", func(t *testing.T) {
		th := loadTestYAML(t, "repeat_policy.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RepeatPolicy)
		assert.True(t, th.Steps[0].RepeatPolicy.Repeat)
		assert.Equal(t, 60*time.Second, th.Steps[0].RepeatPolicy.Interval)
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		th := loadTestYAML(t, "signal_on_stop.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "SIGINT", th.Steps[0].SignalOnStop)
	})
	t.Run("Preconditions", func(t *testing.T) {
		th := loadTestYAML(t, "step_preconditions.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, Condition{Condition: "test -f file.txt", Expected: "true"}, th.Steps[0].Preconditions[0])
	})
}

func TestOverrideBaseConfig(t *testing.T) {
	// Base config has the following values:
	// MailOn: {Failure: true, Success: false}
	t.Run("Override", func(t *testing.T) {
		baseConfig := filepath.Join(testdataDir, "base.yaml")

		// Overwrite the base config with the following values:
		// MailOn: {Failure: false, Success: false}
		filePath := filepath.Join(testdataDir, "overwrite.yaml")
		dag, err := Load(context.Background(), filePath, WithBaseConfig(baseConfig))
		require.NoError(t, err)

		// The MailOn key should be overwritten.
		require.Equal(t, &MailOn{Failure: false, Success: false}, dag.MailOn)
		require.Equal(t, dag.HistRetentionDays, 7)
	})
	t.Run("WithoutOverride", func(t *testing.T) {
		baseConfig := filepath.Join(testdataDir, "base.yaml")

		// no_overwrite.yaml does not have the MailOn key.
		filePath := filepath.Join(testdataDir, "no_overwrite.yaml")
		dag, err := Load(context.Background(), filePath, WithBaseConfig(baseConfig))
		require.NoError(t, err)

		// The MailOn key should be the same as the base config.
		require.Equal(t, &MailOn{Failure: true, Success: false}, dag.MailOn)
		require.Equal(t, dag.HistRetentionDays, 30)
	})
}

func readTestFile(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(testdataDir, filename))
	require.NoError(t, err)
	return data
}
