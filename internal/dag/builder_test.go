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
		input string
	}{
		{
			input: `
steps:
  - command: echo 1`,
		},
		{
			input: `
steps:
  - name: step 1`,
		},
		{
			input: fmt.Sprintf(`
env: 
  - VAR: %q`, "`invalid`"),
		},
		{
			input: fmt.Sprintf(`params: %q`, "`invalid`"),
		},
		{
			input: `schedule: "1"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			d, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(d)
			require.NoError(t, err)

			b := &builder{}

			_, err = b.build(def, nil)
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
			name: "simple key value",
			input: `
env: 
  "1": "123"
`,
			expected: map[string]string{"1": "123"},
		},
		{
			name: "command substitution",
			input: `
env: 
  VAR: "` + "`echo 1`" + `"
`,
			expected: map[string]string{"VAR": "1"},
		},
		{
			name: "env substitution",
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
			d, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(d)
			require.NoError(t, err)

			b := &builder{}
			_, err = b.build(def, nil)
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
			name:   "only one param with value",
			params: "x",
			expected: map[string]string{
				"1": "x",
			},
		},
		{
			name:   "two params with values",
			params: "x y",
			expected: map[string]string{
				"1": "x",
				"2": "y",
			},
		},
		{
			name:   "three params with values",
			params: "x yy zzz",
			expected: map[string]string{
				"1": "x",
				"2": "yy",
				"3": "zzz",
			},
		},
		{
			name:   "params with argument substitution",
			params: "x $1",
			expected: map[string]string{
				"1": "x",
				"2": "x",
			},
		},
		{
			name:   "complex params with argument substitution and command substitution",
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
				"2":  `P1="foo"`,
				"3":  `P2="BAR"`,
				"4":  `P3="BAR"`,
				"5":  `X="bar"`,
				"6":  `Y="foo"`,
				"7":  `Z="A B C"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := unmarshalData([]byte(fmt.Sprintf(`
env:
  - %s
params: %s
  	`, tt.env, tt.params)))
			require.NoError(t, err)

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(d)
			require.NoError(t, err)

			b := &builder{}
			_, err = b.build(def, nil)
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
			name: "simple command with single argument",
			input: `
steps:
  - name: step1
    command: echo 1`,
		},
		{
			name: "JSON array command with single argument",
			input: `
steps:
  - name: step1
    command: ['echo', '1']`,
		},
		{
			name: "JSON array command without quotes",
			input: `
steps:
  - name: step1
    command: [echo, 1]`,
		},
		{
			name: "YAML array command with single argument",
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
			d, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(d)
			require.NoError(t, err)

			b := &builder{}

			dag, err := b.build(def, nil)
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
	_ = os.Setenv("FOO", "BAR")
	require.Equal(t, expandEnv("${FOO}", buildOpts{}), "BAR")
	require.Equal(t, expandEnv("${FOO}", buildOpts{noEval: true}), "${FOO}")
}

func TestBuilder_BuildTags(t *testing.T) {
	t.Run("multiple tags", func(t *testing.T) {
		input := `tags: Daily, Monthly`
		expected := []string{"daily", "monthly"}

		m, err := unmarshalData([]byte(input))
		require.NoError(t, err)

		cdl := &configDefinitionLoader{}
		def, err := cdl.decode(m)
		require.NoError(t, err)

		b := &builder{}
		d, err := b.build(def, nil)
		require.NoError(t, err)

		for _, tag := range expected {
			require.True(t, d.HasTag(tag))
		}

		require.False(t, d.HasTag("weekly"))
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
			name: "start and stop schedules",
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
			name: "start schedule only",
			input: `
schedule:
  start: "0 1 * * *"
`,
			expected: map[string][]string{
				"start": {"0 1 * * *"},
			},
		},
		{
			name: "stop schedule only",
			input: `schedule:
  stop: "0 1 * * *"
`,
			expected: map[string][]string{
				"stop": {"0 1 * * *"},
			},
		},
		{
			name: "multiple schedules for start and stop",
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
			name: "invalid cron expression",
			input: `
schedule:
  stop: "* * * * * * *"
`,
			wantErr: true,
		},
		{
			name: "invalid schedule key",
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

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(m)
			require.NoError(t, err)

			b := &builder{}
			d, err := b.build(def, nil)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			for k, v := range tt.expected {
				var actual []*Schedule
				switch scheduleKey(k) {
				case scheduleStart:
					actual = d.Schedule
				case scheduleStop:
					actual = d.StopSchedule
				case scheduleRestart:
					actual = d.RestartSchedule
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
	t.Run("Overwrite the base config", func(t *testing.T) {
		baseCfg := config.Get().BaseConfig

		d, err := Load(baseCfg, path.Join(testdataDir, "overwrite.yaml"), "")
		require.NoError(t, err)

		require.Equal(t, &MailOn{Failure: false, Success: false}, d.MailOn)
		require.Equal(t, d.HistRetentionDays, 7)
	})
	t.Run("Do not overwrite the base config", func(t *testing.T) {
		baseCfg := config.Get().BaseConfig

		d, err := Load(baseCfg, path.Join(testdataDir, "no_overwrite.yaml"), "")
		require.NoError(t, err)

		require.Equal(t, &MailOn{Failure: true, Success: false}, d.MailOn)
		require.Equal(t, d.HistRetentionDays, 30)
	})
}

func TestBuilder_BuildExecutor(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedExec   string
		expectedConfig map[string]interface{}
	}{
		{
			name: "http executor",
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
			name: "http executor with config",
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
			expectedConfig: map[string]interface{}{
				"key": "value",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			cdl := &configDefinitionLoader{}
			def, err := cdl.decode(d)
			require.NoError(t, err)

			b := &builder{}
			dag, err := b.build(def, nil)
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

func TestBuilder_BuildSignalOnStop(t *testing.T) {
	tests := []struct {
		sig      string
		expected string
		wantErr  bool
	}{
		{
			sig:      "SIGINT",
			expected: "SIGINT",
		},
		{
			sig:     "2000",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.sig, func(t *testing.T) {
			dat := fmt.Sprintf(`name: test DAG
steps:
  - name: "1"
    command: "true"
    signalOnStop: "%s"
`, tt.sig)
			ret, err := LoadYAML([]byte(dat))
			if tt.wantErr != (err != nil) {
				t.Errorf("expected error: %v, got: %v", tt.wantErr, err)
			}
			if tt.wantErr {
				return
			}
			if len(ret.Steps) != 1 {
				t.Fatalf("expected 1 step, got %d", len(ret.Steps))
			}
			require.Equal(t, ret.Steps[0].SignalOnStop, tt.expected)
		})
	}
}

func Test_convertMap(t *testing.T) {
	t.Run("Convert map with string keys", func(t *testing.T) {
		data := map[string]interface{}{
			"key1": "value1",
			"map": map[interface{}]interface{}{
				"key2": "value2",
				"map": map[interface{}]interface{}{
					"key3": "value3",
				},
			},
		}

		err := convertMap(data)
		require.NoError(t, err)

		m1 := data["map"]
		k1 := reflect.TypeOf(m1).Key().Kind()
		require.True(t, k1 == reflect.String)

		m2 := data["map"].(map[string]interface{})["map"]
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
	t.Run("[Invalid] Convert map with non-string keys", func(t *testing.T) {
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
		input    string
		expected string
		wantErr  bool
	}{
		{
			input:    "${TEST_VAR}",
			expected: "test",
		},
		{
			input:    "`echo test`",
			expected: "test",
		},
		{
			input:   "`ech test`",
			wantErr: true,
		},
	}

	// Set the environment variable for the tests
	err := os.Setenv("TEST_VAR", "test")
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r, err := evaluateValue(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expected, r)
		})
	}
}
