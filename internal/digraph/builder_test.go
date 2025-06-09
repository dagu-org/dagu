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
		assert.Empty(t, th.Steps[0].Depends) // setup
		assert.Equal(t, []string{"setup"}, th.Steps[1].Depends) // download-a
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
		assert.Empty(t, th.Steps[0].Depends) // step1
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends) // step2
		assert.Empty(t, th.Steps[2].Depends) // step3 - explicitly no deps
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
	t.Run("RepeatPolicy", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "repeat_policy.yaml")
		assert.Len(t, th.Steps, 1)
		require.NotNil(t, th.Steps[0].RepeatPolicy)
		assert.True(t, th.Steps[0].RepeatPolicy.Repeat)
		assert.Equal(t, 60*time.Second, th.Steps[0].RepeatPolicy.Interval)
	})
	t.Run("RepeatPolicyCondition", func(t *testing.T) {
		th := testLoad(t, "repeat_policy_condition.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		require.NotNil(t, repeatPolicy.Condition)
		assert.Equal(t, "echo hello", repeatPolicy.Condition.Condition)
		assert.Equal(t, "hello", repeatPolicy.Condition.Expected)
		assert.Equal(t, 1*time.Second, repeatPolicy.Interval)
	})
	t.Run("SignalOnStop", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "signal_on_stop.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "SIGINT", th.Steps[0].SignalOnStop)
	})
	t.Run("Preconditions", func(t *testing.T) {
		t.Parallel()

		th := testLoad(t, "step_preconditions.yaml")
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &digraph.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Steps[0].Preconditions[0])
	})
	t.Run("RepeatPolicyExitCode", func(t *testing.T) {
		th := testLoad(t, "repeat_policy_exitcode.yaml")
		assert.Len(t, th.Steps, 1)
		repeatPolicy := th.Steps[0].RepeatPolicy
		assert.Equal(t, []int{42}, repeatPolicy.ExitCode)
		assert.Equal(t, 2*time.Second, repeatPolicy.Interval)
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
