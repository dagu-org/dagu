package eval

import (
	"context"
	"os"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- expandQuotedRefs coverage ---

func TestExpandQuotedRefs_SimpleVariable(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	WithVariables(map[string]string{"VAR": "hello"})(opts)

	result, err := expandQuotedRefs(ctx, `{"key": "${VAR}"}`, opts)
	require.NoError(t, err)
	assert.Equal(t, `{"key": "hello"}`, result)
}

func TestExpandQuotedRefs_JSONPathRef(t *testing.T) {
	ctx := context.Background()
	vars := map[string]string{"DATA": `{"name":"alice"}`}
	opts := NewOptions()
	WithVariables(vars)(opts)

	result, err := expandQuotedRefs(ctx, `{"val": "${DATA.name}"}`, opts)
	require.NoError(t, err)
	assert.Equal(t, `{"val": "alice"}`, result)
}

func TestExpandQuotedRefs_NotFound(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := expandQuotedRefs(ctx, `{"val": "${MISSING}"}`, opts)
	require.NoError(t, err)
	assert.Equal(t, `{"val": "${MISSING}"}`, result)
}

func TestExpandQuotedRefs_JSONPathNotFound(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := expandQuotedRefs(ctx, `{"val": "${MISSING.path}"}`, opts)
	require.NoError(t, err)
	assert.Equal(t, `{"val": "${MISSING.path}"}`, result)
}

func TestExpandQuotedRefs_NoMatch(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := expandQuotedRefs(ctx, `no refs here`, opts)
	require.NoError(t, err)
	assert.Equal(t, `no refs here`, result)
}

func TestExpandQuotedRefs_WithStepRef(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	WithStepMap(map[string]StepInfo{
		"step1": {Stdout: "output_val", ExitCode: "0"},
	})(opts)

	result, err := expandQuotedRefs(ctx, `{"out": "${step1.stdout}"}`, opts)
	require.NoError(t, err)
	assert.Equal(t, `{"out": "output_val"}`, result)
}

// --- shellExpandPhase coverage ---

func TestShellExpandPhase_FallbackOnError(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	t.Setenv("TESTVAR", "value123")

	result, err := shellExpandPhase(ctx, "$(echo hello) $TESTVAR", opts)
	require.NoError(t, err)
	assert.Contains(t, result, "value123")
}

func TestShellExpandPhase_NonCommandError(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := shellExpandPhase(ctx, "${UNSET_XYZ_99:?required}", opts)
	require.NoError(t, err)
	assert.Contains(t, result, "UNSET_XYZ_99")
}

// --- Pipeline execute with disabled phase ---

func TestPipeline_DisabledPhases(t *testing.T) {
	ctx := context.Background()
	t.Setenv("PVAR", "pval")

	result, err := String(ctx, "`echo nope` $PVAR",
		WithoutSubstitute(),
		WithoutExpandEnv(),
		WithVariables(map[string]string{"X": "y"}),
	)
	require.NoError(t, err)
	assert.Contains(t, result, "`echo nope`")
	assert.Contains(t, result, "$PVAR")
}

// --- String with ExpandEnv but expandWithShellContext fails ---

func TestString_ShellExpandFallback(t *testing.T) {
	t.Setenv("FBVAR", "fbval")
	ctx := context.Background()

	result, err := String(ctx, "$(echo x) $FBVAR")
	require.NoError(t, err)
	assert.Contains(t, result, "fbval")
}

// --- String / IntString pipeline tests ---

func TestString(t *testing.T) {
	_ = os.Setenv("TEST_ENV", "test_value")
	_ = os.Setenv("TEST_JSON", `{"key": "value"}`)
	defer func() {
		_ = os.Unsetenv("TEST_ENV")
		_ = os.Unsetenv("TEST_JSON")
	}()

	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    string
		wantErr bool
	}{
		{
			name:    "EmptyString",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "EnvVarExpansion",
			input:   "$TEST_ENV",
			want:    "test_value",
			wantErr: false,
		},
		{
			name:    "CommandSubstitution",
			input:   "`echo hello`",
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "CombinedEnvAndCommand",
			input:   "$TEST_ENV and `echo world`",
			want:    "test_value and world",
			wantErr: false,
		},
		{
			name:    "WithVariables",
			input:   "${FOO} and ${BAR}",
			opts:    []Option{WithVariables(map[string]string{"FOO": "foo", "BAR": "bar"})},
			want:    "foo and bar",
			wantErr: false,
		},
		{
			name:    "WithoutEnvExpansion",
			input:   "$TEST_ENV",
			opts:    []Option{WithoutExpandEnv()},
			want:    "$TEST_ENV",
			wantErr: false,
		},
		{
			name:    "WithoutSubstitution",
			input:   "`echo hello`",
			opts:    []Option{WithoutSubstitute()},
			want:    "`echo hello`",
			wantErr: false,
		},
		{
			name:    "ShellSubstringExpansion",
			input:   "prefix ${UID:0:5} suffix",
			opts:    []Option{WithVariables(map[string]string{"UID": "HBL01_22OCT2025_0536"})},
			want:    "prefix HBL01 suffix",
			wantErr: false,
		},
		{
			name:    "OnlyReplaceVars",
			input:   "$TEST_ENV and `echo hello` and ${FOO}",
			opts:    []Option{OnlyReplaceVars(), WithVariables(map[string]string{"FOO": "foo"})},
			want:    "$TEST_ENV and `echo hello` and foo",
			wantErr: false,
		},
		{
			name:    "InvalidCommandSubstitution",
			input:   "`invalid_command_that_does_not_exist`",
			want:    "",
			wantErr: true,
		},
		{
			name:    "JSONReference",
			input:   "${TEST_JSON.key}",
			opts:    []Option{WithVariables(map[string]string{"TEST_JSON": os.Getenv("TEST_JSON")})},
			want:    "value",
			wantErr: false,
		},
		{
			name:  "MultipleVariableSets",
			input: "${FOO} ${BAR}",
			opts: []Option{
				WithVariables(map[string]string{"FOO": "first"}),
				WithVariables(map[string]string{"BAR": "second"}),
			},
			want:    "first second",
			wantErr: false,
		},
		{
			name:    "QuotedJSONVariableEscaping",
			input:   `params: aJson="${ITEM}"`,
			opts:    []Option{WithVariables(map[string]string{"ITEM": `{"file": "file1.txt", "config": "prod"}`})},
			want:    `params: aJson=` + strconv.Quote(`{"file": "file1.txt", "config": "prod"}`),
			wantErr: false,
		},
		{
			name:    "QuotedFilePathWithSpaces",
			input:   `path: "FILE=\"${ITEM}\""`,
			opts:    []Option{WithVariables(map[string]string{"ITEM": "/path/to/my file.txt"})},
			want:    `path: "FILE=\"/path/to/my file.txt\""`,
			wantErr: false,
		},
		{
			name:    "QuotedStringWithInternalQuotes",
			input:   `value: "VAR=\"${ITEM}\""`,
			opts:    []Option{WithVariables(map[string]string{"ITEM": `say "hello"`})},
			want:    `value: "VAR=\"say "hello"\""`,
			wantErr: false,
		},
		{
			name:    "MixedQuotedAndUnquotedVariables",
			input:   `unquoted ${ITEM} and quoted "value=\"${ITEM}\""`,
			opts:    []Option{WithVariables(map[string]string{"ITEM": `{"test": "value"}`})},
			want:    `unquoted {"test": "value"} and quoted "value=\"{"test": "value"}\""`,
			wantErr: false,
		},
		{
			name:    "QuotedEmptyString",
			input:   `empty: "VAL=\"${EMPTY}\""`,
			opts:    []Option{WithVariables(map[string]string{"EMPTY": ""})},
			want:    `empty: "VAL=\"\""`,
			wantErr: false,
		},
		{
			name:    "QuotedJSONPathReference",
			input:   `config: "file=\"${CONFIG.file}\""`,
			opts:    []Option{WithVariables(map[string]string{"CONFIG": `{"file": "/path/to/config.json", "env": "prod"}`})},
			want:    `config: "file=\"/path/to/config.json\""`,
			wantErr: false,
		},
		{
			name:    "QuotedJSONPathWithSpaces",
			input:   `path: "value=\"${DATA.path}\""`,
			opts:    []Option{WithVariables(map[string]string{"DATA": `{"path": "/my dir/file name.txt"}`})},
			want:    `path: "value=\"/my dir/file name.txt\""`,
			wantErr: false,
		},
		{
			name:    "QuotedNestedJSONPath",
			input:   `nested: "result=\"${OBJ.nested.deep}\""`,
			opts:    []Option{WithVariables(map[string]string{"OBJ": `{"nested": {"deep": "found it"}}`})},
			want:    `nested: "result=\"found it\""`,
			wantErr: false,
		},
		{
			name:    "QuotedJSONPathWithQuotesInValue",
			input:   `msg: "text=\"${MSG.content}\""`,
			opts:    []Option{WithVariables(map[string]string{"MSG": `{"content": "He said \"hello\""}`})},
			want:    `msg: "text=\"He said "hello"\""`,
			wantErr: false,
		},
		{
			name:  "MixedQuotedJSONPathAndSimpleVariable",
			input: `params: "${SIMPLE}" and config="file=\"${CONFIG.file}\""`,
			opts: []Option{WithVariables(map[string]string{
				"SIMPLE": "value",
				"CONFIG": `{"file": "app.conf"}`,
			})},
			want:    `params: "value" and config="file=\"app.conf\""`,
			wantErr: false,
		},
		{
			name:    "QuotedNonExistentJSONPath",
			input:   `missing: "val=\"${CONFIG.missing}\""`,
			opts:    []Option{WithVariables(map[string]string{"CONFIG": `{"file": "app.conf"}`})},
			want:    `missing: "val=\"<nil>\""`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := String(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIntString(t *testing.T) {
	_ = os.Setenv("TEST_INT", "42")
	defer func() {
		_ = os.Unsetenv("TEST_INT")
	}()

	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    int
		wantErr bool
	}{
		{
			name:    "SimpleInteger",
			input:   "123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "EnvVarInteger",
			input:   "$TEST_INT",
			want:    42,
			wantErr: false,
		},
		{
			name:    "CommandSubstitutionInteger",
			input:   "`echo 100`",
			want:    100,
			wantErr: false,
		},
		{
			name:    "WithVariables",
			input:   "${NUM}",
			opts:    []Option{WithVariables(map[string]string{"NUM": "999"})},
			want:    999,
			wantErr: false,
		},
		{
			name:    "InvalidInteger",
			input:   "not_a_number",
			want:    0,
			wantErr: true,
		},
		{
			name:    "InvalidCommand",
			input:   "`invalid_command`",
			want:    0,
			wantErr: true,
		},
		{
			name:    "WithoutSubstitute_SkipsCommandSubstitution",
			input:   "123",
			opts:    []Option{WithoutSubstitute()},
			want:    123,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := IntString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestString_WithStepMap(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    string
		wantErr bool
	}{
		{
			name:  "StepReferenceWithNoVariables",
			input: "Output: ${step1.stdout}",
			opts: []Option{
				WithStepMap(map[string]StepInfo{
					"step1": {
						Stdout: "/tmp/output.txt",
					},
				}),
			},
			want:    "Output: /tmp/output.txt",
			wantErr: false,
		},
		{
			name:  "StepReferenceWithVariables",
			input: "Var: ${VAR}, Step: ${step1.exit_code}",
			opts: []Option{
				WithVariables(map[string]string{"VAR": "value"}),
				WithStepMap(map[string]StepInfo{
					"step1": {
						ExitCode: "0",
					},
				}),
			},
			want:    "Var: value, Step: 0",
			wantErr: false,
		},
		{
			name:  "StepStdoutSlice",
			input: "Slice: ${step1.stdout:0:3}",
			opts: []Option{
				WithStepMap(map[string]StepInfo{
					"step1": {
						Stdout: "HBL01_22OCT2025_0536",
					},
				}),
			},
			want:    "Slice: HBL",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := String(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestIntString_WithStepMap(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []Option
		want    int
		wantErr bool
	}{
		{
			name:  "StepExitCodeAsInteger",
			input: "${step1.exit_code}",
			opts: []Option{
				WithStepMap(map[string]StepInfo{
					"step1": {
						ExitCode: "42",
					},
				}),
			},
			want:    42,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := IntString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestStringWithSteps(t *testing.T) {
	ctx := context.Background()

	stepMap := map[string]StepInfo{
		"download": {
			Stdout:   "/var/log/download.stdout",
			Stderr:   "/var/log/download.stderr",
			ExitCode: "0",
		},
		"process": {
			Stdout:   "/var/log/process.stdout",
			Stderr:   "/var/log/process.stderr",
			ExitCode: "1",
		},
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "StdoutReference",
			input: "cat ${download.stdout}",
			want:  "cat /var/log/download.stdout",
		},
		{
			name:  "StderrReference",
			input: "tail -20 ${process.stderr}",
			want:  "tail -20 /var/log/process.stderr",
		},
		{
			name:  "ExitCodeReference",
			input: "if [ ${process.exit_code} -ne 0 ]; then echo failed; fi",
			want:  "if [ 1 -ne 0 ]; then echo failed; fi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := String(ctx, tt.input, WithStepMap(stepMap))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestString_MultipleVariablesWithStepMapOnLast(t *testing.T) {
	ctx := context.Background()

	stepMap := map[string]StepInfo{
		"deploy": {
			Stdout: "/logs/deploy.out",
		},
	}

	tests := []struct {
		name     string
		input    string
		varSets  []map[string]string
		expected string
	}{
		{
			name:  "StepReferencesProcessedWithLastVariableSet",
			input: "${X} and ${Y} with log at ${deploy.stdout}",
			varSets: []map[string]string{
				{"X": "1", "Y": "2"},
				{"Z": "3"},
			},
			expected: "1 and 2 with log at /logs/deploy.out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := []Option{}
			for _, vars := range tt.varSets {
				opts = append(opts, WithVariables(vars))
			}
			opts = append(opts, WithStepMap(stepMap))

			result, err := String(ctx, tt.input, opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
