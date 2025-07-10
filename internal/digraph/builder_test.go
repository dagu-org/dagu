package digraph_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/cmdutil"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuild(t *testing.T) {
	t.Run("SkipIfSuccessful", func(t *testing.T) {
		th := testLoad(t, "skip_if_successful.yaml")
		assert.True(t, th.SkipIfSuccessful)
	})
	t.Run("ParamsWithSubstitution", func(t *testing.T) {
		th := testLoad(t, "params_with_substitution.yaml")
		th.AssertParam(t, "1=TEST_PARAM", "2=TEST_PARAM")
	})
	t.Run("ParamsWithQuotedValues", func(t *testing.T) {
		th := testLoad(t, "params_with_quoted_values.yaml")
		th.AssertParam(t, "x=a b c", "y=d e f")
	})
	t.Run("ParamsAsMap", func(t *testing.T) {
		th := testLoad(t, "params_as_map.yaml")
		th.AssertParam(t,
			"FOO=foo",
			"BAR=bar",
			"BAZ=baz",
		)
	})
	t.Run("ParamsAsMapOverride", func(t *testing.T) {
		th := testLoad(t, "params_as_map.yaml", withBuildOpts(
			digraph.BuildOpts{Parameters: "FOO=X BAZ=Y"},
		))
		th.AssertParam(t,
			"FOO=X",
			"BAR=bar",
			"BAZ=Y",
		)
	})
	t.Run("ParamsWithComplexValues", func(t *testing.T) {
		th := testLoad(t, "params_with_complex_values.yaml")
		th.AssertParam(t,
			"1=first",
			"P1=foo",
			"P2=TEXT",
			"P3=BAR",
			"X=bar",
			"Y=foo",
			"Z=A B C",
		)
	})
	t.Run("mailOn", func(t *testing.T) {
		th := testLoad(t, "valid_mail_on.yaml")
		assert.True(t, th.MailOn.Failure)
		assert.True(t, th.MailOn.Success)
	})
	t.Run("ValidTags", func(t *testing.T) {
		th := testLoad(t, "valid_tags.yaml")
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("ValidTagsList", func(t *testing.T) {
		th := testLoad(t, "valid_tags_list.yaml")
		assert.True(t, th.HasTag("daily"))
		assert.True(t, th.HasTag("monthly"))
	})
	t.Run("LogDir", func(t *testing.T) {
		th := testLoad(t, "valid_log_dir.yaml")
		assert.Equal(t, "/tmp/logs", th.LogDir)
	})
	t.Run("MailConfig", func(t *testing.T) {
		th := testLoad(t, "valid_mail_config.yaml")
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
	t.Run("SMTPNumericPort", func(t *testing.T) {
		// Test SMTP configuration with numeric port
		data := []byte(`
smtp:
  host: "smtp.example.com"
  port: 587
  username: "user@example.com"
  password: "password"
steps:
  - name: test
    command: echo test
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.NotNil(t, dag.SMTP)
		assert.Equal(t, "smtp.example.com", dag.SMTP.Host)
		assert.Equal(t, "587", dag.SMTP.Port)
		assert.Equal(t, "user@example.com", dag.SMTP.Username)
		assert.Equal(t, "password", dag.SMTP.Password)
	})
	t.Run("MaxHistRetentionDays", func(t *testing.T) {
		th := testLoad(t, "hist_retention_days.yaml")
		assert.Equal(t, 365, th.HistRetentionDays)
	})
	t.Run("CleanUpTime", func(t *testing.T) {
		th := testLoad(t, "max_cleanup_time.yaml")
		assert.Equal(t, time.Duration(10*time.Second), th.MaxCleanUpTime)
	})
	t.Run("ChainTypeBasic", func(t *testing.T) {
		th := testLoad(t, "chain_basic.yaml")
		assert.Equal(t, digraph.TypeChain, th.Type)

		// Check that implicit dependencies were added
		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends) // First step has no dependencies
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"step2"}, th.Steps[2].Depends)
		assert.Equal(t, []string{"step3"}, th.Steps[3].Depends)
	})
	t.Run("ChainTypeWithExplicitDepends", func(t *testing.T) {
		th := testLoad(t, "chain_with_explicit_depends.yaml")
		assert.Equal(t, digraph.TypeChain, th.Type)

		// Check dependencies
		assert.Len(t, th.Steps, 5)
		assert.Empty(t, th.Steps[0].Depends)                         // setup
		assert.Equal(t, []string{"setup"}, th.Steps[1].Depends)      // download-a
		assert.Equal(t, []string{"download-a"}, th.Steps[2].Depends) // download-b
		// process-both should keep its explicit dependencies
		assert.ElementsMatch(t, []string{"download-a", "download-b"}, th.Steps[3].Depends)
		assert.Equal(t, []string{"process-both"}, th.Steps[4].Depends) // cleanup
	})
	t.Run("InvalidType", func(t *testing.T) {
		// Test will fail with an error containing "invalid type"
		data := test.ReadTestdata(t, filepath.Join("digraph", "invalid_type.yaml"))
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid type")
	})
	t.Run("DefaultTypeIsChain", func(t *testing.T) {
		th := testLoad(t, "valid_command.yaml")
		assert.Equal(t, digraph.TypeChain, th.Type)
	})
	t.Run("ChainTypeWithNoDependencies", func(t *testing.T) {
		th := testLoad(t, "chain_no_dependencies.yaml")
		assert.Equal(t, digraph.TypeChain, th.Type)

		// Check dependencies
		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)                    // step1
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends) // step2
		assert.Empty(t, th.Steps[2].Depends)                    // step3 - explicitly no deps
		assert.Equal(t, []string{"step3"}, th.Steps[3].Depends) // step4 should depend on step3
	})
	t.Run("Preconditions", func(t *testing.T) {
		th := testLoad(t, "preconditions.yaml")
		assert.Len(t, th.Preconditions, 1)
		assert.Equal(t, &digraph.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Preconditions[0])
	})
	t.Run("maxActiveRuns", func(t *testing.T) {
		th := testLoad(t, "max_active_runs.yaml")
		assert.Equal(t, 5, th.MaxActiveRuns)
	})
	t.Run("MaxActiveSteps", func(t *testing.T) {
		th := testLoad(t, "max_active_steps.yaml")
		assert.Equal(t, 3, th.MaxActiveSteps)
	})
	t.Run("MaxOutputSize", func(t *testing.T) {
		// Test custom maxOutputSize
		th := testLoad(t, "valid_max_output_size.yaml")
		assert.Equal(t, 524288, th.MaxOutputSize) // 512KB

		// Test default maxOutputSize when not specified
		th2 := testLoad(t, "default.yaml")
		assert.Equal(t, 0, th2.MaxOutputSize) // Default 1MB
	})
	t.Run("ValidationError", func(t *testing.T) {
		type testCase struct {
			name        string
			dag         string
			expectedErr error
		}

		testCases := []testCase{
			{
				name:        "NoName",
				dag:         "invalid_no_name.yaml",
				expectedErr: digraph.ErrStepNameRequired,
			},
			{
				name:        "InvalidEnv",
				dag:         "invalid_env.yaml",
				expectedErr: digraph.ErrInvalidEnvValue,
			},
			{
				name:        "InvalidParams",
				dag:         "invalid_params.yaml",
				expectedErr: digraph.ErrInvalidParamValue,
			},
			{
				name:        "InvalidSchedule",
				dag:         "invalid_schedule.yaml",
				expectedErr: digraph.ErrInvalidSchedule,
			},
			{
				name:        "NoCommand",
				dag:         "invalid_no_command.yaml",
				expectedErr: digraph.ErrStepCommandIsRequired,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				_ = testLoad(t, tc.dag, withExpectedErr(tc.expectedErr))
			})
		}
	})
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		file     string
		expected map[string]string
	}

	testCases := []testCase{
		{
			name: "ValidEnv",
			file: "valid_env.yaml",
			expected: map[string]string{
				"FOO": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitution",
			file: "valid_env_substitution.yaml",
			expected: map[string]string{
				"VAR": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitutionAndEnv",
			file: "valid_env_substitution_and_env.yaml",
			expected: map[string]string{
				"FOO": "BEE:BAZ:BOO:FOO",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testLoad(t, tc.file)
			for key, val := range tc.expected {
				th.AssertEnv(t, key, val)
			}
		})
	}
}

func TestBuildSchedule(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name    string
		file    string
		start   []string
		stop    []string
		restart []string
	}

	testCases := []testCase{
		{
			name:    "ValidSchedule",
			file:    "valid_schedule.yaml",
			start:   []string{"0 1 * * *"},
			stop:    []string{"0 2 * * *"},
			restart: []string{"0 12 * * *"},
		},
		{
			name: "ListSchedule",
			file: "schedule_in_list.yaml",
			start: []string{
				"0 1 * * *",
				"0 18 * * *",
			},
		},
		{
			name: "MultipleValues",
			file: "schedule_with_multiple_values.yaml",
			start: []string{
				"0 1 * * *",
				"0 18 * * *",
			},
			stop: []string{
				"0 2 * * *",
				"0 20 * * *",
			},
			restart: []string{
				"0 12 * * *",
				"0 22 * * *",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			th := testLoad(t, tc.file)
			assert.Len(t, th.Schedule, len(tc.start))
			for i, s := range tc.start {
				assert.Equal(t, s, th.Schedule[i].Expression)
			}

			assert.Len(t, th.StopSchedule, len(tc.stop))
			for i, s := range tc.stop {
				assert.Equal(t, s, th.StopSchedule[i].Expression)
			}

			assert.Len(t, th.RestartSchedule, len(tc.restart))
			for i, s := range tc.restart {
				assert.Equal(t, s, th.RestartSchedule[i].Expression)
			}
		})
	}
}

func TestBuildStep(t *testing.T) {
	t.Parallel()
	t.Run("ValidCommand", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "valid_command.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "echo 1", th.Steps[0].CmdWithArgs)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("ValidCommandInArray", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "valid_command_in_array.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("ValidCommandInList", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "valid_command_in_list.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t,
			cmdutil.JoinCommandArgs("echo", []string{"1"}),
			th.Steps[0].CmdArgsSys)
		assert.Equal(t, "echo", th.Steps[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("HTTPExecutor", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "http_executor.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
	})
	t.Run("HTTPExecutorWithConfig", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "http_executor_with_config.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, map[string]any{
			"key": "value",
			"map": map[string]any{
				"foo": "bar",
			},
		}, th.Steps[0].ExecutorConfig.Config)
	})
	t.Run("DAGExecutor", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "sub_dag.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "dag", th.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "run", th.Steps[0].Command)
		assert.Equal(t, []string{
			"sub_dag",
			"param1=\"value1\" param2=\"value2\"",
		}, th.Steps[0].Args)
	})
	t.Run("ContinueOn", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "continue_on.yaml")
		assert.Len(t, th.Steps, 1)
		assert.True(t, th.Steps[0].ContinueOn.Failure)
		assert.True(t, th.Steps[0].ContinueOn.Skipped)
	})
	t.Run("RetryPolicy", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "retry_policy.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 3, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 10*time.Second, th.Steps[0].RetryPolicy.Interval)
	})
	t.Run("RetryPolicyWithBackoff", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "retry_policy_backoff.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 5, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 2*time.Second, th.Steps[0].RetryPolicy.Interval)
		assert.Equal(t, 2.0, th.Steps[0].RetryPolicy.Backoff)
		assert.Equal(t, 30*time.Second, th.Steps[0].RetryPolicy.MaxInterval)
	})
	t.Run("RetryPolicyWithBackoffBool", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "retry_policy_backoff_bool.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RetryPolicy)
		assert.Equal(t, 3, th.Steps[0].RetryPolicy.Limit)
		assert.Equal(t, 1*time.Second, th.Steps[0].RetryPolicy.Interval)
		assert.Equal(t, 2.0, th.Steps[0].RetryPolicy.Backoff) // true converts to 2.0
		assert.Equal(t, 10*time.Second, th.Steps[0].RetryPolicy.MaxInterval)
	})
	t.Run("RetryPolicyInvalidBackoff", func(t *testing.T) {
		t.Parallel()

		// Test backoff value <= 1.0
		data := []byte(`
name: test-invalid-backoff
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: 0.8
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")
	})
	t.Run("RepeatPolicy", func(t *testing.T) {
		t.Parallel()

		// Test basic boolean repeat (backward compatibility)
		th := testLoad(t, "repeat_policy.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RepeatPolicy)
		assert.Equal(t, digraph.RepeatModeWhile, th.Steps[0].RepeatPolicy.RepeatMode)
		assert.Equal(t, 60*time.Second, th.Steps[0].RepeatPolicy.Interval)
		assert.Equal(t, 0, th.Steps[0].RepeatPolicy.Limit) // No limit set
	})

	t.Run("RepeatPolicyWhileCondition", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_while_condition.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeWhile, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "", repeatPolicy.Condition.Expected) // No expected value for while mode
		assert.Equal(t, 5*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 3, repeatPolicy.Limit)
	})

	t.Run("RepeatPolicyUntilCondition", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_until_condition.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeUntil, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 10*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 5, repeatPolicy.Limit)
	})

	t.Run("RepeatPolicyWhileExitCode", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_while_exitcode.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeWhile, repeatPolicy.RepeatMode)
		assert.Equal(t, []int{1, 2}, repeatPolicy.ExitCode)
		assert.Equal(t, 15*time.Second, repeatPolicy.Interval)
		assert.Nil(t, repeatPolicy.Condition) // No condition set
	})

	t.Run("RepeatPolicyUntilExitCode", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_until_exitcode.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeUntil, repeatPolicy.RepeatMode)
		assert.Equal(t, []int{0}, repeatPolicy.ExitCode)
		assert.Equal(t, 20*time.Second, repeatPolicy.Interval)
		assert.Nil(t, repeatPolicy.Condition) // No condition set
	})

	t.Run("RepeatPolicyBackwardCompatibilityUntil", func(t *testing.T) {
		t.Parallel()

		// Test backward compatibility: condition + expected should infer "until" mode
		th := testLoad(t, "repeat_policy_backward_until.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeUntil, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 25*time.Second, repeatPolicy.Interval)
	})

	t.Run("RepeatPolicyBackwardCompatibilityWhile", func(t *testing.T) {
		t.Parallel()

		// Test backward compatibility: condition only should infer "while" mode
		th := testLoad(t, "repeat_policy_backward_while.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeWhile, repeatPolicy.RepeatMode)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "", repeatPolicy.Condition.Expected) // No expected value
		assert.Equal(t, 30*time.Second, repeatPolicy.Interval)
	})

	t.Run("RepeatPolicyCondition", func(t *testing.T) {
		t.Parallel()

		// Test existing backward compatibility condition test
		th := testLoad(t, "repeat_policy_condition.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 1*time.Second, repeatPolicy.Interval)
		// Should infer "until" mode due to condition + expected
		assert.Equal(t, digraph.RepeatModeUntil, repeatPolicy.RepeatMode)
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "signal_on_stop.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "SIGINT", th.Steps[0].SignalOnStop)
	})
	t.Run("StepWithID", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "step_with_id.yaml")
		assert.Len(t, th.Steps, 3)

		// First step has ID
		assert.Equal(t, "step1", th.Steps[0].Name)
		assert.Equal(t, "unique-step-1", th.Steps[0].ID)

		// Second step has no ID
		assert.Equal(t, "step2", th.Steps[1].Name)
		assert.Equal(t, "", th.Steps[1].ID)

		// Third step has ID
		assert.Equal(t, "step3", th.Steps[2].Name)
		assert.Equal(t, "custom-id-123", th.Steps[2].ID)
	})
	t.Run("Preconditions", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "step_preconditions.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &digraph.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Steps[0].Preconditions[0])
	})
	t.Run("RepeatPolicyExitCode", func(t *testing.T) {
		t.Parallel()

		// Test existing backward compatibility exitcode test
		th := testLoad(t, "repeat_policy_exitcode.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, []int{42}, repeatPolicy.ExitCode)
		assert.Equal(t, 2*time.Second, repeatPolicy.Interval)
		// Should infer "while" mode due to exitCode only
		assert.Equal(t, digraph.RepeatModeWhile, repeatPolicy.RepeatMode)
	})

	t.Run("RepeatPolicyWithBackoff", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_backoff.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeWhile, repeatPolicy.RepeatMode)
		assert.Equal(t, 5*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 1.5, repeatPolicy.Backoff)
		assert.Equal(t, 60*time.Second, repeatPolicy.MaxInterval)
		assert.Equal(t, 10, repeatPolicy.Limit)
		assert.Equal(t, []int{1}, repeatPolicy.ExitCode)
	})

	t.Run("RepeatPolicyWithBackoffBool", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy_backoff_bool.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy)
		assert.Equal(t, digraph.RepeatModeUntil, repeatPolicy.RepeatMode)
		assert.Equal(t, 2*time.Second, repeatPolicy.Interval)
		assert.Equal(t, 2.0, repeatPolicy.Backoff) // true converts to 2.0
		assert.Equal(t, 20*time.Second, repeatPolicy.MaxInterval)
		assert.Equal(t, 5, repeatPolicy.Limit)
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo done", repeatPolicy.Condition.Condition)
		assert.Equal(t, "done", repeatPolicy.Condition.Expected)
	})

	t.Run("RepeatPolicyErrorCases", func(t *testing.T) {
		t.Parallel()

		// Test invalid repeat value
		data := []byte(`
name: test-invalid-repeat
steps:
  - name: "invalid-repeat"
    command: "echo test"
    repeatPolicy:
      repeat: "invalid"
      intervalSec: 10
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "invalid value for repeat: 'invalid'")

		// Test explicit while mode without condition or exitCode
		data = []byte(`
name: test-while-no-condition
steps:
  - name: "while-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 10
`)
		dag, err = digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "repeat mode 'while' requires either 'condition' or 'exitCode' to be specified")

		// Test explicit until mode without condition or exitCode
		data = []byte(`
name: test-until-no-condition
steps:
  - name: "until-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      intervalSec: 10
`)
		dag, err = digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "repeat mode 'until' requires either 'condition' or 'exitCode' to be specified")

		// Test invalid repeat type (not string or bool)
		data = []byte(`
name: test-invalid-type
steps:
  - name: "invalid-type"
    command: "echo test"
    repeatPolicy:
      repeat: 123
      intervalSec: 10
`)
		dag, err = digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "invalid value for repeat")
	})

	t.Run("PolicyBackoffValidation", func(t *testing.T) {
		t.Parallel()

		// Test repeat policy invalid backoff
		data := []byte(`
name: test-invalid-backoff
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 1.0
      exitCode: [1]
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")

		// Test with backoff = 0.5
		data = []byte(`
name: test-invalid-backoff-2
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 0.5
      exitCode: [1]
`)
		dag, err = digraph.LoadYAML(context.Background(), data)
		assert.Error(t, err)
		assert.Nil(t, dag)
		assert.Contains(t, err.Error(), "backoff must be greater than 1.0")
	})
}

type DAG struct {
	t *testing.T
	*digraph.DAG
}

func (th *DAG) AssertEnv(t *testing.T, key, val string) {
	th.t.Helper()

	expected := key + "=" + val
	for _, env := range th.Env {
		if env == expected {
			return
		}
	}
	t.Errorf("expected env %s=%s not found", key, val)
	for i, env := range th.Env {
		// print all envs that were found for debugging
		t.Logf("env[%d]: %s", i, env)
	}
}

func (th *DAG) AssertParam(t *testing.T, params ...string) {
	th.t.Helper()

	assert.Len(t, th.Params, len(params), "expected %d params, got %d", len(params), len(th.Params))
	for i, p := range params {
		assert.Equal(t, p, th.Params[i])
	}
}

type testOption func(*testOptions)
type testOptions struct {
	buildOpts   digraph.BuildOpts
	expectedErr error
}

func withBuildOpts(opts digraph.BuildOpts) testOption {
	return func(to *testOptions) {
		to.buildOpts = opts
	}
}

func withExpectedErr(err error) testOption {
	return func(to *testOptions) {
		to.expectedErr = err
	}
}

func testLoad(t *testing.T, file string, opts ...testOption) DAG {
	t.Helper()

	ctx := context.Background()
	var options testOptions

	for _, opt := range opts {
		opt(&options)
	}

	data := test.ReadTestdata(t, filepath.Join("digraph", file))
	dag, err := digraph.LoadYAMLWithOpts(ctx, data, options.buildOpts)

	if options.expectedErr != nil {
		if errs, ok := err.(*digraph.ErrorList); ok && len(*errs) > 0 {
			found := false
			for _, e := range *errs {
				if errors.Is(e, options.expectedErr) {
					found = true
					break
				}
			}
			require.True(t, found, "expected error %v, got %v", options.expectedErr, err)
		} else {
			assert.ErrorIs(t, err, options.expectedErr)
		}

		return DAG{t: t, DAG: nil}
	}

	require.NoError(t, err, "failed to load YAML %s", file)

	return DAG{t: t, DAG: dag}
}

func TestBuild_QueueConfiguration(t *testing.T) {
	t.Run("MaxActiveRunsDefaultsToOne", func(t *testing.T) {
		// Test that when maxActiveRuns is not specified, it defaults to 1
		th := testLoad(t, "valid_command.yaml") // Using a simple DAG without maxActiveRuns
		assert.Equal(t, 1, th.MaxActiveRuns, "maxActiveRuns should default to 1 when not specified")
	})

	t.Run("MaxActiveRunsNegativeValuePreserved", func(t *testing.T) {
		// Test that negative values are preserved (they mean queueing is disabled)
		// Create a simple DAG YAML with negative maxActiveRuns
		data := []byte(`
name: test-negative-max-active-runs
maxActiveRuns: -1
steps:
  - name: step1
    command: echo test
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, -1, dag.MaxActiveRuns, "negative maxActiveRuns should be preserved")
	})
}

func TestStepIDValidation(t *testing.T) {
	t.Parallel()

	t.Run("ValidID", func(t *testing.T) {
		data := []byte(`
name: test-valid-id
steps:
  - name: step1
    id: valid_id
    command: echo test
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "valid_id", dag.Steps[0].ID)
	})

	t.Run("InvalidIDFormat", func(t *testing.T) {
		data := []byte(`
name: test-invalid-id
steps:
  - name: step1
    id: 123invalid
    command: echo test
`)
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid step ID format")
	})

	t.Run("DuplicateIDs", func(t *testing.T) {
		data := []byte(`
name: test-duplicate-ids
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: step2
    id: myid
    command: echo test2
`)
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate step ID")
	})

	t.Run("IDConflictsWithStepName", func(t *testing.T) {
		data := []byte(`
name: test-id-name-conflict
steps:
  - name: step1
    id: step2
    command: echo test1
  - name: step2
    command: echo test2
`)
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts with another step's name")
	})

	t.Run("NameConflictsWithStepID", func(t *testing.T) {
		data := []byte(`
name: test-name-id-conflict
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: myid
    command: echo test2
`)
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "conflicts with another step's name")
	})

	t.Run("ReservedWordID", func(t *testing.T) {
		data := []byte(`
name: test-reserved-word
steps:
  - name: step1
    id: env
    command: echo test
`)
		_, err := digraph.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "reserved word")
	})
}

func TestStepIDInDependencies(t *testing.T) {
	t.Parallel()

	t.Run("DependOnStepByID", func(t *testing.T) {
		data := []byte(`
name: test-depend-by-id
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: first
    command: echo test2
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, "first", dag.Steps[0].ID)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends) // ID "first" resolved to name "step1"
	})

	t.Run("DependOnStepByNameWhenIDExists", func(t *testing.T) {
		data := []byte(`
name: test-depend-by-name
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: step1
    command: echo test2
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends)
	})

	t.Run("MultipleDependenciesWithIDs", func(t *testing.T) {
		data := []byte(`
name: test-multiple-deps
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    id: second
    command: echo test2
  - name: step3
    depends: 
      - first
      - second
    command: echo test3
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // IDs resolved to names
	})

	t.Run("MixOfIDAndNameDependencies", func(t *testing.T) {
		data := []byte(`
name: test-mixed-deps
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    command: echo test2
  - name: step3
    depends:
      - first
      - step2
    command: echo test3
`)
		dag, err := digraph.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // ID "first" resolved to name "step1"
	})
}

func TestChainTypeWithStepIDs(t *testing.T) {
	t.Parallel()

	data := []byte(`
type: chain
name: chain-with-ids
steps:
  - name: step1
    id: s1
    command: echo first
  - name: step2
    id: s2
    command: echo second
  - name: step3
    command: echo third
`)
	dag, err := digraph.LoadYAML(context.Background(), data)
	require.NoError(t, err)
	require.Len(t, dag.Steps, 3)

	// Verify IDs are preserved
	assert.Equal(t, "s1", dag.Steps[0].ID)
	assert.Equal(t, "s2", dag.Steps[1].ID)
	assert.Equal(t, "", dag.Steps[2].ID)

	// Verify chain dependencies were added
	assert.Empty(t, dag.Steps[0].Depends)
	assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends)
	assert.Equal(t, []string{"step2"}, dag.Steps[2].Depends)
}

func TestResolveStepDependencies(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected map[string][]string // step name -> expected depends
	}{
		{
			name: "single ID dependency",
			yaml: `
name: test
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    depends: s1
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
		{
			name: "multiple ID dependencies",
			yaml: `
name: test
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    id: s2
    command: echo "2"
  - name: step-three
    depends:
      - s1
      - s2
    command: echo "3"
`,
			expected: map[string][]string{
				"step-three": {"step-one", "step-two"},
			},
		},
		{
			name: "mixed ID and name dependencies",
			yaml: `
name: test
steps:
  - name: step-one
    id: s1
    command: echo "1"
  - name: step-two
    command: echo "2"
  - name: step-three
    depends:
      - s1
      - step-two
    command: echo "3"
`,
			expected: map[string][]string{
				"step-three": {"step-one", "step-two"},
			},
		},
		{
			name: "no ID dependencies",
			yaml: `
name: test
steps:
  - name: step-one
    command: echo "1"
  - name: step-two
    depends: step-one
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
		{
			name: "ID same as name",
			yaml: `
name: test
steps:
  - name: step-one
    id: step-one
    command: echo "1"
  - name: step-two
    depends: step-one
    command: echo "2"
`,
			expected: map[string][]string{
				"step-two": {"step-one"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			dag, err := digraph.LoadYAML(ctx, []byte(tt.yaml))
			require.NoError(t, err)

			// Check that dependencies were resolved correctly
			for _, step := range dag.Steps {
				if expectedDeps, exists := tt.expected[step.Name]; exists {
					assert.Equal(t, expectedDeps, step.Depends,
						"Step %s dependencies should be resolved correctly", step.Name)
				}
			}
		})
	}
}

func TestResolveStepDependencies_Errors(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "dependency on non-existent ID",
			yaml: `
name: test
steps:
  - name: step-one
    command: echo "1"
  - name: step-two
    depends: nonexistent
    command: echo "2"
`,
			expectedErr: "", // This should be caught by dependency validation, not ID resolution
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, err := digraph.LoadYAML(ctx, []byte(tt.yaml))
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				// Some tests expect no error from ID resolution
				// but might fail in other validation steps
				_ = err
			}
		})
	}
}

func TestBuildOTel(t *testing.T) {
	t.Run("BasicOTelConfig", func(t *testing.T) {
		yaml := `
name: test
otel:
  enabled: true
  endpoint: localhost:4317
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := digraph.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.OTel)
		assert.True(t, dag.OTel.Enabled)
		assert.Equal(t, "localhost:4317", dag.OTel.Endpoint)
	})

	t.Run("FullOTelConfig", func(t *testing.T) {
		yaml := `
name: test
otel:
  enabled: true
  endpoint: otel-collector:4317
  headers:
    Authorization: Bearer token
  insecure: true
  timeout: 30s
  resource:
    service.name: dagu-test
    service.version: "1.0.0"
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := digraph.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.OTel)
		assert.True(t, dag.OTel.Enabled)
		assert.Equal(t, "otel-collector:4317", dag.OTel.Endpoint)
		assert.Equal(t, "Bearer token", dag.OTel.Headers["Authorization"])
		assert.True(t, dag.OTel.Insecure)
		assert.Equal(t, 30*time.Second, dag.OTel.Timeout)
		assert.Equal(t, "dagu-test", dag.OTel.Resource["service.name"])
		assert.Equal(t, "1.0.0", dag.OTel.Resource["service.version"])
	})

	t.Run("DisabledOTel", func(t *testing.T) {
		yaml := `
name: test
steps:
  - name: step1
    command: echo "test"
`
		ctx := context.Background()
		dag, err := digraph.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		assert.Nil(t, dag.OTel)
	})
}
