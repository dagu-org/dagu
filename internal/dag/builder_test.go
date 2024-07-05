package dag

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/dagu-dev/dagu/internal/config"
	"github.com/stretchr/testify/require"
)

func TestBuilder_BuildErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "NoName",
			input: `
steps:
  - command: echo 1`,
		},
		{
			name: "NoCommand",
			input: `
steps:
  - name: step 1`,
		},
		{
			name: "InvalidEnv",
			input: fmt.Sprintf(`
env: 
  - VAR: %q`, "`invalid`"),
		},
		{
			name:  "InvalidParams",
			input: fmt.Sprintf(`params: %q`, "`invalid`"),
		},
		{
			name:  "InvalidSchedule",
			input: `schedule: "1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := decode(dg)
			require.NoError(t, err)

			_, err = new(builder).build(def, nil)
			require.Error(t, err)
		})
	}
}

func TestBuilder_BuildEnvs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected map[string]string
	}{
		{
			name: "ValidEnv",
			input: `
env: 
  "1": "123"
`,
			expected: map[string]string{"1": "123"},
		},
		{
			name: "ValidEnvWithSubstitution",
			input: `
env: 
  VAR: "` + "`echo 1`" + `"
`,
			expected: map[string]string{"VAR": "1"},
		},
		{
			name: "ValidEnvWithSubstitutionAndEnv",
			input: `
env: 
  - "FOO": "BAR"
  - "FOO": "${FOO}:BAZ"
  - "FOO": "${FOO}:BAR"
  - "FOO": "${FOO}:FOO"
`,
			expected: map[string]string{"FOO": "BAR:BAZ:BAR:FOO"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := decode(dg)
			require.NoError(t, err)

			_, err = new(builder).build(def, nil)
			require.NoError(t, err)

			for k, v := range tt.expected {
				require.Equal(t, v, os.Getenv(k))
			}
		})
	}
}

func TestBuilder_BuildParams(t *testing.T) {
	tests := []struct {
		name     string
		params   string
		env      string
		expected map[string]string
	}{
		{
			name:   "ValidParams",
			params: "x",
			expected: map[string]string{
				"1": "x",
			},
		},
		{
			name:   "TwoParams",
			params: "x y",
			expected: map[string]string{
				"1": "x",
				"2": "y",
			},
		},
		{
			name:   "ThreeParams",
			params: "x yy zzz",
			expected: map[string]string{
				"1": "x",
				"2": "yy",
				"3": "zzz",
			},
		},
		{
			name:   "ParamsWithSubstitution",
			params: "x $1",
			expected: map[string]string{
				"1": "x",
				"2": "x",
			},
		},
		{
			name:   "ComplexParams",
			params: "first P1=foo P2=${FOO} P3=`/bin/echo BAR` X=bar Y=${P1} Z=\"A B C\"",
			env:    "FOO: BAR",
			expected: map[string]string{
				"P1": "foo",
				"P2": "BAR",
				"P3": "BAR",
				"X":  "bar",
				"Y":  "foo",
				"Z":  "A B C",
				"1":  "first",
				"2":  `P1=foo`,
				"3":  `P2=BAR`,
				"4":  `P3=BAR`,
				"5":  `X=bar`,
				"6":  `Y=foo`,
				"7":  `Z=A B C`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := unmarshalData([]byte(fmt.Sprintf(`
env:
  - %s
params: %s
  	`, tt.env, tt.params)))
			require.NoError(t, err)

			def, err := decode(dg)
			require.NoError(t, err)

			_, err = new(builder).build(def, nil)
			require.NoError(t, err)

			for k, v := range tt.expected {
				require.Equal(t, v, os.Getenv(k))
			}
		})
	}
}

func TestBuilder_BuildCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "ValidCommand",
			input: `
steps:
  - name: step1
    command: echo 1`,
		},
		{
			name: "ValidCommandInArray",
			input: `
steps:
  - name: step1
    command: ['echo', '1']`,
		},
		{
			name: "ValidCommandInJSONArray",
			input: `
steps:
  - name: step1
    command: [echo, 1]`,
		},
		{
			name: "ValidCommandInYAMLArray",
			input: `
steps:
  - name: step1
    command:
      - echo
      - 1`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := decode(dg)
			require.NoError(t, err)

			dag, err := new(builder).build(def, nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(dag.Steps) != 1 {
				t.Fatalf("expected 1 step, got %d", len(dag.Steps))
			}

			step := dag.Steps[0]
			require.Equal(t, "echo", step.Command)
			require.Equal(t, []string{"1"}, step.Args)
		})
	}
}

func Test_expandEnv(t *testing.T) {
	t.Run("ExpandEnv", func(t *testing.T) {
		_ = os.Setenv("FOO", "BAR")
		require.Equal(t, expandEnv("${FOO}", false), "BAR")
		require.Equal(t, expandEnv("${FOO}", true), "${FOO}")
	})
}

func TestBuilder_BuildTags(t *testing.T) {
	t.Run("ValidTags", func(t *testing.T) {
		input := `tags: Daily, Monthly`
		expected := []string{"daily", "monthly"}

		m, err := unmarshalData([]byte(input))
		require.NoError(t, err)

		def, err := decode(m)
		require.NoError(t, err)

		dg, err := new(builder).build(def, nil)
		require.NoError(t, err)

		for _, tag := range expected {
			require.True(t, dg.HasTag(tag))
		}

		require.False(t, dg.HasTag("weekly"))
	})
}

func TestBuilder_BuildSchedule(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		expected map[string][]string
	}{
		{
			name: "ValidSchedule",
			input: `
schedule:
  start: "0 1 * * *"
  stop: "0 2 * * *"
  restart: "0 12 * * *"
`,
			expected: map[string][]string{
				"start":   {"0 1 * * *"},
				"stop":    {"0 2 * * *"},
				"restart": {"0 12 * * *"},
			},
		},
		{
			name: "OnlyStartSchedule",
			input: `
schedule:
  start: "0 1 * * *"
`,
			expected: map[string][]string{
				"start": {"0 1 * * *"},
			},
		},
		{
			name: "OnlyStopSchedule",
			input: `schedule:
  stop: "0 1 * * *"
`,
			expected: map[string][]string{
				"stop": {"0 1 * * *"},
			},
		},
		{
			name: "MultipleSchedules",
			input: `
schedule:
  start: 
    - "0 1 * * *"
    - "0 18 * * *"
  stop:
    - "0 2 * * *"
    - "0 20 * * *"
`,
			expected: map[string][]string{
				"start": {"0 1 * * *", "0 18 * * *"},
				"stop":  {"0 2 * * *", "0 20 * * *"},
			},
		},
		{
			name: "InvalidCronExp",
			input: `
schedule:
  stop: "* * * * * * *"
`,
			wantErr: true,
		},
		{
			name: "InvalidKey",
			input: `
schedule:
  invalid: "* * * * * * *"
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := decode(m)
			require.NoError(t, err)

			dg, err := new(builder).build(def, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			for k, v := range tt.expected {
				var actual []Schedule
				switch scheduleKey(k) {
				case scheduleKeyStart:
					actual = dg.Schedule
				case scheduleKeyStop:
					actual = dg.StopSchedule
				case scheduleKeyRestart:
					actual = dg.RestartSchedule
				}

				if len(actual) != len(v) {
					t.Errorf("expected %d schedules, got %d", len(v), len(actual))
				}

				for i, s := range actual {
					if s.Expression != v[i] {
						t.Errorf("expected %s, got %s", v[i], s.Expression)
					}
				}
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Base config has the following values:
	// MailOn: {Failure: true, Success: false}
	t.Run("WithBaseConfig", func(t *testing.T) {
		cfg, err := config.Load()
		require.NoError(t, err)

		// Overwrite the base config with the following values:
		// MailOn: {Failure: false, Success: false}
		loader := NewLoader()
		dg, err := loader.Load(cfg.BaseConfig, path.Join(testdataDir, "overwrite.yaml"), "")
		require.NoError(t, err)

		// The MailOn key should be overwritten.
		require.Equal(t, &MailOn{Failure: false, Success: false}, dg.MailOn)
		require.Equal(t, dg.HistRetentionDays, 7)
	})
	t.Run("WithoutBaseConfig", func(t *testing.T) {
		cfg, err := config.Load()
		require.NoError(t, err)

		// no_overwrite.yaml does not have the MailOn key.
		loader := NewLoader()
		dg, err := loader.Load(cfg.BaseConfig, path.Join(testdataDir, "no_overwrite.yaml"), "")
		require.NoError(t, err)

		// The MailOn key should be the same as the base config.
		require.Equal(t, &MailOn{Failure: true, Success: false}, dg.MailOn)
		require.Equal(t, dg.HistRetentionDays, 30)
	})
}

func TestBuilder_BuildExecutor(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedExec   string
		expectedConfig map[string]any
	}{
		{
			name: "HTTPExecutor",
			input: `
steps:
  - name: S1
    command: echo 1
    executor: http
`,
			expectedExec:   "http",
			expectedConfig: nil,
		},
		{
			name: "HTTPExecutorWithConfig",
			input: `
steps:
  - name: S1
    command: echo 1
    executor:
      type: http
      config:
        key: value
`,
			expectedExec: "http",
			expectedConfig: map[string]any{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dg, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := decode(dg)
			require.NoError(t, err)

			dag, err := new(builder).build(def, nil)
			require.NoError(t, err)

			if len(dag.Steps) != 1 {
				t.Errorf("expected 1 step, got %d", len(dag.Steps))
			}

			require.Equal(t, tt.expectedExec, dag.Steps[0].ExecutorConfig.Type)
			if tt.expectedConfig != nil {
				require.Equal(t, tt.expectedConfig, dag.Steps[0].ExecutorConfig.Config)
			}
		})
	}
}

const (
	testSignalOnStop = `
steps:
  - name: "1"
    command: "true"
    signalOnStop: "SIGINT"
`
	testSignalOnStopInvalid = `
steps:
  - name: "1"
    command: "true"
    signalOnStop: 1000
`
)

func TestBuilder_BuildSignalOnStop(t *testing.T) {
	t.Run("SignalOnStop", func(t *testing.T) {
		loader := NewLoader()

		ret, err := loader.LoadYAML([]byte(testSignalOnStop))
		require.NoError(t, err)
		if len(ret.Steps) != 1 {
			t.Fatalf("expected 1 step, got %d", len(ret.Steps))
		}
		require.Equal(t, ret.Steps[0].SignalOnStop, "SIGINT")
	})
	t.Run("InvalidSignal", func(t *testing.T) {
		loader := NewLoader()

		_, err := loader.LoadYAML([]byte(testSignalOnStopInvalid))
		require.Error(t, err)
	})
}

func Test_convertMap(t *testing.T) {
	t.Run("ValidMap", func(t *testing.T) {
		data := map[string]any{
			"key1": "value1",
			"map": map[any]any{
				"key2": "value2",
				"map": map[any]any{
					"key3": "value3",
				},
			},
		}

		err := convertMap(data)
		require.NoError(t, err)

		m1 := data["map"]
		k1 := reflect.TypeOf(m1).Key().Kind()
		require.True(t, k1 == reflect.String)

		m2 := data["map"].(map[string]any)["map"]
		k2 := reflect.TypeOf(m2).Key().Kind()
		require.True(t, k2 == reflect.String)

		expected := map[string]any{
			"key1": "value1",
			"map": map[string]any{
				"key2": "value2",
				"map": map[string]any{
					"key3": "value3",
				},
			},
		}
		require.Equal(t, expected, data)
	})
	t.Run("InvalidMap", func(t *testing.T) {
		data := map[string]any{
			"key1": "value1",
			"map": map[any]any{
				1: "value2",
			},
		}

		err := convertMap(data)
		require.Error(t, err)
	})
}

func Test_evaluateValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "EnvVar",
			input:    "${TEST_VAR}",
			expected: "test",
		},
		{
			name:     "CommandSubstitution",
			input:    "`echo test`",
			expected: "test",
		},
		{
			name:    "InvalidCommand",
			input:   "`ech test`",
			wantErr: true,
		},
	}

	// Set the environment variable for the tests
	err := os.Setenv("TEST_VAR", "test")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r, err := substituteCommands(os.ExpandEnv(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, r)
		})
	}
}

func Test_parseParams(t *testing.T) {
	t.Run("ParamsWithCommandSubstitution", func(t *testing.T) {
		val := "QUESTION=\"what is your favorite activity?\""
		ret, err := parseParamValue(val, true)
		require.NoError(t, err)
		require.Equal(t, 1, len(ret))
		require.Equal(t, ret[0].name, "QUESTION")
		require.Equal(t, ret[0].value, "what is your favorite activity?")
	})
}
