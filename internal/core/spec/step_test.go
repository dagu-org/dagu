package spec

import (
	"context"
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/goccy/go-yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testStepBuildContext creates a StepBuildContext for testing
func testStepBuildContext() StepBuildContext {
	return StepBuildContext{
		BuildContext: BuildContext{
			ctx:   context.Background(),
			file:  "/test/dag.yaml",
			opts:  BuildOpts{},
			index: 0,
		},
	}
}

// Helper to create ShellValue from string
func shellValue(s string) types.ShellValue {
	var v types.ShellValue
	_ = yaml.Unmarshal([]byte(`"`+s+`"`), &v)
	return v
}

// Helper to create ShellValue from array
func shellValueArray(args []string) types.ShellValue {
	var v types.ShellValue
	data, _ := yaml.Marshal(args)
	_ = yaml.Unmarshal(data, &v)
	return v
}

// Helper to create ContinueOnValue from string
func continueOnValue(s string) types.ContinueOnValue {
	var v types.ContinueOnValue
	_ = yaml.Unmarshal([]byte(`"`+s+`"`), &v)
	return v
}

// Helper to create ContinueOnValue from map
func continueOnValueMap(m map[string]any) types.ContinueOnValue {
	var v types.ContinueOnValue
	data, _ := yaml.Marshal(m)
	_ = yaml.Unmarshal(data, &v)
	return v
}

// Helper to create EnvValue from map
func envValueMap(m map[string]string) types.EnvValue {
	var v types.EnvValue
	data, _ := yaml.Marshal(m)
	_ = yaml.Unmarshal(data, &v)
	return v
}

func TestBuildStepName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimpleName", input: "my-step", expected: "my-step"},
		{name: "Trimmed", input: "  step  ", expected: "step"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Name: tt.input}
			result, err := buildStepName(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimpleID", input: "step-1", expected: "step-1"},
		{name: "Trimmed", input: "  id  ", expected: "id"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{ID: tt.input}
			result, err := buildStepID(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimpleDescription", input: "My step description", expected: "My step description"},
		{name: "Trimmed", input: "  description  ", expected: "description"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Description: tt.input}
			result, err := buildStepDescription(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepShellPackages(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{name: "SinglePackage", input: []string{"python3"}, expected: []string{"python3"}},
		{name: "MultiplePackages", input: []string{"python3", "nodejs"}, expected: []string{"python3", "nodejs"}},
		{name: "Empty", input: nil, expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{ShellPackages: tt.input}
			result, err := buildStepShellPackages(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepScript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimpleScript", input: "echo hello", expected: "echo hello"},
		{name: "MultilineScript", input: "echo hello\necho world", expected: "echo hello\necho world"},
		{name: "Trimmed", input: "  script  ", expected: "script"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Script: tt.input}
			result, err := buildStepScript(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepStdout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimplePath", input: "/tmp/output.log", expected: "/tmp/output.log"},
		{name: "Trimmed", input: "  /tmp/out.log  ", expected: "/tmp/out.log"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Stdout: tt.input}
			result, err := buildStepStdout(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepStderr(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimplePath", input: "/tmp/error.log", expected: "/tmp/error.log"},
		{name: "Trimmed", input: "  /tmp/err.log  ", expected: "/tmp/err.log"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Stderr: tt.input}
			result, err := buildStepStderr(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepMailOnError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    bool
		expected bool
	}{
		{name: "True", input: true, expected: true},
		{name: "False", input: false, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{MailOnError: tt.input}
			result, err := buildStepMailOnError(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepWorkerSelector(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{name: "SingleLabel", input: map[string]string{"env": "prod"}, expected: map[string]string{"env": "prod"}},
		{name: "MultipleLabels", input: map[string]string{"env": "prod", "region": "us-west"}, expected: map[string]string{"env": "prod", "region": "us-west"}},
		{name: "Empty", input: nil, expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{WorkerSelector: tt.input}
			result, err := buildStepWorkerSelector(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepWorkingDir(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		workingDir string
		dir        string // deprecated field
		expected   string
	}{
		{name: "FromWorkingDir", workingDir: "/path/to/dir", dir: "", expected: "/path/to/dir"},
		{name: "FromDeprecatedDir", workingDir: "", dir: "/old/path", expected: "/old/path"},
		{name: "WorkingDirTakesPrecedence", workingDir: "/new/path", dir: "/old/path", expected: "/new/path"},
		{name: "Trimmed", workingDir: "  /path  ", dir: "", expected: "/path"},
		{name: "Empty", workingDir: "", dir: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{WorkingDir: tt.workingDir, Dir: tt.dir}
			result, err := buildStepWorkingDir(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepShell(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		shell    types.ShellValue
		expected string
	}{
		{name: "SimpleShell", shell: shellValue("bash"), expected: "bash"},
		{name: "ShellWithArgsAsString", shell: shellValue("bash -e"), expected: "bash"},
		{name: "ShellAsArray", shell: shellValueArray([]string{"bash", "-e", "-x"}), expected: "bash"},
		{name: "Empty", shell: types.ShellValue{}, expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Shell: tt.shell}
			result, err := buildStepShell(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepShellArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		shell    types.ShellValue
		expected []string
	}{
		{name: "NoArgs", shell: shellValue("bash"), expected: []string{}},
		{name: "ShellWithArgsAsString", shell: shellValue("bash -e"), expected: []string{"-e"}},
		{name: "ShellAsArray", shell: shellValueArray([]string{"bash", "-e", "-x"}), expected: []string{"-e", "-x"}},
		{name: "Empty", shell: types.ShellValue{}, expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Shell: tt.shell}
			result, err := buildStepShellArgs(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepTimeout(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    int
		expected time.Duration
		wantErr  bool
	}{
		{name: "PositiveTimeout", input: 60, expected: 60 * time.Second},
		{name: "ZeroTimeout", input: 0, expected: 0},
		{name: "NegativeTimeout", input: -1, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{TimeoutSec: tt.input}
			result, err := buildStepTimeout(testStepBuildContext(), s)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepDepends(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		depends  types.StringOrArray
		expected []string
	}{
		{name: "SingleDependency", depends: stringOrArray("step1"), expected: []string{"step1"}},
		{name: "MultipleDependencies", depends: stringOrArrayList([]string{"step1", "step2"}), expected: []string{"step1", "step2"}},
		{name: "Empty", depends: types.StringOrArray{}, expected: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Depends: tt.depends}
			result, err := buildStepDepends(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepExplicitlyNoDeps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		depends  types.StringOrArray
		expected bool
	}{
		{name: "ExplicitEmptyArray", depends: stringOrArrayList([]string{}), expected: true},
		{name: "HasDependencies", depends: stringOrArrayList([]string{"step1"}), expected: false},
		{name: "ZeroValue", depends: types.StringOrArray{}, expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Depends: tt.depends}
			result, err := buildStepExplicitlyNoDeps(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepContinueOn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		continueOn types.ContinueOnValue
		expected   core.ContinueOn
	}{
		{
			name:       "SkippedString",
			continueOn: continueOnValue("skipped"),
			expected:   core.ContinueOn{Skipped: true},
		},
		{
			name:       "FailedString",
			continueOn: continueOnValue("failed"),
			expected:   core.ContinueOn{Failure: true},
		},
		{
			name: "ObjectWithMultipleFields",
			continueOn: continueOnValueMap(map[string]any{
				"skipped":     true,
				"failed":      true,
				"markSuccess": true,
			}),
			expected: core.ContinueOn{Skipped: true, Failure: true, MarkSuccess: true},
		},
		{
			name:       "Empty",
			continueOn: types.ContinueOnValue{},
			expected:   core.ContinueOn{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{ContinueOn: tt.continueOn}
			result, err := buildStepContinueOn(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepRetryPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		retryPolicy *retryPolicy
		expected    core.RetryPolicy
		wantErr     bool
	}{
		{
			name:        "NilPolicy",
			retryPolicy: nil,
			expected:    core.RetryPolicy{},
		},
		{
			name: "BasicPolicyWithIntValues",
			retryPolicy: &retryPolicy{
				Limit:       3,
				IntervalSec: 10,
			},
			expected: core.RetryPolicy{
				Limit:    3,
				Interval: 10 * time.Second,
			},
		},
		{
			name: "PolicyWithStringLimit",
			retryPolicy: &retryPolicy{
				Limit:       "${RETRY_LIMIT}",
				IntervalSec: 5,
			},
			expected: core.RetryPolicy{
				LimitStr: "${RETRY_LIMIT}",
				Interval: 5 * time.Second,
			},
		},
		{
			name: "PolicyWithExitCodes",
			retryPolicy: &retryPolicy{
				Limit:       2,
				IntervalSec: 5,
				ExitCode:    []int{1, 2, 3},
			},
			expected: core.RetryPolicy{
				Limit:     2,
				Interval:  5 * time.Second,
				ExitCodes: []int{1, 2, 3},
			},
		},
		{
			name: "PolicyWithBackoffTrue",
			retryPolicy: &retryPolicy{
				Limit:       3,
				IntervalSec: 5,
				Backoff:     true,
			},
			expected: core.RetryPolicy{
				Limit:    3,
				Interval: 5 * time.Second,
				Backoff:  2.0,
			},
		},
		{
			name: "PolicyWithInvalidBackoffMultiplier",
			retryPolicy: &retryPolicy{
				Limit:       3,
				IntervalSec: 5,
				Backoff:     0.5,
			},
			wantErr: true,
		},
		{
			name: "PolicyWithValidBackoffMultiplier",
			retryPolicy: &retryPolicy{
				Limit:       3,
				IntervalSec: 5,
				Backoff:     2.5,
			},
			expected: core.RetryPolicy{
				Limit:    3,
				Interval: 5 * time.Second,
				Backoff:  2.5,
			},
		},
		{
			name: "PolicyWithMaxInterval",
			retryPolicy: &retryPolicy{
				Limit:          3,
				IntervalSec:    5,
				Backoff:        2.0,
				MaxIntervalSec: 60,
			},
			expected: core.RetryPolicy{
				Limit:       3,
				Interval:    5 * time.Second,
				Backoff:     2.0,
				MaxInterval: 60 * time.Second,
			},
		},
		{
			name: "MissingLimit",
			retryPolicy: &retryPolicy{
				IntervalSec: 5,
			},
			wantErr: true,
		},
		{
			name: "MissingIntervalSec",
			retryPolicy: &retryPolicy{
				Limit: 3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{RetryPolicy: tt.retryPolicy}
			result, err := buildStepRetryPolicy(testStepBuildContext(), s)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepRepeatPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		repeatPolicy *repeatPolicy
		expected     core.RepeatPolicy
		wantErr      bool
	}{
		{
			name:         "NilPolicy",
			repeatPolicy: nil,
			expected:     core.RepeatPolicy{},
		},
		{
			name: "WhileModeWithCondition",
			repeatPolicy: &repeatPolicy{
				Repeat:      "while",
				Condition:   "test -f /tmp/flag",
				IntervalSec: 5,
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeWhile,
				Condition:  &core.Condition{Condition: "test -f /tmp/flag"},
				Interval:   5 * time.Second,
			},
		},
		{
			name: "UntilModeWithConditionAndExpected",
			repeatPolicy: &repeatPolicy{
				Repeat:      "until",
				Condition:   "cat /tmp/status",
				Expected:    "done",
				IntervalSec: 10,
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeUntil,
				Condition:  &core.Condition{Condition: "cat /tmp/status", Expected: "done"},
				Interval:   10 * time.Second,
			},
		},
		{
			name: "LegacyBooleanTrue",
			repeatPolicy: &repeatPolicy{
				Repeat:    true,
				Condition: "test condition",
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeWhile,
				Condition:  &core.Condition{Condition: "test condition"},
			},
		},
		{
			name: "WithExitCodes",
			repeatPolicy: &repeatPolicy{
				Repeat:   "while",
				ExitCode: []int{0, 1},
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeWhile,
				ExitCode:   []int{0, 1},
			},
		},
		{
			name: "WithLimit",
			repeatPolicy: &repeatPolicy{
				Repeat:    "while",
				Condition: "true",
				Limit:     10,
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeWhile,
				Condition:  &core.Condition{Condition: "true"},
				Limit:      10,
			},
		},
		{
			name: "WithBackoff",
			repeatPolicy: &repeatPolicy{
				Repeat:      "while",
				Condition:   "true",
				IntervalSec: 5,
				Backoff:     2.0,
			},
			expected: core.RepeatPolicy{
				RepeatMode: core.RepeatModeWhile,
				Condition:  &core.Condition{Condition: "true"},
				Interval:   5 * time.Second,
				Backoff:    2.0,
			},
		},
		{
			name: "WithMaxInterval",
			repeatPolicy: &repeatPolicy{
				Repeat:         "while",
				Condition:      "true",
				IntervalSec:    5,
				Backoff:        2.0,
				MaxIntervalSec: 120,
			},
			expected: core.RepeatPolicy{
				RepeatMode:  core.RepeatModeWhile,
				Condition:   &core.Condition{Condition: "true"},
				Interval:    5 * time.Second,
				Backoff:     2.0,
				MaxInterval: 120 * time.Second,
			},
		},
		{
			name: "InvalidRepeatValue",
			repeatPolicy: &repeatPolicy{
				Repeat: "invalid",
			},
			wantErr: true,
		},
		{
			name: "WhileWithoutConditionOrExitCode",
			repeatPolicy: &repeatPolicy{
				Repeat: "while",
			},
			wantErr: true,
		},
		{
			name: "InvalidBackoff",
			repeatPolicy: &repeatPolicy{
				Repeat:    "while",
				Condition: "true",
				Backoff:   0.5,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{RepeatPolicy: tt.repeatPolicy}
			result, err := buildStepRepeatPolicy(testStepBuildContext(), s)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepSignalOnStop(t *testing.T) {
	t.Parallel()

	sigTerm := "SIGTERM"
	sigKill := "SIGKILL"
	sigInt := "SIGINT"
	invalid := "INVALID"

	tests := []struct {
		name         string
		signalOnStop *string
		expected     string
		wantErr      bool
	}{
		{name: "Nil", signalOnStop: nil, expected: ""},
		{name: "SIGTERM", signalOnStop: &sigTerm, expected: "SIGTERM"},
		{name: "SIGKILL", signalOnStop: &sigKill, expected: "SIGKILL"},
		{name: "SIGINT", signalOnStop: &sigInt, expected: "SIGINT"},
		{name: "InvalidSignal", signalOnStop: &invalid, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{SignalOnStop: tt.signalOnStop}
			result, err := buildStepSignalOnStop(testStepBuildContext(), s)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "SimpleVariable", input: "MY_VAR", expected: "MY_VAR"},
		{name: "WithDollarPrefix", input: "$MY_VAR", expected: "MY_VAR"},
		{name: "Trimmed", input: "  OUTPUT  ", expected: "OUTPUT"},
		{name: "Empty", input: "", expected: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Output: tt.input}
			result, err := buildStepOutput(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepEnvs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		env      types.EnvValue
		expected []string
	}{
		{
			name:     "SingleEnv",
			env:      envValueMap(map[string]string{"KEY": "value"}),
			expected: []string{"KEY=value"},
		},
		{
			name:     "Empty",
			env:      types.EnvValue{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Env: tt.env}
			result, err := buildStepEnvs(testStepBuildContext(), s)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		command  any
		expected struct {
			command     string
			args        []string
			cmdWithArgs string
			script      string
		}
		wantErr bool
	}{
		{
			name:    "NilCommand",
			command: nil,
		},
		{
			name:    "SimpleStringCommand",
			command: "echo hello",
			expected: struct {
				command     string
				args        []string
				cmdWithArgs string
				script      string
			}{
				command:     "echo",
				args:        []string{"hello"},
				cmdWithArgs: "echo hello",
			},
		},
		{
			name:    "MultilineCommandBecomesScript",
			command: "echo hello\necho world",
			expected: struct {
				command     string
				args        []string
				cmdWithArgs string
				script      string
			}{
				script: "echo hello\necho world",
			},
		},
		{
			name:    "ArrayCommand",
			command: []any{"echo", "hello", "world"},
			expected: struct {
				command     string
				args        []string
				cmdWithArgs string
				script      string
			}{
				command:     "echo",
				args:        []string{"hello", "world"},
				cmdWithArgs: `echo "hello" "world"`,
			},
		},
		{
			name:    "EmptyStringCommand",
			command: "   ",
			wantErr: true,
		},
		{
			name:    "InvalidType",
			command: 123,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Command: tt.command}
			result := &core.Step{ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)}}
			err := buildStepCommand(testStepBuildContext(), s, result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.command, result.Command)
			assert.Equal(t, tt.expected.args, result.Args)
			assert.Equal(t, tt.expected.cmdWithArgs, result.CmdWithArgs)
			assert.Equal(t, tt.expected.script, result.Script)
		})
	}
}

func TestBuildStepExecutor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		step     *step
		ctx      StepBuildContext
		expected core.ExecutorConfig
		wantErr  bool
	}{
		{
			name:     "NilExecutor",
			step:     &step{},
			ctx:      testStepBuildContext(),
			expected: core.ExecutorConfig{Config: make(map[string]any)},
		},
		{
			name:     "StringExecutor",
			step:     &step{Executor: "http"},
			ctx:      testStepBuildContext(),
			expected: core.ExecutorConfig{Type: "http", Config: make(map[string]any)},
		},
		{
			name: "ObjectExecutor",
			step: &step{
				Executor: map[string]any{
					"type": "docker",
					"config": map[string]any{
						"image": "alpine:latest",
					},
				},
			},
			ctx: testStepBuildContext(),
			expected: core.ExecutorConfig{
				Type:   "docker",
				Config: map[string]any{"image": "alpine:latest"},
			},
		},
		{
			name: "InheritsContainerExecutor",
			step: &step{},
			ctx: StepBuildContext{
				BuildContext: testBuildContext(),
				dag:          &core.DAG{Container: &core.Container{Image: "alpine"}},
			},
			expected: core.ExecutorConfig{Type: "container", Config: make(map[string]any)},
		},
		{
			name: "InheritsSSHExecutor",
			step: &step{},
			ctx: StepBuildContext{
				BuildContext: testBuildContext(),
				dag:          &core.DAG{SSH: &core.SSHConfig{Host: "example.com"}},
			},
			expected: core.ExecutorConfig{Type: "ssh", Config: make(map[string]any)},
		},
		{
			name: "InvalidExecutorType",
			step: &step{
				Executor: map[string]any{
					"type": 123,
				},
			},
			ctx:     testStepBuildContext(),
			wantErr: true,
		},
		{
			name:    "InvalidExecutorValue",
			step:    &step{Executor: 123},
			ctx:     testStepBuildContext(),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &core.Step{ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)}}
			err := buildStepExecutor(tt.ctx, tt.step, result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected.Type, result.ExecutorConfig.Type)
			assert.Equal(t, tt.expected.Config, result.ExecutorConfig.Config)
		})
	}
}

func TestBuildStepParamsField(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		params     any
		wantEmpty  bool
		wantParams map[string]string
	}{
		{
			name:      "NilParams",
			params:    nil,
			wantEmpty: true,
		},
		{
			name: "ParamsAsMap",
			params: map[string]any{
				"repository": "myorg/myrepo",
				"ref":        "main",
				"token":      "secret123",
			},
			wantParams: map[string]string{
				"repository": "myorg/myrepo",
				"ref":        "main",
				"token":      "secret123",
			},
		},
		{
			name:   "ParamsAsString",
			params: "go-version=1.21 cache=true",
			wantParams: map[string]string{
				"go-version": "1.21",
				"cache":      "true",
			},
		},
		{
			name: "ParamsWithNumbers",
			params: map[string]any{
				"timeout": 300,
				"retries": 3,
				"enabled": true,
			},
			wantParams: map[string]string{
				"timeout": "300",
				"retries": "3",
				"enabled": "true",
			},
		},
		{
			name:       "EmptyMap",
			params:     map[string]any{},
			wantParams: map[string]string{},
		},
		{
			name:   "ParamsWithQuotedValues",
			params: `message="hello world" count="42"`,
			wantParams: map[string]string{
				"message": "hello world",
				"count":   "42",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s := &step{Params: tt.params}
			result := &core.Step{}
			err := buildStepParamsField(testStepBuildContext(), s, result)
			require.NoError(t, err)

			if tt.wantEmpty {
				assert.True(t, result.Params.IsEmpty())
				return
			}

			params, err := result.Params.AsStringMap()
			require.NoError(t, err)
			if len(tt.wantParams) == 0 {
				assert.Empty(t, params)
			} else {
				for k, v := range tt.wantParams {
					assert.Equal(t, v, params[k])
				}
			}
		})
	}
}

func TestBuildStepParallel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		parallel any
		expected *core.ParallelConfig
		wantErr  bool
	}{
		{
			name:     "NilParallel",
			parallel: nil,
			expected: nil,
		},
		{
			name:     "StringVariableReference",
			parallel: "${ITEMS}",
			expected: &core.ParallelConfig{Variable: "${ITEMS}", MaxConcurrent: core.DefaultMaxConcurrent},
		},
		{
			name:     "StaticArrayOfStrings",
			parallel: []any{"item1", "item2", "item3"},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Value: "item1"},
					{Value: "item2"},
					{Value: "item3"},
				},
				MaxConcurrent: core.DefaultMaxConcurrent,
			},
		},
		{
			name:     "ArrayOfNumbers",
			parallel: []any{1, 2, 3},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Value: "1"},
					{Value: "2"},
					{Value: "3"},
				},
				MaxConcurrent: core.DefaultMaxConcurrent,
			},
		},
		{
			name: "ArrayOfObjects",
			parallel: []any{
				map[string]any{"name": "first", "value": 100},
				map[string]any{"name": "second", "value": 200},
			},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Params: map[string]string{"name": "first", "value": "100"}},
					{Params: map[string]string{"name": "second", "value": "200"}},
				},
				MaxConcurrent: core.DefaultMaxConcurrent,
			},
		},
		{
			name: "ObjectConfigWithItemsArray",
			parallel: map[string]any{
				"items":         []any{"a", "b"},
				"maxConcurrent": 5,
			},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Value: "a"},
					{Value: "b"},
				},
				MaxConcurrent: 5,
			},
		},
		{
			name: "ObjectConfigWithVariableReference",
			parallel: map[string]any{
				"items":         "${MY_ITEMS}",
				"maxConcurrent": 3,
			},
			expected: &core.ParallelConfig{
				Variable:      "${MY_ITEMS}",
				MaxConcurrent: 3,
			},
		},
		{
			name: "MaxConcurrentAsInt64",
			parallel: map[string]any{
				"items":         "${ITEMS}",
				"maxConcurrent": int64(10),
			},
			expected: &core.ParallelConfig{
				Variable:      "${ITEMS}",
				MaxConcurrent: 10,
			},
		},
		{
			name: "MaxConcurrentAsFloat64",
			parallel: map[string]any{
				"items":         "${ITEMS}",
				"maxConcurrent": float64(7),
			},
			expected: &core.ParallelConfig{
				Variable:      "${ITEMS}",
				MaxConcurrent: 7,
			},
		},
		{
			name:     "InvalidType",
			parallel: 123,
			wantErr:  true,
		},
		{
			name: "InvalidItemsType",
			parallel: map[string]any{
				"items": 123,
			},
			wantErr: true,
		},
		{
			name: "InvalidMaxConcurrentType",
			parallel: map[string]any{
				"items":         "${ITEMS}",
				"maxConcurrent": "invalid",
			},
			wantErr: true,
		},
		{
			name:     "InvalidItemValue_NestedArray",
			parallel: []any{[]any{"nested", "array"}},
			wantErr:  true,
		},
		{
			name: "ObjectItemWithBool",
			parallel: []any{
				map[string]any{"name": "test", "enabled": true},
			},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Params: map[string]string{"name": "test", "enabled": "true"}},
				},
				MaxConcurrent: core.DefaultMaxConcurrent,
			},
		},
		{
			name: "ObjectItemWithFloat",
			parallel: []any{
				map[string]any{"name": "test", "rate": 3.14},
			},
			expected: &core.ParallelConfig{
				Items: []core.ParallelItem{
					{Params: map[string]string{"name": "test", "rate": "3.14"}},
				},
				MaxConcurrent: core.DefaultMaxConcurrent,
			},
		},
		{
			name: "InvalidObjectParamType_NestedMap",
			parallel: []any{
				map[string]any{"name": "test", "nested": map[string]any{"key": "value"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Parallel: tt.parallel}
			result := &core.Step{ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)}}
			err := buildStepParallel(testStepBuildContext(), s, result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.Parallel)
		})
	}
}

func TestBuildStepSubDAG(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		step     *step
		expected *core.SubDAG
	}{
		{
			name:     "NoSubDAG",
			step:     &step{},
			expected: nil,
		},
		{
			name:     "SimpleCall",
			step:     &step{Call: "other-dag"},
			expected: &core.SubDAG{Name: "other-dag", Params: ""},
		},
		{
			name: "CallWithParams",
			step: &step{
				Call:   "other-dag",
				Params: map[string]any{"key": "value"},
			},
			expected: &core.SubDAG{Name: "other-dag", Params: `key="value"`},
		},
		{
			name:     "LegacyRunField",
			step:     &step{Run: "legacy-dag"},
			expected: &core.SubDAG{Name: "legacy-dag", Params: ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &core.Step{ExecutorConfig: core.ExecutorConfig{Config: make(map[string]any)}}
			err := buildStepSubDAG(testStepBuildContext(), tt.step, result)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.SubDAG)
		})
	}
}

func TestParseParallelItems(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		items    []any
		expected []core.ParallelItem
		wantErr  bool
	}{
		{
			name:  "StringItems",
			items: []any{"a", "b", "c"},
			expected: []core.ParallelItem{
				{Value: "a"},
				{Value: "b"},
				{Value: "c"},
			},
		},
		{
			name:  "NumericItems",
			items: []any{1, 2.5, int64(3)},
			expected: []core.ParallelItem{
				{Value: "1"},
				{Value: "2.5"},
				{Value: "3"},
			},
		},
		{
			name: "ObjectItems",
			items: []any{
				map[string]any{"name": "item1", "count": 5},
				map[string]any{"name": "item2", "count": 10},
			},
			expected: []core.ParallelItem{
				{Params: map[string]string{"name": "item1", "count": "5"}},
				{Params: map[string]string{"name": "item2", "count": "10"}},
			},
		},
		{
			name:    "InvalidItemType",
			items:   []any{[]string{"nested", "array"}},
			wantErr: true,
		},
		{
			name: "InvalidParamValueType",
			items: []any{
				map[string]any{"nested": []string{"array"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseParallelItems(tt.items)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildStepContainer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *container
		expected *core.Container
		wantErr  bool
	}{
		{
			name:     "Nil",
			input:    nil,
			expected: nil,
		},
		{
			name:    "MissingImage",
			input:   &container{},
			wantErr: true,
		},
		{
			name: "BasicContainer",
			input: &container{
				Image: "alpine:latest",
			},
			expected: &core.Container{
				Image:      "alpine:latest",
				PullPolicy: core.PullPolicyMissing,
			},
		},
		{
			name: "FullContainerConfig",
			input: &container{
				Name:          "my-step-container",
				Image:         "golang:1.22",
				PullPolicy:    "always",
				Volumes:       []string{"./src:/app"},
				User:          "1000",
				WorkingDir:    "/app",
				Platform:      "linux/amd64",
				Ports:         []string{"8080:8080"},
				Network:       "host",
				KeepContainer: false,
				Startup:       "entrypoint",
				Command:       []string{"go", "build"},
				WaitFor:       "running",
				LogPattern:    "ready",
				RestartPolicy: "no",
			},
			expected: &core.Container{
				Name:          "my-step-container",
				Image:         "golang:1.22",
				PullPolicy:    core.PullPolicyAlways,
				Volumes:       []string{"./src:/app"},
				User:          "1000",
				WorkingDir:    "/app",
				Platform:      "linux/amd64",
				Ports:         []string{"8080:8080"},
				Network:       "host",
				KeepContainer: false,
				Startup:       core.StartupEntrypoint,
				Command:       []string{"go", "build"},
				WaitFor:       core.WaitForRunning,
				LogPattern:    "ready",
				RestartPolicy: "no",
			},
		},
		{
			name: "ContainerWithEnvAsMap",
			input: &container{
				Image: "node:20",
				Env:   map[string]any{"NODE_ENV": "production"},
			},
			expected: &core.Container{
				Image:      "node:20",
				PullPolicy: core.PullPolicyMissing,
				Env:        []string{"NODE_ENV=production"},
			},
		},
		{
			name: "ContainerWithVolumes",
			input: &container{
				Image:   "postgres:16",
				Volumes: []string{"./data:/var/lib/postgresql/data", "/tmp:/tmp:ro"},
			},
			expected: &core.Container{
				Image:      "postgres:16",
				PullPolicy: core.PullPolicyMissing,
				Volumes:    []string{"./data:/var/lib/postgresql/data", "/tmp:/tmp:ro"},
			},
		},
		{
			name: "PullPolicyNever",
			input: &container{
				Image:      "myimage:local",
				PullPolicy: "never",
			},
			expected: &core.Container{
				Image:      "myimage:local",
				PullPolicy: core.PullPolicyNever,
			},
		},
		{
			name: "PullPolicyMissing",
			input: &container{
				Image:      "alpine:3.18",
				PullPolicy: "missing",
			},
			expected: &core.Container{
				Image:      "alpine:3.18",
				PullPolicy: core.PullPolicyMissing,
			},
		},
		{
			name: "BackwardCompatWorkDir",
			input: &container{
				Image:   "alpine:latest",
				WorkDir: "/legacy",
			},
			expected: &core.Container{
				Image:      "alpine:latest",
				PullPolicy: core.PullPolicyMissing,
				WorkingDir: "/legacy",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &step{Container: tt.input}
			result := &core.Step{}
			err := buildStepContainer(testStepBuildContext(), s, result)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, result.Container)
		})
	}
}
