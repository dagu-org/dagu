package spec_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type DAG struct {
	t *testing.T
	*core.DAG
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

func TestEnvParams(t *testing.T) {
	t.Parallel()

	// Params tests - these test complex parameter substitution and env expansion
	// which requires the full pipeline (YAML parsing + build)
	paramTests := []struct {
		name       string
		yaml       string
		opts       *spec.BuildOpts
		wantParams []string
	}{
		{
			name: "ParamsWithSubstitution",
			yaml: `
params: "TEST_PARAM $1"
`,
			wantParams: []string{"1=TEST_PARAM", "2=TEST_PARAM"},
		},
		{
			name: "ParamsWithQuotedValues",
			yaml: `
params: x="a b c" y="d e f"
`,
			wantParams: []string{"x=a b c", "y=d e f"},
		},
		{
			name: "ParamsAsMap",
			yaml: `
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`,
			wantParams: []string{"FOO=foo", "BAR=bar", "BAZ=baz"},
		},
		{
			name: "ParamsAsMapOverride",
			yaml: `
params:
  - FOO: foo
  - BAR: bar
  - BAZ: "` + "`echo baz`" + `"
`,
			opts:       &spec.BuildOpts{Parameters: "FOO=X BAZ=Y"},
			wantParams: []string{"FOO=X", "BAR=bar", "BAZ=Y"},
		},
		{
			name: "ParamsWithComplexValues",
			yaml: `
params: first P1=foo P2=${A001} P3=` + "`/bin/echo BAR`" + ` X=bar Y=${P1} Z="A B C"
env:
  - A001: TEXT
`,
			wantParams: []string{"1=first", "P1=foo", "P2=TEXT", "P3=BAR", "X=bar", "Y=foo", "Z=A B C"},
		},
		{
			name: "ParamsWithSubstringAndDefaults",
			yaml: `
env:
  - SOURCE_ID: HBL01_22OCT2025_0536
params:
  - BASE: ${SOURCE_ID}
  - PREFIX: ${BASE:0:5}
  - REMAINDER: ${BASE:5}
  - FALLBACK: ${MISSING_VALUE:-fallback}
`,
			wantParams: []string{"BASE=HBL01_22OCT2025_0536", "PREFIX=HBL01", "REMAINDER=_22OCT2025_0536", "FALLBACK=fallback"},
		},
		{
			name: "ParamsNoEvalPreservesRaw",
			yaml: `
env:
  - SOURCE_ID: HBL01_22OCT2025_0536
params:
  - BASE: ${SOURCE_ID}
  - PREFIX: ${BASE:0:5}
`,
			opts:       &spec.BuildOpts{Flags: spec.BuildFlagNoEval},
			wantParams: []string{"BASE=${SOURCE_ID}", "PREFIX=${BASE:0:5}"},
		},
	}

	for _, tt := range paramTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var dag *core.DAG
			var err error
			if tt.opts != nil {
				dag, err = spec.LoadYAMLWithOpts(context.Background(), []byte(tt.yaml), *tt.opts)
			} else {
				dag, err = spec.LoadYAML(context.Background(), []byte(tt.yaml))
			}
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
			th.AssertParam(t, tt.wantParams...)
		})
	}
}

func TestBuildChainType(t *testing.T) {
	t.Parallel()

	t.Run("Basic", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
type: chain
steps:
  - echo "First"
  - echo "Second"
  - echo "Third"
  - echo "Fourth"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
		assert.Equal(t, []string{"cmd_3"}, th.Steps[3].Depends)
	})

	t.Run("WithExplicitDepends", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
type: chain
steps:
  - name: setup
    command: ./setup.sh
  - name: download-a
    command: wget fileA
  - name: download-b
    command: wget fileB
  - name: process-both
    command: process.py fileA fileB
    depends:
      - download-a
      - download-b
  - name: cleanup
    command: rm -f fileA fileB
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		assert.Len(t, th.Steps, 5)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"setup"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"download-a"}, th.Steps[2].Depends)
		assert.ElementsMatch(t, []string{"download-a", "download-b"}, th.Steps[3].Depends)
		assert.Equal(t, []string{"process-both"}, th.Steps[4].Depends)
	})

	t.Run("WithEmptyDependencies", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
type: chain
steps:
  - name: step1
    command: echo "First"
  - name: step2
    command: echo "Second"
  - name: step3
    command: echo "Third"
    depends: []
  - name: step4
    command: echo "Fourth"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Equal(t, core.TypeChain, th.Type)

		assert.Len(t, th.Steps, 4)
		assert.Empty(t, th.Steps[0].Depends)
		assert.Equal(t, []string{"step1"}, th.Steps[1].Depends)
		assert.Empty(t, th.Steps[2].Depends)
		assert.Equal(t, []string{"step3"}, th.Steps[3].Depends)
	})
}

func TestBuildValidationError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		expectedErr error
	}{
		{
			name: "InvalidEnv",
			yaml: `
env:
  - VAR: "` + "`invalid command`" + `"`,
			expectedErr: spec.ErrInvalidEnvValue,
		},
		{
			name: "InvalidParams",
			yaml: `
params: "` + "`invalid command`" + `"`,
			expectedErr: spec.ErrInvalidParamValue,
		},
		{
			name: "InvalidSchedule",
			yaml: `
schedule: "1"`,
			expectedErr: spec.ErrInvalidSchedule,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			if errs, ok := err.(*core.ErrorList); ok && len(*errs) > 0 {
				found := false
				for _, e := range *errs {
					if errors.Is(e, tt.expectedErr) {
						found = true
						break
					}
				}
				require.True(t, found, "expected error %v, got %v", tt.expectedErr, err)
			} else {
				assert.ErrorIs(t, err, tt.expectedErr)
			}
		})
	}
}

func TestBuildEnv(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		yaml     string
		expected map[string]string
	}

	testCases := []testCase{
		{
			name: "ValidEnv",
			yaml: `
env:
  - FOO: "123"

steps:
  - "true"
`,
			expected: map[string]string{
				"FOO": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitution",
			yaml: `
env:
  - VAR: "` + "`echo 123`" + `"

steps:
  - "true"
`,
			expected: map[string]string{
				"VAR": "123",
			},
		},
		{
			name: "ValidEnvWithSubstitutionAndEnv",
			yaml: `
env:
  - BEE: "BEE"
  - BAZ: "BAZ"
  - BOO: "BOO"
  - FOO: "${BEE}:${BAZ}:${BOO}:FOO"

steps:
  - "true"
`,
			expected: map[string]string{
				"FOO": "BEE:BAZ:BOO:FOO",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dag, err := spec.LoadYAML(context.Background(), []byte(tc.yaml))
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
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
		yaml    string
		start   []string
		stop    []string
		restart []string
	}

	testCases := []testCase{
		{
			name: "ValidSchedule",
			yaml: `
schedule:
  start: "0 1 * * *"
  stop: "0 2 * * *"
  restart: "0 12 * * *"

steps:
  - "true"
`,
			start:   []string{"0 1 * * *"},
			stop:    []string{"0 2 * * *"},
			restart: []string{"0 12 * * *"},
		},
		{
			name: "ListSchedule",
			yaml: `
schedule:
  - "0 1 * * *"
  - "0 18 * * *"

steps:
  - "true"
`,
			start: []string{
				"0 1 * * *",
				"0 18 * * *",
			},
		},
		{
			name: "MultipleValues",
			yaml: `
schedule:
  start:
    - "0 1 * * *"
    - "0 18 * * *"
  stop:
    - "0 2 * * *"
    - "0 20 * * *"
  restart:
    - "0 12 * * *"
    - "0 22 * * *"

steps:
  - "true"
`,
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

			dag, err := spec.LoadYAML(context.Background(), []byte(tc.yaml))
			require.NoError(t, err)
			th := DAG{t: t, DAG: dag}
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

		data := []byte(`
steps:
  - command: echo 1
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		require.Len(t, th.Steps[0].Commands, 1)
		assert.Equal(t, "echo 1", th.Steps[0].Commands[0].CmdWithArgs)
		assert.Equal(t, "echo", th.Steps[0].Commands[0].Command)
		assert.Equal(t, []string{"1"}, th.Steps[0].Commands[0].Args)
		assert.Equal(t, "step1", th.Steps[0].Name)
	})
	t.Run("CommandAsScript", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: |
      echo hello
      echo world
    name: script
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		require.Len(t, th.Steps, 1)
		step := th.Steps[0]
		assert.Equal(t, "script", step.Name)
		assert.Equal(t, "echo hello\necho world", step.Script)
		assert.Empty(t, step.Command)
		assert.Empty(t, step.CmdWithArgs)
		assert.Empty(t, step.CmdArgsSys)
		assert.Nil(t, step.Args)
	})
	t.Run("ValidCommandInArray", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: [echo, 1]
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "step1", th.Steps[0].Name)
		assert.Len(t, th.Steps[0].Commands, 2)
		assert.Equal(t, "echo", th.Steps[0].Commands[0].Command)
		assert.Equal(t, "1", th.Steps[0].Commands[1].Command)
	})
	t.Run("ValidCommandInList", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command:
      - echo
      - 1
    name: step1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "step1", th.Steps[0].Name)
		assert.Len(t, th.Steps[0].Commands, 2)
		assert.Equal(t, "echo", th.Steps[0].Commands[0].Command)
		assert.Equal(t, "1", th.Steps[0].Commands[1].Command)
	})
	t.Run("MultipleCommandsInArray", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: build
    command:
      - npm install
      - npm run build
      - npm test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "build", th.Steps[0].Name)
		assert.Len(t, th.Steps[0].Commands, 3)
		assert.Equal(t, "npm", th.Steps[0].Commands[0].Command)
		assert.Equal(t, []string{"install"}, th.Steps[0].Commands[0].Args)
		assert.Equal(t, "npm", th.Steps[0].Commands[1].Command)
		assert.Equal(t, []string{"run", "build"}, th.Steps[0].Commands[1].Args)
		assert.Equal(t, "npm", th.Steps[0].Commands[2].Command)
		assert.Equal(t, []string{"test"}, th.Steps[0].Commands[2].Args)
	})
	t.Run("HTTPExecutor", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: GET http://example.com
    name: step1
    type: http
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "http", th.Steps[0].ExecutorConfig.Type)
	})
	t.Run("HTTPExecutorWithConfig", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: http://example.com
    name: step1
    type: http
    config:
      key: value
      map:
        foo: bar
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
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

		data := []byte(`
steps:
  - name: execute a sub-dag
    call: sub_dag
    params: "param1=value1 param2=value2"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "dag", th.Steps[0].ExecutorConfig.Type)
		require.NotNil(t, th.Steps[0].SubDAG)
		assert.Equal(t, "sub_dag", th.Steps[0].SubDAG.Name)
		assert.Equal(t, "param1=\"value1\" param2=\"value2\"", th.Steps[0].SubDAG.Params)
		assert.Empty(t, dag.BuildWarnings)

		// Legacy run field is still accepted
		dataLegacy := []byte(`
steps:
  - name: legacy sub-dag
    run: sub_dag_legacy
`)
		dagLegacy, err := spec.LoadYAML(context.Background(), dataLegacy)
		require.NoError(t, err)
		thLegacy := DAG{t: t, DAG: dagLegacy}
		assert.Len(t, thLegacy.Steps, 1)
		assert.Equal(t, "dag", thLegacy.Steps[0].ExecutorConfig.Type)
		require.NotNil(t, thLegacy.Steps[0].SubDAG)
		assert.Equal(t, "sub_dag_legacy", thLegacy.Steps[0].SubDAG.Name)
		require.Len(t, dagLegacy.BuildWarnings, 1)
		assert.Contains(t, dagLegacy.BuildWarnings[0], "Step field 'run' is deprecated")
	})
	// ContinueOn success cases
	continueOnTests := []struct {
		name            string
		yaml            string
		wantSkipped     bool
		wantFailure     bool
		wantExitCode    []int
		wantMarkSuccess bool
	}{
		{
			name: "ContinueOnObject",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      skipped: true
      failure: true
`,
			wantSkipped: true,
			wantFailure: true,
		},
		{
			name: "ContinueOnStringSkipped",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: skipped
`,
			wantSkipped: true,
			wantFailure: false,
		},
		{
			name: "ContinueOnStringFailed",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: failed
`,
			wantSkipped: false,
			wantFailure: true,
		},
		{
			name: "ContinueOnStringCaseInsensitive",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: SKIPPED
`,
			wantSkipped: true,
		},
		{
			name: "ContinueOnObjectWithExitCode",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      exitCode: [1, 2, 3]
      markSuccess: true
`,
			wantExitCode:    []int{1, 2, 3},
			wantMarkSuccess: true,
		},
	}

	for _, tt := range continueOnTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			assert.Equal(t, tt.wantSkipped, dag.Steps[0].ContinueOn.Skipped)
			assert.Equal(t, tt.wantFailure, dag.Steps[0].ContinueOn.Failure)
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, dag.Steps[0].ContinueOn.ExitCode)
			}
			if tt.wantMarkSuccess {
				assert.True(t, dag.Steps[0].ContinueOn.MarkSuccess)
			}
		})
	}

	// ContinueOn error cases
	continueOnErrorTests := []struct {
		name        string
		yaml        string
		errContains []string
	}{
		{
			name: "ContinueOnInvalidString",
			yaml: `
steps:
  - command: "echo 1"
    continueOn: invalid
`,
			errContains: []string{"continueOn"},
		},
		{
			name: "ContinueOnInvalidFailureType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      failure: "true"
`,
			errContains: []string{"continueOn.failure", "boolean"},
		},
		{
			name: "ContinueOnInvalidSkippedType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      skipped: 1
`,
			errContains: []string{"continueOn.skipped", "boolean"},
		},
		{
			name: "ContinueOnInvalidMarkSuccessType",
			yaml: `
steps:
  - command: "echo 1"
    continueOn:
      markSuccess: "yes"
`,
			errContains: []string{"continueOn.markSuccess", "boolean"},
		},
	}

	for _, tt := range continueOnErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			for _, s := range tt.errContains {
				assert.Contains(t, err.Error(), s)
			}
		})
	}
	// RetryPolicy success tests
	retryPolicyTests := []struct {
		name            string
		yaml            string
		wantLimit       int
		wantInterval    time.Duration
		wantBackoff     float64
		wantMaxInterval time.Duration
	}{
		{
			name: "RetryPolicyBasic",
			yaml: `
steps:
  - command: "echo 2"
    retryPolicy:
      limit: 3
      intervalSec: 10
`,
			wantLimit:    3,
			wantInterval: 10 * time.Second,
		},
		{
			name: "RetryPolicyWithBackoff",
			yaml: `
steps:
  - name: "test_backoff"
    command: "echo test"
    retryPolicy:
      limit: 5
      intervalSec: 2
      backoff: 2.0
      maxIntervalSec: 30
`,
			wantLimit:       5,
			wantInterval:    2 * time.Second,
			wantBackoff:     2.0,
			wantMaxInterval: 30 * time.Second,
		},
		{
			name: "RetryPolicyWithBackoffBool",
			yaml: `
steps:
  - name: "test_backoff_bool"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: true
      maxIntervalSec: 10
`,
			wantLimit:       3,
			wantInterval:    1 * time.Second,
			wantBackoff:     2.0, // true converts to 2.0
			wantMaxInterval: 10 * time.Second,
		},
	}

	for _, tt := range retryPolicyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			require.NotNil(t, dag.Steps[0].RetryPolicy)
			assert.Equal(t, tt.wantLimit, dag.Steps[0].RetryPolicy.Limit)
			assert.Equal(t, tt.wantInterval, dag.Steps[0].RetryPolicy.Interval)
			if tt.wantBackoff > 0 {
				assert.Equal(t, tt.wantBackoff, dag.Steps[0].RetryPolicy.Backoff)
			}
			if tt.wantMaxInterval > 0 {
				assert.Equal(t, tt.wantMaxInterval, dag.Steps[0].RetryPolicy.MaxInterval)
			}
		})
	}

	// RetryPolicy error tests
	retryPolicyErrorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "RetryPolicyInvalidBackoff",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
      intervalSec: 1
      backoff: 0.8
`,
			errContains: "backoff must be greater than 1.0",
		},
		{
			name: "RetryPolicyMissingLimit",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      intervalSec: 5
`,
			errContains: "limit is required when retryPolicy is specified",
		},
		{
			name: "RetryPolicyMissingIntervalSec",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    retryPolicy:
      limit: 3
`,
			errContains: "intervalSec is required when retryPolicy is specified",
		},
	}

	for _, tt := range retryPolicyErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Nil(t, dag)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
	// RepeatPolicy success tests
	repeatPolicyTests := []struct {
		name            string
		yaml            string
		wantMode        core.RepeatMode
		wantInterval    time.Duration
		wantLimit       int
		wantExitCode    []int
		wantCondition   string
		wantExpected    string
		wantBackoff     float64
		wantMaxInterval time.Duration
		wantNoCondition bool
	}{
		{
			name: "RepeatPolicyBasic",
			yaml: `
steps:
  - command: "echo 2"
    repeatPolicy:
      repeat: true
      intervalSec: 60
`,
			wantMode:     core.RepeatModeWhile,
			wantInterval: 60 * time.Second,
			wantLimit:    0,
		},
		{
			name: "RepeatPolicyWhileCondition",
			yaml: `
steps:
  - name: "repeat-while-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      condition: "echo hello"
      intervalSec: 5
      limit: 3
`,
			wantMode:      core.RepeatModeWhile,
			wantInterval:  5 * time.Second,
			wantLimit:     3,
			wantCondition: "echo hello",
			wantExpected:  "",
		},
		{
			name: "RepeatPolicyUntilCondition",
			yaml: `
steps:
  - name: "repeat-until-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      condition: "echo hello"
      expected: "hello"
      intervalSec: 10
      limit: 5
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  10 * time.Second,
			wantLimit:     5,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyWhileExitCode",
			yaml: `
steps:
  - name: "repeat-while-exitcode"
    command: "exit 1"
    repeatPolicy:
      repeat: "while"
      exitCode: [1, 2]
      intervalSec: 15
`,
			wantMode:        core.RepeatModeWhile,
			wantInterval:    15 * time.Second,
			wantExitCode:    []int{1, 2},
			wantNoCondition: true,
		},
		{
			name: "RepeatPolicyUntilExitCode",
			yaml: `
steps:
  - name: "repeat-until-exitcode"
    command: "exit 0"
    repeatPolicy:
      repeat: "until"
      exitCode: [0]
      intervalSec: 20
`,
			wantMode:        core.RepeatModeUntil,
			wantInterval:    20 * time.Second,
			wantExitCode:    []int{0},
			wantNoCondition: true,
		},
		{
			name: "RepeatPolicyBackwardCompatibilityUntil",
			yaml: `
steps:
  - name: "repeat-backward-compatibility-until"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 25
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  25 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyBackwardCompatibilityWhile",
			yaml: `
steps:
  - name: "repeat-backward-compatibility-while"
    command: "echo test"
    repeatPolicy:
      condition: "echo hello"
      intervalSec: 30
`,
			wantMode:      core.RepeatModeWhile,
			wantInterval:  30 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "",
		},
		{
			name: "RepeatPolicyCondition",
			yaml: `
steps:
  - name: "repeat-condition"
    command: "echo hello"
    repeatPolicy:
      condition: "echo hello"
      expected: "hello"
      intervalSec: 1
`,
			wantMode:      core.RepeatModeUntil,
			wantInterval:  1 * time.Second,
			wantCondition: "echo hello",
			wantExpected:  "hello",
		},
		{
			name: "RepeatPolicyExitCode",
			yaml: `
steps:
  - name: "repeat-exitcode"
    command: "exit 42"
    repeatPolicy:
      exitCode: [42]
      intervalSec: 2
`,
			wantMode:     core.RepeatModeWhile,
			wantInterval: 2 * time.Second,
			wantExitCode: []int{42},
		},
		{
			name: "RepeatPolicyWithBackoff",
			yaml: `
steps:
  - name: "test_repeat_backoff"
    command: "echo test"
    repeatPolicy:
      repeat: while
      intervalSec: 5
      backoff: 1.5
      maxIntervalSec: 60
      limit: 10
      exitCode: [1]
`,
			wantMode:        core.RepeatModeWhile,
			wantInterval:    5 * time.Second,
			wantBackoff:     1.5,
			wantMaxInterval: 60 * time.Second,
			wantLimit:       10,
			wantExitCode:    []int{1},
		},
		{
			name: "RepeatPolicyWithBackoffBool",
			yaml: `
steps:
  - name: "test_repeat_backoff_bool"
    command: "echo test"
    repeatPolicy:
      repeat: until
      intervalSec: 2
      backoff: true
      maxIntervalSec: 20
      limit: 5
      condition: "echo done"
      expected: "done"
`,
			wantMode:        core.RepeatModeUntil,
			wantInterval:    2 * time.Second,
			wantBackoff:     2.0,
			wantMaxInterval: 20 * time.Second,
			wantLimit:       5,
			wantCondition:   "echo done",
			wantExpected:    "done",
		},
	}

	for _, tt := range repeatPolicyTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)
			rp := dag.Steps[0].RepeatPolicy
			require.NotNil(t, rp)
			assert.Equal(t, tt.wantMode, rp.RepeatMode)
			assert.Equal(t, tt.wantInterval, rp.Interval)
			assert.Equal(t, tt.wantLimit, rp.Limit)
			if tt.wantExitCode != nil {
				assert.Equal(t, tt.wantExitCode, rp.ExitCode)
			}
			if tt.wantNoCondition {
				assert.Nil(t, rp.Condition)
			} else if tt.wantCondition != "" {
				require.NotNil(t, rp.Condition)
				assert.Equal(t, tt.wantCondition, rp.Condition.Condition)
				assert.Equal(t, tt.wantExpected, rp.Condition.Expected)
			}
			if tt.wantBackoff > 0 {
				assert.Equal(t, tt.wantBackoff, rp.Backoff)
			}
			if tt.wantMaxInterval > 0 {
				assert.Equal(t, tt.wantMaxInterval, rp.MaxInterval)
			}
		})
	}
	t.Run("SignalOnStop", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: echo 1
    name: step1
    signalOnStop: SIGINT
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Equal(t, "SIGINT", th.Steps[0].SignalOnStop)
	})
	t.Run("StepWithID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: unique-step-1
    command: echo "Step with ID"
  - name: step2
    command: echo "Step without ID"
  - name: step3
    id: custom-id-123
    command: echo "Another step with ID"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
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

		data := []byte(`
steps:
  - name: "2"
    command: "echo 2"
    preconditions:
      - condition: "test -f file.txt"
        expected: "true"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &core.Condition{Condition: "test -f file.txt", Expected: "true"}, th.Steps[0].Preconditions[0])
	})
	t.Run("StepPreconditionsWithNegate", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: "step_with_negate"
    command: "echo hello"
    preconditions:
      - condition: "${STATUS}"
        expected: "success"
        negate: true
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}
		assert.Len(t, th.Steps, 1)
		assert.Len(t, th.Steps[0].Preconditions, 1)
		assert.Equal(t, &core.Condition{Condition: "${STATUS}", Expected: "success", Negate: true}, th.Steps[0].Preconditions[0])
	})
	// RepeatPolicy error tests
	repeatPolicyErrorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "RepeatPolicyInvalidRepeatValue",
			yaml: `
steps:
  - name: "invalid-repeat"
    command: "echo test"
    repeatPolicy:
      repeat: "invalid"
      intervalSec: 10
`,
			errContains: "invalid value for repeat: 'invalid'",
		},
		{
			name: "RepeatPolicyWhileNoCondition",
			yaml: `
steps:
  - name: "while-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 10
`,
			errContains: "repeat mode 'while' requires either 'condition' or 'exitCode' to be specified",
		},
		{
			name: "RepeatPolicyUntilNoCondition",
			yaml: `
steps:
  - name: "until-no-condition"
    command: "echo test"
    repeatPolicy:
      repeat: "until"
      intervalSec: 10
`,
			errContains: "repeat mode 'until' requires either 'condition' or 'exitCode' to be specified",
		},
		{
			name: "RepeatPolicyInvalidType",
			yaml: `
steps:
  - name: "invalid-type"
    command: "echo test"
    repeatPolicy:
      repeat: 123
      intervalSec: 10
`,
			errContains: "invalid value for repeat",
		},
		{
			name: "RepeatPolicyBackoffTooLow",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 1.0
      exitCode: [1]
`,
			errContains: "backoff must be greater than 1.0",
		},
		{
			name: "RepeatPolicyBackoffBelowOne",
			yaml: `
steps:
  - name: "test"
    command: "echo test"
    repeatPolicy:
      repeat: "while"
      intervalSec: 1
      backoff: 0.5
      exitCode: [1]
`,
			errContains: "backoff must be greater than 1.0",
		},
	}

	for _, tt := range repeatPolicyErrorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Nil(t, dag)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestNestedArrayParallelSyntax(t *testing.T) {
	t.Parallel()

	t.Run("SimpleParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "step 1"
  - 
    - echo "parallel 1"
    - echo "parallel 2"
  - echo "step 3"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		// First step (sequential)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		require.Len(t, dag.Steps[0].Commands, 1)
		assert.Equal(t, "echo \"step 1\"", dag.Steps[0].Commands[0].CmdWithArgs)
		assert.Empty(t, dag.Steps[0].Depends)

		// Parallel steps
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		require.Len(t, dag.Steps[1].Commands, 1)
		assert.Equal(t, "echo \"parallel 1\"", dag.Steps[1].Commands[0].CmdWithArgs)
		assert.Equal(t, []string{"cmd_1"}, dag.Steps[1].Depends)

		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
		require.Len(t, dag.Steps[2].Commands, 1)
		assert.Equal(t, "echo \"parallel 2\"", dag.Steps[2].Commands[0].CmdWithArgs)
		assert.Equal(t, []string{"cmd_1"}, dag.Steps[2].Depends)

		// Last step (sequential, depends on both parallel steps)
		assert.Equal(t, "cmd_4", dag.Steps[3].Name)
		require.Len(t, dag.Steps[3].Commands, 1)
		assert.Equal(t, "echo \"step 3\"", dag.Steps[3].Commands[0].CmdWithArgs)

		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
		assert.Contains(t, dag.Steps[3].Depends, "cmd_3")
	})

	t.Run("MixedParallelAndNormalSyntax", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: setup
    command: echo "setup"
  -
    - echo "parallel 1"
    - name: test
      command: npm test
  - name: cleanup
    command: echo "cleanup"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		// Setup step
		assert.Equal(t, "setup", dag.Steps[0].Name)
		assert.Empty(t, dag.Steps[0].Depends)

		// Parallel steps
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		assert.Equal(t, []string{"setup"}, dag.Steps[1].Depends)

		assert.Equal(t, "test", dag.Steps[2].Name)
		assert.Equal(t, []string{"setup"}, dag.Steps[2].Depends)

		// Cleanup step
		assert.Equal(t, "cleanup", dag.Steps[3].Name)
		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
		assert.Contains(t, dag.Steps[3].Depends, "test")
	})

	t.Run("ParallelStepsWithExplicitDependencies", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    command: echo "1"
  - name: step2
    command: echo "2"
  - 
    - name: parallel1
      command: echo "p1"
      depends: [step1]  # Explicit dependency overrides automatic
    - name: parallel2
      command: echo "p2"
      # This will get automatic dependency on step2
  - name: final
    command: echo "done"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 5)

		// Parallel1 has explicit dependency on step1
		parallel1 := dag.Steps[2]
		assert.Equal(t, "parallel1", parallel1.Name)
		assert.Equal(t, []string{"step1", "step2"}, parallel1.Depends)

		// Parallel2 gets automatic dependency on step2
		parallel2 := dag.Steps[3]
		assert.Equal(t, "parallel2", parallel2.Name)
		assert.Equal(t, []string{"step2"}, parallel2.Depends)

		// Final depends on both parallel steps
		final := dag.Steps[4]
		assert.Equal(t, "final", final.Name)
		assert.Contains(t, final.Depends, "parallel1")
		assert.Contains(t, final.Depends, "parallel2")
	})

	t.Run("OnlyParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - 
    - echo "parallel 1"
    - echo "parallel 2"
    - echo "parallel 3"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 3)

		// All steps should have no dependencies (first group)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		// Note: Due to the way dependencies are handled, these may have dependencies on each other
		// The important thing is they work in parallel since they don't have external dependencies

		assert.Equal(t, "cmd_2", dag.Steps[1].Name)
		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
	})
	t.Run("ConsequentParallelSteps", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - 
    - echo "parallel 1"
    - echo "parallel 2"
  - 
    - echo "parallel 3"
    - echo "parallel 4"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 4)

		assert.Equal(t, "cmd_1", dag.Steps[0].Name)
		assert.Equal(t, "cmd_2", dag.Steps[1].Name)

		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
		assert.Contains(t, dag.Steps[2].Depends, "cmd_1")
		assert.Contains(t, dag.Steps[2].Depends, "cmd_2")

		assert.Equal(t, "cmd_4", dag.Steps[3].Name)
		assert.Contains(t, dag.Steps[3].Depends, "cmd_1")
		assert.Contains(t, dag.Steps[3].Depends, "cmd_2")
	})
}

func TestShorthandCommandSyntax(t *testing.T) {
	t.Parallel()

	t.Run("SimpleShorthandCommands", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "hello"
  - ls -la
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 2)

		// First step
		require.Len(t, dag.Steps[0].Commands, 1)
		assert.Equal(t, "echo \"hello\"", dag.Steps[0].Commands[0].CmdWithArgs)
		assert.Equal(t, "echo", dag.Steps[0].Commands[0].Command)
		assert.Equal(t, []string{"hello"}, dag.Steps[0].Commands[0].Args)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name) // Auto-generated name

		// Second step
		require.Len(t, dag.Steps[1].Commands, 1)
		assert.Equal(t, "ls -la", dag.Steps[1].Commands[0].CmdWithArgs)
		assert.Equal(t, "ls", dag.Steps[1].Commands[0].Command)
		assert.Equal(t, []string{"-la"}, dag.Steps[1].Commands[0].Args)
		assert.Equal(t, "cmd_2", dag.Steps[1].Name) // Auto-generated name
	})

	t.Run("MixedShorthandAndStandardSyntax", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "starting"
  - name: build
    command: make build
    env:
      DEBUG: "true"
  - ls -la
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Len(t, dag.Steps, 3)

		// First step (shorthand)
		require.Len(t, dag.Steps[0].Commands, 1)
		assert.Equal(t, "echo \"starting\"", dag.Steps[0].Commands[0].CmdWithArgs)
		assert.Equal(t, "cmd_1", dag.Steps[0].Name)

		// Second step (standard)
		require.Len(t, dag.Steps[1].Commands, 1)
		assert.Equal(t, "make build", dag.Steps[1].Commands[0].CmdWithArgs)
		assert.Equal(t, "build", dag.Steps[1].Name)
		assert.Contains(t, dag.Steps[1].Env, "DEBUG=true")

		// Third step (shorthand)
		require.Len(t, dag.Steps[2].Commands, 1)
		assert.Equal(t, "ls -la", dag.Steps[2].Commands[0].CmdWithArgs)
		assert.Equal(t, "cmd_3", dag.Steps[2].Name)
	})
}

func TestOptionalStepNames(t *testing.T) {
	t.Parallel()

	t.Run("AutoGenerateNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "hello"
  - npm test
  - go build
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("MixedExplicitAndGenerated", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - setup.sh
  - name: build
    command: make all
  - command: test.sh
    depends: build
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "build", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("HandleNameConflicts", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - echo "first"
  - name: cmd_2
    command: echo "explicit"
  - echo "third"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
	})

	t.Run("DependenciesWithGeneratedNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - git pull
  - command: npm install
    depends: cmd_1
  - command: npm test
    depends: cmd_2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)

		// Check dependencies are correctly resolved
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
	})

	t.Run("OutputVariablesWithGeneratedNames", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - command: echo "v1.0.0"
    output: VERSION
  - command: echo "Building version ${VERSION}"
    depends: cmd_1
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 2)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "VERSION", th.Steps[0].Output)
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
	})

	t.Run("TypeBasedNaming", func(t *testing.T) {
		t.Parallel()

		// Test different step types get appropriate names
		data := []byte(`
steps:
  - echo "command"
  - script: |
      echo "script content"
  - type: http
    config:
      url: https://example.com
  - call: sub-dag
  - type: docker
    config:
      image: alpine
  - type: ssh
    config:
      host: example.com
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 6)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "script_2", th.Steps[1].Name)
		assert.Equal(t, "http_3", th.Steps[2].Name)
		assert.Equal(t, "dag_4", th.Steps[3].Name)
		assert.Equal(t, "docker_5", th.Steps[4].Name)
		assert.Equal(t, "ssh_6", th.Steps[5].Name)
	})

	t.Run("BackwardCompatibility", func(t *testing.T) {
		t.Parallel()

		// Ensure existing DAGs with explicit names still work
		data := []byte(`
steps:
  - echo "setup"
  - echo "test"
  - echo "deploy"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		th := DAG{t: t, DAG: dag}

		require.Len(t, th.Steps, 3)
		assert.Equal(t, "cmd_1", th.Steps[0].Name)
		assert.Equal(t, "cmd_2", th.Steps[1].Name)
		assert.Equal(t, "cmd_3", th.Steps[2].Name)
		// In chain mode, sequential dependencies are implicit
		assert.Equal(t, []string{"cmd_1"}, th.Steps[1].Depends)
		assert.Equal(t, []string{"cmd_2"}, th.Steps[2].Depends)
	})
}

func TestStepIDValidation(t *testing.T) {
	t.Parallel()

	// Success test
	t.Run("ValidID", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: step1
    id: valid_id
    command: echo test
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, "valid_id", dag.Steps[0].ID)
	})

	// Error tests
	errorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "InvalidIDFormat",
			yaml: `
steps:
  - name: step1
    id: 123invalid
    command: echo test
`,
			errContains: "invalid step ID format",
		},
		{
			name: "DuplicateIDs",
			yaml: `
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: step2
    id: myid
    command: echo test2
`,
			errContains: "duplicate step ID",
		},
		{
			name: "IDConflictsWithStepName",
			yaml: `
steps:
  - name: step1
    id: step2
    command: echo test1
  - name: step2
    command: echo test2
`,
			errContains: "conflicts with another step's name",
		},
		{
			name: "NameConflictsWithStepID",
			yaml: `
steps:
  - name: step1
    id: myid
    command: echo test1
  - name: myid
    command: echo test2
`,
			errContains: "conflicts with another step's name",
		},
		{
			name: "ReservedWordID",
			yaml: `
steps:
  - name: step1
    id: env
    command: echo test
`,
			errContains: "reserved word",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}
}

func TestStepIDInDependencies(t *testing.T) {
	t.Parallel()

	t.Run("DependOnStepByID", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: first
    command: echo test2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, "first", dag.Steps[0].ID)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends) // ID "first" resolved to name "step1"
	})

	t.Run("DependOnStepByNameWhenIDExists", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
steps:
  - name: step1
    id: first
    command: echo test1
  - name: step2
    depends: step1
    command: echo test2
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)
		assert.Equal(t, []string{"step1"}, dag.Steps[1].Depends)
	})

	t.Run("MultipleDependenciesWithIDs", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
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
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // IDs resolved to names
	})

	t.Run("MixOfIDAndNameDependencies", func(t *testing.T) {
		data := []byte(`
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
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"step1", "step2"}, dag.Steps[2].Depends) // ID "first" resolved to name "step1"
	})
}

func TestChainTypeWithStepIDs(t *testing.T) {
	t.Parallel()

	data := []byte(`
type: chain
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
	dag, err := spec.LoadYAML(context.Background(), data)
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
	t.Parallel()

	tests := []struct {
		name     string
		yaml     string
		expected map[string][]string // step name -> expected depends
	}{
		{
			name: "SingleIDDependency",
			yaml: `
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
			name: "MultipleIDDependencies",
			yaml: `
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
			name: "MixedIDAndNameDependencies",
			yaml: `
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
			name: "NoIDDependencies",
			yaml: `
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
			name: "IDSameAsName",
			yaml: `
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
			dag, err := spec.LoadYAML(ctx, []byte(tt.yaml))
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
	t.Parallel()

	tests := []struct {
		name        string
		yaml        string
		expectedErr string
	}{
		{
			name: "DependencyOnNonExistentID",
			yaml: `
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
			_, err := spec.LoadYAML(ctx, []byte(tt.yaml))
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

func TestContainer(t *testing.T) {
	t.Parallel()

	// Basic container tests
	basicTests := []struct {
		name           string
		yaml           string
		wantImage      string
		wantName       string
		wantPullPolicy core.PullPolicy
		wantNil        bool
		wantEnv        []string
	}{
		{
			name: "BasicContainer",
			yaml: `
container:
  image: python:3.11-slim
  pullPolicy: always
steps:
  - name: step1
    command: python script.py
`,
			wantImage:      "python:3.11-slim",
			wantPullPolicy: core.PullPolicyAlways,
		},
		{
			name: "ContainerWithName",
			yaml: `
container:
  name: my-dag-container
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "my-dag-container",
		},
		{
			name: "ContainerNameEmpty",
			yaml: `
container:
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "",
		},
		{
			name: "ContainerNameTrimmed",
			yaml: `
container:
  name: "  my-container  "
  image: alpine:latest
steps:
  - name: step1
    command: echo hello
`,
			wantImage: "alpine:latest",
			wantName:  "my-container",
		},
		{
			name: "ContainerEnvAsMap",
			yaml: `
container:
  image: alpine
  env:
    FOO: bar
    BAZ: qux
steps:
  - name: step1
    command: echo test
`,
			wantImage: "alpine",
			wantEnv:   []string{"FOO=bar", "BAZ=qux"},
		},
		{
			name: "ContainerWithoutPullPolicy",
			yaml: `
container:
  image: alpine
steps:
  - name: step1
    command: echo test
`,
			wantImage:      "alpine",
			wantPullPolicy: core.PullPolicyMissing,
		},
		{
			name: "NoContainer",
			yaml: `
steps:
  - name: step1
    command: echo test
`,
			wantNil: true,
		},
	}

	for _, tt := range basicTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, dag.Container)
				return
			}
			require.NotNil(t, dag.Container)
			if tt.wantImage != "" {
				assert.Equal(t, tt.wantImage, dag.Container.Image)
			}
			if tt.wantName != "" || tt.name == "ContainerNameEmpty" {
				assert.Equal(t, tt.wantName, dag.Container.Name)
			}
			if tt.wantPullPolicy != 0 {
				assert.Equal(t, tt.wantPullPolicy, dag.Container.PullPolicy)
			}
			for _, env := range tt.wantEnv {
				assert.Contains(t, dag.Container.Env, env)
			}
		})
	}

	// Pull policy variations
	pullPolicyTests := []struct {
		name       string
		pullPolicy string
		expected   core.PullPolicy
	}{
		{"Always", "always", core.PullPolicyAlways},
		{"Never", "never", core.PullPolicyNever},
		{"Missing", "missing", core.PullPolicyMissing},
		{"TrueString", "true", core.PullPolicyAlways},
		{"FalseString", "false", core.PullPolicyNever},
	}

	for _, tt := range pullPolicyTests {
		t.Run("PullPolicy"+tt.name, func(t *testing.T) {
			t.Parallel()
			yaml := `
container:
  image: alpine
  pullPolicy: ` + tt.pullPolicy + `
steps:
  - name: step1
    command: echo test
`
			dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
			require.NoError(t, err)
			require.NotNil(t, dag.Container)
			assert.Equal(t, tt.expected, dag.Container.PullPolicy)
		})
	}

	// Exec mode tests (container as string or object with exec field)
	t.Run("ContainerStringForm", func(t *testing.T) {
		t.Parallel()
		yaml := `
container: my-running-container
steps:
  - name: step1
    command: echo test
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, "my-running-container", dag.Container.Exec)
		assert.Empty(t, dag.Container.Image)
		assert.True(t, dag.Container.IsExecMode())
	})

	t.Run("ContainerStringFormTrimmed", func(t *testing.T) {
		t.Parallel()
		yaml := `
container: "  my-container  "
steps:
  - name: step1
    command: echo test
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, "my-container", dag.Container.Exec)
	})

	t.Run("ContainerObjectExecForm", func(t *testing.T) {
		t.Parallel()
		yaml := `
container:
  exec: my-container
  user: root
  workingDir: /app
  env:
    - MY_VAR: value
steps:
  - name: step1
    command: echo test
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, "my-container", dag.Container.Exec)
		assert.Empty(t, dag.Container.Image)
		assert.Equal(t, "root", dag.Container.User)
		assert.Equal(t, "/app", dag.Container.WorkingDir)
		assert.Contains(t, dag.Container.Env, "MY_VAR=value")
		assert.True(t, dag.Container.IsExecMode())
	})

	t.Run("StepContainerStringForm", func(t *testing.T) {
		t.Parallel()
		yaml := `
steps:
  - name: step1
    container: my-step-container
    command: echo test
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Steps[0].Container)
		assert.Equal(t, "my-step-container", dag.Steps[0].Container.Exec)
		assert.True(t, dag.Steps[0].Container.IsExecMode())
	})

	t.Run("StepContainerObjectExecForm", func(t *testing.T) {
		t.Parallel()
		yaml := `
steps:
  - name: step1
    container:
      exec: my-step-container
      user: nobody
      workingDir: /tmp
    command: echo test
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Steps[0].Container)
		assert.Equal(t, "my-step-container", dag.Steps[0].Container.Exec)
		assert.Equal(t, "nobody", dag.Steps[0].Container.User)
		assert.Equal(t, "/tmp", dag.Steps[0].Container.WorkingDir)
		assert.True(t, dag.Steps[0].Container.IsExecMode())
	})

	// Error tests
	errorTests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "InvalidPullPolicy",
			yaml: `
container:
  image: alpine
  pullPolicy: invalid_policy
steps:
  - name: step1
    command: echo test
`,
			errContains: "failed to parse pull policy",
		},
		{
			name: "ContainerWithoutImage",
			yaml: `
container:
  pullPolicy: always
  env:
    - FOO: bar
steps:
  - name: step1
    command: echo test
`,
			errContains: "either 'exec' or 'image' must be specified",
		},
		{
			name: "ContainerExecAndImageMutualExclusive",
			yaml: `
container:
  exec: my-container
  image: alpine:latest
steps:
  - name: step1
    command: echo test
`,
			errContains: "'exec' and 'image' are mutually exclusive",
		},
		{
			name: "ContainerExecWithInvalidVolumes",
			yaml: `
container:
  exec: my-container
  volumes:
    - /data:/data
steps:
  - name: step1
    command: echo test
`,
			errContains: "cannot be used with 'exec'",
		},
		{
			name: "ContainerExecWithInvalidPorts",
			yaml: `
container:
  exec: my-container
  ports:
    - "8080:80"
steps:
  - name: step1
    command: echo test
`,
			errContains: "cannot be used with 'exec'",
		},
		{
			name: "ContainerExecWithInvalidNetwork",
			yaml: `
container:
  exec: my-container
  network: bridge
steps:
  - name: step1
    command: echo test
`,
			errContains: "cannot be used with 'exec'",
		},
		{
			name: "ContainerExecWithInvalidPullPolicy",
			yaml: `
container:
  exec: my-container
  pullPolicy: always
steps:
  - name: step1
    command: echo test
`,
			errContains: "cannot be used with 'exec'",
		},
		{
			name: "ContainerStringFormEmpty",
			yaml: `
container: "   "
steps:
  - name: step1
    command: echo test
`,
			errContains: "container name cannot be empty",
		},
		{
			name: "StepContainerExecAndImageMutualExclusive",
			yaml: `
steps:
  - name: step1
    container:
      exec: my-container
      image: alpine:latest
    command: echo test
`,
			errContains: "'exec' and 'image' are mutually exclusive",
		},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.errContains)
		})
	}

	// Complex container with all fields (separate due to many assertions)
	t.Run("ContainerWithAllFields", func(t *testing.T) {
		t.Parallel()
		yaml := `
container:
  image: node:18-alpine
  pullPolicy: missing
  env:
    - NODE_ENV: production
    - API_KEY: secret123
  volumes:
    - /data:/data:ro
    - /output:/output:rw
  user: "1000:1000"
  workingDir: /app
  platform: linux/amd64
  ports:
    - "8080:8080"
    - "9090:9090"
  network: bridge
  keepContainer: true
steps:
  - name: step1
    command: node app.js
`
		dag, err := spec.LoadYAML(context.Background(), []byte(yaml))
		require.NoError(t, err)
		require.NotNil(t, dag.Container)
		assert.Equal(t, "node:18-alpine", dag.Container.Image)
		assert.Equal(t, core.PullPolicyMissing, dag.Container.PullPolicy)
		assert.Contains(t, dag.Container.Env, "NODE_ENV=production")
		assert.Contains(t, dag.Container.Env, "API_KEY=secret123")
		assert.Equal(t, []string{"/data:/data:ro", "/output:/output:rw"}, dag.Container.Volumes)
		assert.Equal(t, "1000:1000", dag.Container.User)
		assert.Equal(t, "/app", dag.Container.GetWorkingDir())
		assert.Equal(t, "linux/amd64", dag.Container.Platform)
		assert.Equal(t, []string{"8080:8080", "9090:9090"}, dag.Container.Ports)
		assert.Equal(t, "bridge", dag.Container.Network)
		assert.True(t, dag.Container.KeepContainer)
	})
}

func TestContainerExecutorIntegration(t *testing.T) {
	t.Run("StepInheritsContainerExecutor", func(t *testing.T) {
		yaml := `
container:
  image: python:3.11-slim
steps:
  - name: step1
    command: python script.py
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Step should have docker executor type when DAG has container
		assert.Equal(t, "container", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("ExplicitExecutorOverridesContainer", func(t *testing.T) {
		yaml := `
container:
  image: python:3.11-slim
steps:
  - name: step1
    command: echo test
    type: shell
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Explicit type should override DAG-level container
		assert.Equal(t, "shell", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("NoContainerNoExecutor", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// No container and no executor means default (empty) executor
		assert.Equal(t, "", dag.Steps[0].ExecutorConfig.Type)
	})

	t.Run("StepWithDockerExecutorConfig", func(t *testing.T) {
		yaml := `
container:
  image: node:18-alpine
steps:
  - name: step1
    command: node app.js
    type: docker
    config:
      image: python:3.11
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)

		// Step-level docker config should override DAG container
		assert.Equal(t, "docker", dag.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "python:3.11", dag.Steps[0].ExecutorConfig.Config["image"])
	})

	t.Run("MultipleStepsWithContainer", func(t *testing.T) {
		yaml := `
container:
  image: alpine:latest
steps:
  - name: step1
    command: echo "step 1"
  - name: step2
    command: echo "step 2"
    type: shell
  - name: step3
    command: echo "step 3"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)

		// Step 1 and 3 should inherit docker executor
		assert.Equal(t, "container", dag.Steps[0].ExecutorConfig.Type)
		assert.Equal(t, "shell", dag.Steps[1].ExecutorConfig.Type)
		assert.Equal(t, "container", dag.Steps[2].ExecutorConfig.Type)
	})
}

func TestSSHInheritance(t *testing.T) {
	t.Run("StepInheritsSSHFromDAG", func(t *testing.T) {
		yaml := `
ssh:
  user: testuser
  host: example.com
  key: ~/.ssh/id_rsa
steps:
  - name: step1
    command: echo hello
  - name: step2
    command: ls -la
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// Both steps should inherit SSH executor
		for _, step := range dag.Steps {
			assert.Equal(t, "ssh", step.ExecutorConfig.Type)
		}
	})

	t.Run("StepOverridesSSHConfig", func(t *testing.T) {
		yaml := `
ssh:
  user: defaultuser
  host: default.com
  key: ~/.ssh/default_key
steps:
  - name: step1
    command: echo hello
    type: ssh
    config:
      user: overrideuser
      ip: override.com
  - name: step2
    command: echo world
    type: command
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 2)

		// Step 1 should have overridden values
		step1 := dag.Steps[0]
		assert.Equal(t, "ssh", step1.ExecutorConfig.Type)

		// Step 2 should use command executor
		step2 := dag.Steps[1]
		assert.Equal(t, "command", step2.ExecutorConfig.Type)
	})
}

func TestStepLevelEnv(t *testing.T) {
	t.Run("BasicStepEnv", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo $STEP_VAR
    env:
      - STEP_VAR: step_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Equal(t, []string{"STEP_VAR=step_value"}, dag.Steps[0].Env)
	})

	t.Run("StepEnvOverridesDAGEnv", func(t *testing.T) {
		yaml := `
env:
  - SHARED_VAR: dag_value
  - DAG_ONLY: dag_only_value
steps:
  - name: step1
    command: echo $SHARED_VAR
    env:
      - SHARED_VAR: step_value
      - STEP_ONLY: step_only_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		// Check DAG-level env
		assert.Contains(t, dag.Env, "SHARED_VAR=dag_value")
		assert.Contains(t, dag.Env, "DAG_ONLY=dag_only_value")
		// Check step-level env
		assert.Contains(t, dag.Steps[0].Env, "SHARED_VAR=step_value")
		assert.Contains(t, dag.Steps[0].Env, "STEP_ONLY=step_only_value")
	})

	t.Run("StepEnvAsMap", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
    env:
      FOO: foo_value
      BAR: bar_value
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "FOO=foo_value")
		assert.Contains(t, dag.Steps[0].Env, "BAR=bar_value")
	})

	t.Run("StepEnvWithSubstitution", func(t *testing.T) {
		yaml := `
env:
  - BASE_PATH: /tmp
steps:
  - name: step1
    command: echo $FULL_PATH
    env:
      - FULL_PATH: ${BASE_PATH}/data
      - COMPUTED: "` + "`echo computed_value`" + `"
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "FULL_PATH=${BASE_PATH}/data")
		assert.Contains(t, dag.Steps[0].Env, "COMPUTED=`echo computed_value`")
	})

	t.Run("MultipleStepsWithDifferentEnvs", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo $ENV_VAR
    env:
      - ENV_VAR: value1
  - name: step2
    command: echo $ENV_VAR
    env:
      - ENV_VAR: value2
  - name: step3
    command: echo $ENV_VAR
    # No env, should inherit DAG env only
`
		ctx := context.Background()
		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 3)
		assert.Equal(t, []string{"ENV_VAR=value1"}, dag.Steps[0].Env)
		assert.Equal(t, []string{"ENV_VAR=value2"}, dag.Steps[1].Env)
		assert.Empty(t, dag.Steps[2].Env)
	})

	t.Run("StepEnvComplexValues", func(t *testing.T) {
		yaml := `
steps:
  - name: step1
    command: echo test
    env:
      - PATH: "/custom/bin:${PATH}"
      - JSON_CONFIG: '{"key": "value", "nested": {"foo": "bar"}}'
      - MULTI_LINE: |
          line1
          line2
`
		ctx := context.Background()
		// Set PATH env var for substitution test
		origPath := os.Getenv("PATH")
		defer func() { os.Setenv("PATH", origPath) }()
		os.Setenv("PATH", "/usr/bin")

		dag, err := spec.LoadYAML(ctx, []byte(yaml))
		require.NoError(t, err)
		require.Len(t, dag.Steps, 1)
		assert.Contains(t, dag.Steps[0].Env, "PATH=/custom/bin:${PATH}")
		assert.Contains(t, dag.Steps[0].Env, `JSON_CONFIG={"key": "value", "nested": {"foo": "bar"}}`)
		assert.Contains(t, dag.Steps[0].Env, "MULTI_LINE=line1\nline2\n")
	})
}

func TestDAGLoadEnv(t *testing.T) {
	t.Run("LoadEnvWithDotenvAndEnvVars", func(t *testing.T) {
		// Create a temp directory with a .env file
		tempDir := t.TempDir()
		envFile := filepath.Join(tempDir, ".env")
		envContent := "LOAD_ENV_DOTENV_VAR=from_file\n"
		err := os.WriteFile(envFile, []byte(envContent), 0644)
		require.NoError(t, err)

		yaml := fmt.Sprintf(`
workingDir: %s
dotenv: .env
env:
  - LOAD_ENV_ENV_VAR: from_dag
  - LOAD_ENV_ANOTHER_VAR: another_value
steps:
  - echo hello
`, tempDir)

		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// Load environment variables from dotenv file
		dag.LoadDotEnv(context.Background())

		// Verify environment variables are in dag.Env (not process env)
		// Child processes will receive them via cmd.Env = AllEnvs()
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				envMap[key] = value
			}
		}
		assert.Equal(t, "from_file", envMap["LOAD_ENV_DOTENV_VAR"])
		assert.Equal(t, "from_dag", envMap["LOAD_ENV_ENV_VAR"])
		assert.Equal(t, "another_value", envMap["LOAD_ENV_ANOTHER_VAR"])
	})

	t.Run("LoadEnvWithMissingDotenvFile", func(t *testing.T) {
		yaml := `
dotenv: nonexistent.env
env:
  - TEST_VAR_LOAD_ENV: test_value
steps:
  - echo hello
`
		dag, err := spec.LoadYAMLWithOpts(context.Background(), []byte(yaml), spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		require.NotNil(t, dag)

		// LoadDotEnv should not fail even if dotenv file doesn't exist
		dag.LoadDotEnv(context.Background())

		// Environment variables from env should still be in dag.Env
		envMap := make(map[string]string)
		for _, env := range dag.Env {
			key, value, found := strings.Cut(env, "=")
			if found {
				envMap[key] = value
			}
		}
		assert.Equal(t, "test_value", envMap["TEST_VAR_LOAD_ENV"])
	})
}

func TestBuildShell(t *testing.T) {
	// Shell is no longer expanded at build time - expansion happens at runtime
	// See runtime/env.go Shell() method
	// Standard parsing cases are covered by types/shell_test.go
	t.Run("WithEnvVarPreserved", func(t *testing.T) {
		t.Setenv("MY_SHELL", "/bin/zsh")
		data := []byte(`
shell: $MY_SHELL
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		// Expects unexpanded value (expansion deferred to runtime)
		assert.Equal(t, "$MY_SHELL", dag.Shell)
		assert.Empty(t, dag.ShellArgs)
	})

	t.Run("ArrayWithEnvVarPreserved", func(t *testing.T) {
		t.Setenv("SHELL_ARG", "-x")
		data := []byte(`
shell:
  - bash
  - $SHELL_ARG
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, "bash", dag.Shell)
		// Expects unexpanded value (expansion deferred to runtime)
		assert.Equal(t, []string{"$SHELL_ARG"}, dag.ShellArgs)
	})

	// NoEval tests (cannot use t.Parallel due to t.Setenv)
	t.Run("NoEvalPreservesRaw", func(t *testing.T) {
		t.Setenv("MY_SHELL", "/bin/zsh")
		data := []byte(`
shell: $MY_SHELL -e
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		assert.Equal(t, "$MY_SHELL", dag.Shell)
		assert.Equal(t, []string{"-e"}, dag.ShellArgs)
	})

	t.Run("ArrayNoEvalPreservesRaw", func(t *testing.T) {
		t.Setenv("SHELL_ARG", "-x")
		data := []byte(`
shell:
  - bash
  - $SHELL_ARG
steps:
  - "echo hello"
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)
		assert.Equal(t, "bash", dag.Shell)
		assert.Equal(t, []string{"$SHELL_ARG"}, dag.ShellArgs)
	})
}

func TestBuildStepShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		yaml              string
		wantDAGShell      string
		wantDAGShellArgs  []string
		wantStepShell     string
		wantStepShellArgs []string
		wantStepEmpty     bool
	}{
		{
			name: "SimpleString",
			yaml: `
steps:
  - name: test
    shell: zsh
    command: echo hello
`,
			wantStepShell:     "zsh",
			wantStepShellArgs: nil,
		},
		{
			name: "StringWithArgs",
			yaml: `
steps:
  - name: test
    shell: bash -e -u
    command: echo hello
`,
			wantStepShell:     "bash",
			wantStepShellArgs: []string{"-e", "-u"},
		},
		{
			name: "Array",
			yaml: `
steps:
  - name: test
    shell:
      - bash
      - -e
      - -o
      - pipefail
    command: echo hello
`,
			wantStepShell:     "bash",
			wantStepShellArgs: []string{"-e", "-o", "pipefail"},
		},
		{
			name: "OverridesDAGShell",
			yaml: `
shell: bash -e
steps:
  - name: test
    shell: zsh
    command: echo hello
`,
			wantDAGShell:      "bash",
			wantDAGShellArgs:  []string{"-e"},
			wantStepShell:     "zsh",
			wantStepShellArgs: nil,
		},
		{
			name: "NotSpecified",
			yaml: `
steps:
  - name: test
    command: echo hello
`,
			wantStepEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dag, err := spec.LoadYAML(context.Background(), []byte(tt.yaml))
			require.NoError(t, err)
			require.Len(t, dag.Steps, 1)

			if tt.wantDAGShell != "" {
				assert.Equal(t, tt.wantDAGShell, dag.Shell)
				assert.Equal(t, tt.wantDAGShellArgs, dag.ShellArgs)
			}

			if tt.wantStepEmpty {
				assert.Empty(t, dag.Steps[0].Shell)
			} else {
				assert.Equal(t, tt.wantStepShell, dag.Steps[0].Shell)
			}

			if tt.wantStepShellArgs == nil {
				assert.Empty(t, dag.Steps[0].ShellArgs)
			} else {
				assert.Equal(t, tt.wantStepShellArgs, dag.Steps[0].ShellArgs)
			}
		})
	}
}

func TestLoadWithOptions(t *testing.T) {
	t.Run("WithoutEval_DisablesEnvExpansion", func(t *testing.T) {
		// Cannot use t.Parallel() with t.Setenv()
		t.Setenv("MY_VAR", "expanded-value")

		data := []byte(`
env:
  - TEST: "${MY_VAR}"
steps:
  - name: test
    command: echo test
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagNoEval})
		require.NoError(t, err)

		// When NoEval is set, the variable should not be expanded
		// dag.Env is []string in format "KEY=VALUE"
		assert.Contains(t, dag.Env, "TEST=${MY_VAR}")
	})

	t.Run("WithAllowBuildErrors_CapturesErrors", func(t *testing.T) {
		t.Parallel()
		data := []byte(`
steps:
  - name: test
    command: echo test
    depends:
      - nonexistent
`)
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagAllowBuildErrors})
		require.NoError(t, err)
		require.NotNil(t, dag)
		assert.NotEmpty(t, dag.BuildErrors)
	})

	t.Run("SkipSchemaValidation_SkipsParamsSchema", func(t *testing.T) {
		t.Parallel()
		// BuildFlagSkipSchemaValidation skips JSON schema validation for params,
		// not YAML structure validation
		data := []byte(`
params:
  schema: "nonexistent-schema.json"
  values:
    key: value
steps:
  - name: test
    command: echo test
`)
		// Without the flag, it would error due to missing schema file
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)

		// With schema validation skipped, it succeeds
		dag, err := spec.LoadYAMLWithOpts(context.Background(), data, spec.BuildOpts{Flags: spec.BuildFlagSkipSchemaValidation})
		require.NoError(t, err)
		require.NotNil(t, dag)
	})
}

func TestBuildLogOutput(t *testing.T) {
	t.Parallel()

	t.Run("DAGLevelSeparate", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: separate
steps:
  - name: step1
    command: echo hello
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, core.LogOutputSeparate, dag.LogOutput)
	})

	t.Run("DAGLevelMerged", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: merged
steps:
  - name: step1
    command: echo hello
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		assert.Equal(t, core.LogOutputMerged, dag.LogOutput)
	})

	t.Run("DAGLevelDefault", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
steps:
  - name: step1
    command: echo hello
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)
		// Empty at build time - default applied in InitializeDefaults
		assert.Equal(t, core.LogOutputMode(""), dag.LogOutput)

		// After InitializeDefaults, should be separate
		core.InitializeDefaults(dag)
		assert.Equal(t, core.LogOutputSeparate, dag.LogOutput)
	})

	t.Run("StepLevelOverride", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: separate
steps:
  - name: step1
    command: echo hello
    logOutput: merged
  - name: step2
    command: echo world
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)

		// DAG level is separate
		assert.Equal(t, core.LogOutputSeparate, dag.LogOutput)

		// Step 1 overrides to merged
		assert.Equal(t, core.LogOutputMerged, dag.Steps[0].LogOutput)

		// Step 2 inherits from DAG (empty means inherit)
		assert.Equal(t, core.LogOutputMode(""), dag.Steps[1].LogOutput)
	})

	t.Run("StepLevelExplicitSeparate", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: merged
steps:
  - name: step1
    command: echo hello
    logOutput: separate
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)

		// DAG level is merged
		assert.Equal(t, core.LogOutputMerged, dag.LogOutput)

		// Step 1 explicitly sets separate
		assert.Equal(t, core.LogOutputSeparate, dag.Steps[0].LogOutput)
	})

	t.Run("CaseInsensitive", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: MERGED
steps:
  - name: step1
    command: echo hello
    logOutput: SEPARATE
`)
		dag, err := spec.LoadYAML(context.Background(), data)
		require.NoError(t, err)

		assert.Equal(t, core.LogOutputMerged, dag.LogOutput)
		assert.Equal(t, core.LogOutputSeparate, dag.Steps[0].LogOutput)
	})

	t.Run("InvalidValue", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
logoutput: invalid
steps:
  - name: step1
    command: echo hello
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logOutput value")
	})

	t.Run("InvalidStepValue", func(t *testing.T) {
		t.Parallel()

		data := []byte(`
name: test-dag
steps:
  - name: step1
    command: echo hello
    logOutput: both
`)
		_, err := spec.LoadYAML(context.Background(), data)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid logOutput value")
	})
}
