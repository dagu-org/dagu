package dag

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
	testdataDir = path.Join(utils.MustGetwd(), "testdata")
	testHomeDir = path.Join(utils.MustGetwd(), "testdata/home")
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

func TestBuildErrors(t *testing.T) {
	tests := []struct {
		input         string
		expectedError string
	}{
		{
			input: `
steps:
  - command: echo 1`,
			expectedError: "step name must be specified",
		},
		{
			input: `
steps:
  - name: step 1`,
			expectedError: "step command must be specified",
		},
	}

	for _, tt := range tests {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(tt.input))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), tt.expectedError)
	}
}

func TestConfigReadClone(t *testing.T) {
	l := &Loader{}

	d, err := l.Load(path.Join(testdataDir, "default.yaml"), "")
	require.NoError(t, err)

	dd := d.Clone()
	require.Equal(t, d, dd)
}

func TestToString(t *testing.T) {
	l := &Loader{}

	d, err := l.Load(path.Join(testdataDir, "default.yaml"), "")
	require.NoError(t, err)

	ret := d.String()
	require.Contains(t, ret, "Name: default")
}

func TestReadConfig(t *testing.T) {
	tmpDir := utils.MustTempDir("read-config-test")
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	tmpFile := path.Join(tmpDir, "config.yaml")
	input := `steps:
  - name: step 1
    command: echo test
`
	err := os.WriteFile(tmpFile, []byte(input), 0644)
	require.NoError(t, err)

	ret, err := ReadFile(tmpFile)
	require.NoError(t, err)
	require.Equal(t, input, ret)
}

func TestConfigLoadHeadOnly(t *testing.T) {
	l := &Loader{}

	d, err := l.LoadHeadOnly(path.Join(testdataDir, "default.yaml"))
	require.NoError(t, err)

	require.Equal(t, d.Name, "default")
	require.True(t, len(d.Steps) == 0)
}

func TestLoadInvalidDAG(t *testing.T) {
	tests := []struct {
		input string
	}{
		{`env: 
  VAR: "` + "`ech 1`" + `"
`},
		{`logDir: "` + "`ech foo`" + `"`},
		{`params: "` + "`ech foo`" + `"`},
		{`schedule: "` + "1" + `"`},
	}

	for _, tt := range tests {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(tt.input))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.Error(t, err)
	}
}

func TestLoadEnv(t *testing.T) {
	tests := []struct {
		input, envKey, expectedValue string
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
	}

	for _, tt := range tests {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(tt.input))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		require.Equal(t, tt.expectedValue, os.Getenv(tt.envKey))
	}
}

func TestParseParameter(t *testing.T) {
	tests := []struct {
		params         string
		environ        string
		expectedParams map[string]string
	}{
		{
			params: "x",
			expectedParams: map[string]string{
				"1": "x",
			},
		},
		{
			params: "x y",
			expectedParams: map[string]string{
				"1": "x",
				"2": "y",
			},
		},
		{
			params: "x yy zzz",
			expectedParams: map[string]string{
				"1": "x",
				"2": "yy",
				"3": "zzz",
			},
		},
		{
			params: "x $1",
			expectedParams: map[string]string{
				"1": "x",
				"2": "x",
			},
		},
		{
			params:  "first P1=foo P2=${FOO} P3=`/bin/echo ${P2}` X=bar Y=${P1} Z=\"A B C\"",
			environ: "FOO: BAR",
			expectedParams: map[string]string{
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
	}

	for _, tt := range tests {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(fmt.Sprintf(`
env:
  - %s
params: %s
  	`, tt.environ, tt.params)))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		_, err = b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		for k, v := range tt.expectedParams {
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
	input := `
tags: Daily, Monthly
`
	expectedTags := []string{"daily", "monthly"}
	l := &Loader{}
	m, err := l.unmarshalData([]byte(input))
	require.NoError(t, err)

	def, err := l.decode(m)
	require.NoError(t, err)

	b := &builder{}
	d, err := b.buildFromDefinition(def, nil)
	require.NoError(t, err)

	require.Equal(t, expectedTags, d.Tags)

	require.True(t, d.HasTag("daily"))
	require.False(t, d.HasTag("weekly"))
}

func TestSchedule(t *testing.T) {
	tests := []struct {
		input       string
		err         bool
		expectedLen int
	}{
		{
			input:       "schedule: \"*/5 * * * *\"",
			expectedLen: 1,
		},
		{
			input: `schedule:
  - "*/5 * * * *"
  - "* * * * *"`,
			expectedLen: 2,
		},
		{
			input: `schedule:
  - true 
  - "* * * * *"`,
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := &Loader{}
			m, err := l.unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := l.decode(m)
			require.NoError(t, err)

			b := &builder{}
			d, err := b.buildFromDefinition(def, nil)

			if tt.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expectedLen, len(d.Schedule))
			}
		})
	}
}

func TestScheduleStop(t *testing.T) {
	tests := []struct {
		input              string
		err                bool
		expectedStartLen   int
		expectedStopLen    int
		expectedRestartLen int
	}{
		{
			input: `schedule:
  start: "0 1 * * *"
  stop: "0 2 * * *"
`,
			expectedStartLen: 1,
			expectedStopLen:  1,
		},
		{
			input: `schedule:
  start: "0 1 * * *"
`,
			expectedStartLen: 1,
			expectedStopLen:  0,
		},
		{
			input: `schedule:
  stop: "0 1 * * *"
`,
			expectedStartLen: 0,
			expectedStopLen:  1,
		},
		{
			input: `schedule:
  start: 
    - "0 1 * * *"
    - "0 18 * * *"
  stop:
    - "0 2 * * *"
    - "0 20 * * *"
`,
			expectedStartLen: 2,
			expectedStopLen:  2,
		},
		{
			input: `schedule:
  start: "0 8 * * *"
  restart: "0 12 * * *"
  stop: "0 20 * * *"
`,
			expectedStartLen:   1,
			expectedStopLen:    1,
			expectedRestartLen: 1,
		},
		{
			input: `schedule:
  stop: "* * * * * * *"
`,
			err: true,
		},
		{
			input: `schedule:
  invalid: "* * * * * * *"
`,
			err: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := &Loader{}
			m, err := l.unmarshalData([]byte(tt.input))
			require.NoError(t, err)

			def, err := l.decode(m)
			require.NoError(t, err)

			b := &builder{}
			d, err := b.buildFromDefinition(def, nil)

			if tt.err {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.expectedStartLen, len(d.Schedule))
			require.Equal(t, tt.expectedStopLen, len(d.StopSchedule))
			require.Equal(t, tt.expectedRestartLen, len(d.RestartSchedule))
		})
	}
}

func TestSockAddr(t *testing.T) {
	d := &DAG{Location: "testdata/testDag.yml"}
	require.Regexp(t, `^/tmp/@dagu-testDag-[0-9a-f]+\.sock$`, d.SockAddr())
}

func TestOverwriteGlobalConfig(t *testing.T) {
	l := &Loader{BaseConfig: settings.MustGet(settings.SETTING__BASE_CONFIG)}

	d, err := l.Load(path.Join(testdataDir, "overwrite.yaml"), "")
	require.NoError(t, err)

	require.Equal(t, &MailOn{Failure: false, Success: false}, d.MailOn)
	require.Equal(t, d.HistRetentionDays, 7)

	d, err = l.Load(path.Join(testdataDir, "no_overwrite.yaml"), "")
	require.NoError(t, err)

	require.Equal(t, &MailOn{Failure: true, Success: false}, d.MailOn)
	require.Equal(t, d.HistRetentionDays, 30)
}

func TestExecutor(t *testing.T) {
	tests := []struct {
		input, exepectedType, expectedConfig string
	}{
		{
			`
steps:
  - name: S1
    command: echo 1
    executor: http
`,
			"http",
			"",
		},
		{
			`
steps:
  - name: S1
    command: echo 1
    executor:
      type: http
      config: some option 
`,
			"http",
			"some option",
		},
	}

	for _, tt := range tests {
		l := &Loader{}
		d, err := l.unmarshalData([]byte(tt.input))
		require.NoError(t, err)

		def, err := l.decode(d)
		require.NoError(t, err)

		b := &builder{}
		dag, err := b.buildFromDefinition(def, nil)
		require.NoError(t, err)

		if len(dag.Steps) <= 0 {
			t.Fatal("no steps")
		}
		require.Equal(t, tt.exepectedType, dag.Steps[0].ExecutorConfig.Type)
		if tt.expectedConfig != "" {
			require.Equal(t, tt.expectedConfig, dag.Steps[0].ExecutorConfig.Config["config"])
		}
	}
}
