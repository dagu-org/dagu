package cmdutil

import (
	"context"
	"os"
	"reflect"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvalStringFields(t *testing.T) {
	// Set up test environment variables
	_ = os.Setenv("TEST_VAR", "test_value")
	_ = os.Setenv("NESTED_VAR", "nested_value")
	defer func() {
		_ = os.Unsetenv("TEST_VAR")
		_ = os.Unsetenv("NESTED_VAR")
	}()

	type Nested struct {
		NestedField   string
		NestedCommand string
		unexported    string
	}

	type TestStruct struct {
		SimpleField  string
		EnvField     string
		CommandField string
		MultiField   string
		EmptyField   string
		unexported   string
		NestedStruct Nested
	}

	tests := []struct {
		name    string
		input   TestStruct
		want    TestStruct
		wantErr bool
	}{
		{
			name: "basic substitution",
			input: TestStruct{
				SimpleField:  "hello",
				EnvField:     "$TEST_VAR",
				CommandField: "`echo hello`",
				MultiField:   "$TEST_VAR and `echo command`",
				EmptyField:   "",
				NestedStruct: Nested{
					NestedField:   "$NESTED_VAR",
					NestedCommand: "`echo nested`",
					unexported:    "should not change",
				},
				unexported: "should not change",
			},
			want: TestStruct{
				SimpleField:  "hello",
				EnvField:     "test_value",
				CommandField: "hello",
				MultiField:   "test_value and command",
				EmptyField:   "",
				NestedStruct: Nested{
					NestedField:   "nested_value",
					NestedCommand: "nested",
					unexported:    "should not change",
				},
				unexported: "should not change",
			},
			wantErr: false,
		},
		{
			name: "invalid command",
			input: TestStruct{
				CommandField: "`invalid_command_that_does_not_exist`",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalStringFields(ctx, tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("SubstituteStringFields() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SubstituteStringFields() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestEvalStringFields_AnonymousStruct(t *testing.T) {
	ctx := context.Background()
	obj, err := EvalStringFields(ctx, struct {
		Field string
	}{
		Field: "`echo hello`",
	})
	require.NoError(t, err)
	require.Equal(t, "hello", obj.Field)
}

func TestSubstituteStringFields_NonStruct(t *testing.T) {
	ctx := context.Background()
	_, err := EvalStringFields(ctx, "not a struct")
	if err == nil {
		t.Error("SubstituteStringFields() should return error for non-struct input")
	}
}

func TestEvalStringFields_NestedStructs(t *testing.T) {
	type DeepNested struct {
		Field string
	}

	type Nested struct {
		Field      string
		DeepNested DeepNested
	}

	type Root struct {
		Field  string
		Nested Nested
	}

	input := Root{
		Field: "$TEST_VAR",
		Nested: Nested{
			Field: "`echo nested`",
			DeepNested: DeepNested{
				Field: "$NESTED_VAR",
			},
		},
	}

	// Set up environment
	_ = os.Setenv("TEST_VAR", "test_value")
	_ = os.Setenv("NESTED_VAR", "deep_nested_value")
	defer func() {
		_ = os.Unsetenv("TEST_VAR")
		_ = os.Unsetenv("NESTED_VAR")
	}()

	want := Root{
		Field: "test_value",
		Nested: Nested{
			Field: "nested",
			DeepNested: DeepNested{
				Field: "deep_nested_value",
			},
		},
	}

	ctx := context.Background()
	got, err := EvalStringFields(ctx, input)
	if err != nil {
		t.Fatalf("SubstituteStringFields() error = %v", err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("SubstituteStringFields() = %+v, want %+v", got, want)
	}
}

func TestEvalStringFields_EmptyStruct(t *testing.T) {
	type Empty struct{}

	input := Empty{}
	ctx := context.Background()
	got, err := EvalStringFields(ctx, input)
	if err != nil {
		t.Fatalf("SubstituteStringFields() error = %v", err)
	}

	if !reflect.DeepEqual(got, input) {
		t.Errorf("SubstituteStringFields() = %+v, want %+v", got, input)
	}
}

func TestReplaceVars(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "basic substitution",
			template: "${FOO}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "short syntax",
			template: "$FOO",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "no substitution",
			template: "$FOO_",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "$FOO_",
		},
		{
			name:     "in middle of string",
			template: "prefix $FOO suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix BAR suffix",
		},
		{
			name:     "in middle of string and no substitution",
			template: "prefix $FOO1 suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix $FOO1 suffix",
		},
		{
			name:     "missing var",
			template: "${MISSING}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "${MISSING}",
		},
		{
			name:     "multiple vars",
			template: "$FOO ${BAR} $BAZ",
			vars: map[string]string{
				"FOO": "1",
				"BAR": "2",
				"BAZ": "3",
			},
			want: "1 2 3",
		},
		{
			name:     "nested vars not supported",
			template: "${FOO${BAR}}",
			vars:     map[string]string{"FOO": "1", "BAR": "2"},
			want:     "${FOO${BAR}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceVars(tt.template, tt.vars)
			if got != tt.want {
				t.Errorf("replaceVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestExpandReferences checks multiple scenarios using table-driven tests.
func TestExpandReferences(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		want    string
	}{
		{
			name:  "Basic replacement with curly braces",
			input: "Hello: ${FOO.bar}",
			dataMap: map[string]string{
				"FOO": `{"bar": "World"}`,
			},
			want: "Hello: World",
		},
		{
			name:  "Basic replacement with single dollar sign",
			input: "Output => $FOO.value",
			dataMap: map[string]string{
				"FOO": `{"value": "SingleDollarWorks"}`,
			},
			want: "Output => SingleDollarWorks",
		},
		{
			name:  "Missing key in dataMap",
			input: "Hello: ${BAR.xyz}",
			dataMap: map[string]string{
				// no "BAR" key
				"FOO": `{"bar":"zzz"}`,
			},
			// Because "BAR" does not exist in dataMap, no replacement
			want: "Hello: ${BAR.xyz}",
		},
		{
			name:  "Invalid JSON in dataMap",
			input: "Test => ${FOO.bar}",
			dataMap: map[string]string{
				"FOO": `{"bar":`, // invalid JSON
			},
			want: "Test => ${FOO.bar}",
		},
		{
			name:  "Nested sub-path extraction",
			input: "Deep => ${FOO.level1.level2}",
			dataMap: map[string]string{
				"FOO": `{"level1": {"level2":"DeepValue"}}`,
			},
			want: "Deep => DeepValue",
		},
		{
			name:  "Non-existent sub-path in valid JSON",
			input: "Data => ${FOO.bar.baz}",
			dataMap: map[string]string{
				"FOO": `{"bar":"NotAnObject"}`,
			},
			// "bar" is a string, so .bar.baz can't exist => original string remains
			want: "Data => ${FOO.bar.baz}",
		},
		{
			name:  "Multiple placeholders, including single-dollar form",
			input: "Multi: ${FOO.one}, $FOO.two , and ${FOO.three}",
			dataMap: map[string]string{
				"FOO": `{
									"one": "1",
									"two": "2",
									"three": "3"
							}`,
			},
			want: "Multi: 1, 2 , and 3",
		},
		{
			name:    "lookup from environment",
			input:   "${TEST_JSON_VAR.bar}",
			dataMap: map[string]string{},
			want:    "World",
		},
	}

	_ = os.Setenv("TEST_JSON_VAR", `{"bar": "World"}`)
	t.Cleanup(func() {
		_ = os.Unsetenv("TEST_JSON_VAR")
	})

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferences(ctx, tt.input, tt.dataMap)
			if got != tt.want {
				t.Errorf("ExpandReferences() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildCommandEscapedString(t *testing.T) {
	tests := []struct {
		name    string
		command string
		args    []string
		want    string
	}{
		{
			name:    "command with no args",
			command: "echo",
			args:    []string{},
			want:    "echo",
		},
		{
			name:    "command with simple args",
			command: "echo",
			args:    []string{"hello", "world"},
			want:    "echo hello world",
		},
		{
			name:    "args with spaces need quoting",
			command: "echo",
			args:    []string{"hello world", "foo bar"},
			want:    `echo "hello world" "foo bar"`,
		},
		{
			name:    "already quoted with double quotes",
			command: "echo",
			args:    []string{`"hello world"`, "test"},
			want:    `echo "hello world" test`,
		},
		{
			name:    "already quoted with single quotes",
			command: "echo",
			args:    []string{`'hello world'`, "test"},
			want:    `echo 'hello world' test`,
		},
		{
			name:    "key-value pair already quoted",
			command: "docker",
			args:    []string{"run", "-e", `VAR="value with spaces"`},
			want:    `docker run -e VAR="value with spaces"`,
		},
		{
			name:    "arg with double quotes inside",
			command: "echo",
			args:    []string{`hello "world" test`},
			want:    `echo "hello \"world\" test"`,
		},
		{
			name:    "mixed args",
			command: "command",
			args:    []string{"simple", "with space", `"already quoted"`, `key="value"`, "test=value"},
			want:    `command simple "with space" "already quoted" key="value" test=value`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildCommandEscapedString(tt.command, tt.args)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalString(t *testing.T) {
	// Set up test environment
	_ = os.Setenv("TEST_ENV", "test_value")
	_ = os.Setenv("TEST_JSON", `{"key": "value"}`)
	defer func() {
		_ = os.Unsetenv("TEST_ENV")
		_ = os.Unsetenv("TEST_JSON")
	}()

	tests := []struct {
		name    string
		input   string
		opts    []EvalOption
		want    string
		wantErr bool
	}{
		{
			name:    "empty string",
			input:   "",
			want:    "",
			wantErr: false,
		},
		{
			name:    "env var expansion",
			input:   "$TEST_ENV",
			want:    "test_value",
			wantErr: false,
		},
		{
			name:    "command substitution",
			input:   "`echo hello`",
			want:    "hello",
			wantErr: false,
		},
		{
			name:    "combined env and command",
			input:   "$TEST_ENV and `echo world`",
			want:    "test_value and world",
			wantErr: false,
		},
		{
			name:    "with variables",
			input:   "${FOO} and ${BAR}",
			opts:    []EvalOption{WithVariables(map[string]string{"FOO": "foo", "BAR": "bar"})},
			want:    "foo and bar",
			wantErr: false,
		},
		{
			name:    "without env expansion",
			input:   "$TEST_ENV",
			opts:    []EvalOption{WithoutExpandEnv()},
			want:    "$TEST_ENV",
			wantErr: false,
		},
		{
			name:    "without substitution",
			input:   "`echo hello`",
			opts:    []EvalOption{WithoutSubstitute()},
			want:    "`echo hello`",
			wantErr: false,
		},
		{
			name:    "only replace vars",
			input:   "$TEST_ENV and `echo hello` and ${FOO}",
			opts:    []EvalOption{OnlyReplaceVars(), WithVariables(map[string]string{"FOO": "foo"})},
			want:    "$TEST_ENV and `echo hello` and foo",
			wantErr: false,
		},
		{
			name:    "invalid command substitution",
			input:   "`invalid_command_that_does_not_exist`",
			want:    "",
			wantErr: true,
		},
		{
			name:    "JSON reference",
			input:   "${TEST_JSON.key}",
			opts:    []EvalOption{WithVariables(map[string]string{"TEST_JSON": os.Getenv("TEST_JSON")})},
			want:    "value",
			wantErr: false,
		},
		{
			name:  "multiple variable sets",
			input: "${FOO} ${BAR}",
			opts: []EvalOption{
				WithVariables(map[string]string{"FOO": "first"}),
				WithVariables(map[string]string{"BAR": "second"}),
			},
			want:    "first second",
			wantErr: false,
		},
		{
			name:    "quoted JSON variable escaping",
			input:   `params: aJson="${ITEM}"`,
			opts:    []EvalOption{WithVariables(map[string]string{"ITEM": `{"file": "file1.txt", "config": "prod"}`})},
			want:    `params: aJson=` + strconv.Quote(`{"file": "file1.txt", "config": "prod"}`),
			wantErr: false,
		},
		{
			name:    "quoted file path with spaces",
			input:   `path: "FILE=\"${ITEM}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"ITEM": "/path/to/my file.txt"})},
			want:    `path: "FILE=\"/path/to/my file.txt\""`,
			wantErr: false,
		},
		{
			name:    "quoted string with internal quotes",
			input:   `value: "VAR=\"${ITEM}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"ITEM": `say "hello"`})},
			want:    `value: "VAR=\"say "hello"\""`,
			wantErr: false,
		},
		{
			name:    "mixed quoted and unquoted variables",
			input:   `unquoted ${ITEM} and quoted "value=\"${ITEM}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"ITEM": `{"test": "value"}`})},
			want:    `unquoted {"test": "value"} and quoted "value=\"{"test": "value"}\""`,
			wantErr: false,
		},
		{
			name:    "quoted empty string",
			input:   `empty: "VAL=\"${EMPTY}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"EMPTY": ""})},
			want:    `empty: "VAL=\"\""`,
			wantErr: false,
		},
		{
			name:    "quoted JSON path reference",
			input:   `config: "file=\"${CONFIG.file}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"CONFIG": `{"file": "/path/to/config.json", "env": "prod"}`})},
			want:    `config: "file=\"/path/to/config.json\""`,
			wantErr: false,
		},
		{
			name:    "quoted JSON path with spaces",
			input:   `path: "value=\"${DATA.path}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"DATA": `{"path": "/my dir/file name.txt"}`})},
			want:    `path: "value=\"/my dir/file name.txt\""`,
			wantErr: false,
		},
		{
			name:    "quoted nested JSON path",
			input:   `nested: "result=\"${OBJ.nested.deep}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"OBJ": `{"nested": {"deep": "found it"}}`})},
			want:    `nested: "result=\"found it\""`,
			wantErr: false,
		},
		{
			name:    "quoted JSON path with quotes in value",
			input:   `msg: "text=\"${MSG.content}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"MSG": `{"content": "He said \"hello\""}`})},
			want:    `msg: "text=\"He said "hello"\""`,
			wantErr: false,
		},
		{
			name:  "mixed quoted JSON path and simple variable",
			input: `params: "${SIMPLE}" and config="file=\"${CONFIG.file}\""`,
			opts: []EvalOption{WithVariables(map[string]string{
				"SIMPLE": "value",
				"CONFIG": `{"file": "app.conf"}`,
			})},
			want:    `params: "value" and config="file=\"app.conf\""`,
			wantErr: false,
		},
		{
			name:    "quoted non-existent JSON path",
			input:   `missing: "val=\"${CONFIG.missing}\""`,
			opts:    []EvalOption{WithVariables(map[string]string{"CONFIG": `{"file": "app.conf"}`})},
			want:    `missing: "val=\"<nil>\""`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEvalIntString(t *testing.T) {
	// Set up test environment
	_ = os.Setenv("TEST_INT", "42")
	defer func() {
		_ = os.Unsetenv("TEST_INT")
	}()

	tests := []struct {
		name    string
		input   string
		opts    []EvalOption
		want    int
		wantErr bool
	}{
		{
			name:    "simple integer",
			input:   "123",
			want:    123,
			wantErr: false,
		},
		{
			name:    "env var integer",
			input:   "$TEST_INT",
			want:    42,
			wantErr: false,
		},
		{
			name:    "command substitution integer",
			input:   "`echo 100`",
			want:    100,
			wantErr: false,
		},
		{
			name:    "with variables",
			input:   "${NUM}",
			opts:    []EvalOption{WithVariables(map[string]string{"NUM": "999"})},
			want:    999,
			wantErr: false,
		},
		{
			name:    "invalid integer",
			input:   "not_a_number",
			want:    0,
			wantErr: true,
		},
		{
			name:    "invalid command",
			input:   "`invalid_command`",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalIntString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEvalStringFields_Map(t *testing.T) {
	// Set up test environment
	_ = os.Setenv("MAP_ENV", "map_value")
	defer func() {
		_ = os.Unsetenv("MAP_ENV")
	}()

	tests := []struct {
		name    string
		input   map[string]any
		opts    []EvalOption
		want    map[string]any
		wantErr bool
	}{
		{
			name: "simple map with string values",
			input: map[string]any{
				"key1": "$MAP_ENV",
				"key2": "`echo hello`",
				"key3": "plain",
			},
			want: map[string]any{
				"key1": "map_value",
				"key2": "hello",
				"key3": "plain",
			},
			wantErr: false,
		},
		{
			name: "nested map",
			input: map[string]any{
				"outer": map[string]any{
					"inner": "$MAP_ENV",
				},
			},
			want: map[string]any{
				"outer": map[string]any{
					"inner": "map_value",
				},
			},
			wantErr: false,
		},
		{
			name: "map with non-string values",
			input: map[string]any{
				"string": "$MAP_ENV",
				"int":    42,
				"bool":   true,
				"nil":    nil,
			},
			want: map[string]any{
				"string": "map_value",
				"int":    42,
				"bool":   true,
				"nil":    nil,
			},
			wantErr: false,
		},
		{
			name: "map with struct value",
			input: map[string]any{
				"struct": struct {
					Field string
				}{
					Field: "$MAP_ENV",
				},
			},
			want: map[string]any{
				"struct": struct {
					Field string
				}{
					Field: "map_value",
				},
			},
			wantErr: false,
		},
		{
			name: "with variables option",
			input: map[string]any{
				"key": "${VAR}",
			},
			opts: []EvalOption{WithVariables(map[string]string{"VAR": "value"})},
			want: map[string]any{
				"key": "value",
			},
			wantErr: false,
		},
		{
			name: "map with pointer values",
			input: map[string]any{
				"ptr": ptrString("$MAP_ENV"),
			},
			want: map[string]any{
				"ptr": "map_value",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalStringFields(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExpandReferencesWithSteps_Extended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		stepMap map[string]StepInfo
		want    string
	}{
		{
			name:    "step stdout reference",
			input:   "The output is at ${step1.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "/tmp/step1.out",
				},
			},
			want: "The output is at /tmp/step1.out",
		},
		{
			name:    "step stderr reference",
			input:   "Errors at ${step1.stderr}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stderr: "/tmp/step1.err",
				},
			},
			want: "Errors at /tmp/step1.err",
		},
		{
			name:    "step exit code reference",
			input:   "Exit code: ${step1.exit_code}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					ExitCode: "0",
				},
			},
			want: "Exit code: 0",
		},
		{
			name:    "missing step reference",
			input:   "Missing: ${missing_step.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "/tmp/step1.out",
				},
			},
			want: "Missing: ${missing_step.stdout}",
		},
		{
			name:    "empty step property",
			input:   "Empty: ${step1.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "",
				},
			},
			want: "Empty: ${step1.stdout}",
		},
		{
			name:    "nil stepMap",
			input:   "No steps: ${step1.stdout}",
			dataMap: map[string]string{},
			stepMap: nil,
			want:    "No steps: ${step1.stdout}",
		},
		{
			name:  "regular variable takes precedence",
			input: "Value: ${step1.field}",
			dataMap: map[string]string{
				"step1": `{"field": "from_var"}`,
			},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "/tmp/step1.out",
				},
			},
			want: "Value: from_var",
		},
		{
			name:    "dollar sign without braces",
			input:   "Path: $step1.stdout",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "/tmp/out",
				},
			},
			want: "Path: /tmp/out",
		},
		{
			name:    "multiple step references",
			input:   "Out: ${step1.stdout}, Err: ${step1.stderr}, Code: ${step1.exit_code}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout:   "/tmp/out",
					Stderr:   "/tmp/err",
					ExitCode: "1",
				},
			},
			want: "Out: /tmp/out, Err: /tmp/err, Code: 1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferencesWithSteps(ctx, tt.input, tt.dataMap, tt.stepMap)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalString_WithStepMap(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []EvalOption
		want    string
		wantErr bool
	}{
		{
			name:  "step reference with no variables",
			input: "Output: ${step1.stdout}",
			opts: []EvalOption{
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
			name:  "step reference with variables",
			input: "Var: ${VAR}, Step: ${step1.exit_code}",
			opts: []EvalOption{
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got, err := EvalString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestEvalIntString_WithStepMap(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		opts    []EvalOption
		want    int
		wantErr bool
	}{
		{
			name:  "step exit code as integer",
			input: "${step1.exit_code}",
			opts: []EvalOption{
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
			got, err := EvalIntString(ctx, tt.input, tt.opts...)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestProcessStructFields_WithStepMap(t *testing.T) {
	type TestStruct struct {
		StepOutput string
		StepError  string
	}

	input := TestStruct{
		StepOutput: "${step1.stdout}",
		StepError:  "${step1.stderr}",
	}

	ctx := context.Background()
	got, err := EvalStringFields(ctx, input,
		WithStepMap(map[string]StepInfo{
			"step1": {
				Stdout: "/tmp/out.txt",
				Stderr: "/tmp/err.txt",
			},
		}),
	)

	require.NoError(t, err)
	assert.Equal(t, "/tmp/out.txt", got.StepOutput)
	assert.Equal(t, "/tmp/err.txt", got.StepError)
}

func TestProcessMap_WithStepMap(t *testing.T) {
	input := map[string]any{
		"output": "${step1.stdout}",
		"nested": map[string]any{
			"exit_code": "${step1.exit_code}",
		},
	}

	ctx := context.Background()
	got, err := EvalStringFields(ctx, input,
		WithStepMap(map[string]StepInfo{
			"step1": {
				Stdout:   "/tmp/output",
				ExitCode: "0",
			},
		}),
	)

	require.NoError(t, err)
	assert.Equal(t, "/tmp/output", got["output"])
	nested, ok := got["nested"].(map[string]any)
	require.True(t, ok, "Expected nested to be map[string]any, got %T", got["nested"])
	assert.Equal(t, "0", nested["exit_code"])
}

func TestReplaceVars_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "single quotes preserved",
			template: "'$FOO'",
			vars:     map[string]string{"FOO": "bar"},
			want:     "'$FOO'",
		},
		{
			name:     "single quotes preserved with braces",
			template: "'${FOO}'",
			vars:     map[string]string{"FOO": "bar"},
			want:     "'${FOO}'",
		},
		{
			name:     "empty variable name",
			template: "${}",
			vars:     map[string]string{"": "value"},
			want:     "${}",
		},
		{
			name:     "underscore in var name",
			template: "${FOO_BAR}",
			vars:     map[string]string{"FOO_BAR": "value"},
			want:     "value",
		},
		{
			name:     "number in var name",
			template: "${FOO123}",
			vars:     map[string]string{"FOO123": "value"},
			want:     "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := replaceVars(tt.template, tt.vars)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExpandReferencesWithSteps(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		stepMap map[string]StepInfo
		want    string
	}{
		{
			name:    "Basic step ID stdout reference",
			input:   "Log file is at ${download.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout:   "/tmp/logs/download.out",
					Stderr:   "/tmp/logs/download.err",
					ExitCode: "0",
				},
			},
			want: "Log file is at /tmp/logs/download.out",
		},
		{
			name:    "Step ID stderr reference",
			input:   "Check errors at ${build.stderr}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"build": {
					Stdout:   "/tmp/logs/build.out",
					Stderr:   "/tmp/logs/build.err",
					ExitCode: "1",
				},
			},
			want: "Check errors at /tmp/logs/build.err",
		},
		{
			name:    "Step ID exit code reference",
			input:   "Build exited with code ${build.exit_code}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"build": {
					Stdout:   "/tmp/logs/build.out",
					Stderr:   "/tmp/logs/build.err",
					ExitCode: "1",
				},
			},
			want: "Build exited with code 1",
		},
		{
			name:    "Multiple step references",
			input:   "Download log: ${download.stdout}, Build errors: ${build.stderr}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
				"build": {
					Stderr: "/tmp/logs/build.err",
				},
			},
			want: "Download log: /tmp/logs/download.out, Build errors: /tmp/logs/build.err",
		},
		{
			name:    "Unknown step ID leaves as-is",
			input:   "Unknown step: ${unknown.stdout}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"known": {
					Stdout: "/tmp/logs/known.out",
				},
			},
			want: "Unknown step: ${unknown.stdout}",
		},
		{
			name:    "Unknown property leaves as-is",
			input:   "Unknown prop: ${download.unknown}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
			},
			want: "Unknown prop: ${download.unknown}",
		},
		{
			name:  "Regular variable takes precedence over step ID",
			input: "Value: ${download.stdout}",
			dataMap: map[string]string{
				"download": `{"stdout": "from-variable"}`,
			},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
			},
			want: "Value: from-variable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			got := ExpandReferencesWithSteps(ctx, tt.input, tt.dataMap, tt.stepMap)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvalStringWithSteps(t *testing.T) {
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
			name:  "stdout reference",
			input: "cat ${download.stdout}",
			want:  "cat /var/log/download.stdout",
		},
		{
			name:  "stderr reference",
			input: "tail -20 ${process.stderr}",
			want:  "tail -20 /var/log/process.stderr",
		},
		{
			name:  "exit code reference",
			input: "if [ ${process.exit_code} -ne 0 ]; then echo failed; fi",
			want:  "if [ 1 -ne 0 ]; then echo failed; fi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvalString(ctx, tt.input, WithStepMap(stepMap))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestEvalStringFields_MultipleVariablesWithStepMapOnLast tests the specific case
// where we have multiple variable sets and StepMap is applied only with the last set
func TestEvalStringFields_MultipleVariablesWithStepMapOnLast(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 string
		Field3 string
		Field4 string
	}

	stepMap := map[string]StepInfo{
		"build": {
			Stdout:   "/logs/build.out",
			Stderr:   "/logs/build.err",
			ExitCode: "0",
		},
		"test": {
			Stdout: "/logs/test.out",
		},
	}

	tests := []struct {
		name     string
		input    TestStruct
		varSets  []map[string]string
		expected TestStruct
	}{
		{
			name: "three variable sets with step references",
			input: TestStruct{
				Field1: "${A}",
				Field2: "${B}",
				Field3: "${C}",
				Field4: "${build.stderr}",
			},
			varSets: []map[string]string{
				{"A": "alpha"},
				{"B": "beta"},
				{"C": "gamma"},
			},
			expected: TestStruct{
				Field1: "alpha",
				Field2: "beta",
				Field3: "gamma",
				Field4: "/logs/build.err",
			},
		},
		{
			name: "step references only on last variable set",
			input: TestStruct{
				Field1: "${VAR1}",
				Field2: "${VAR2}",
				Field3: "${test.stdout}",
				Field4: "${VAR3}",
			},
			varSets: []map[string]string{
				{"VAR1": "first"},
				{"VAR2": "second"},
				{"VAR3": "third"},
			},
			expected: TestStruct{
				Field1: "first",
				Field2: "second",
				Field3: "/logs/test.out",
				Field4: "third",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Build options with multiple variable sets
			opts := []EvalOption{}
			for _, vars := range tt.varSets {
				opts = append(opts, WithVariables(vars))
			}
			// Add StepMap as the last option
			opts = append(opts, WithStepMap(stepMap))

			result, err := EvalStringFields(ctx, tt.input, opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestEvalString_MultipleVariablesWithStepMapOnLast tests EvalString with multiple variable sets
func TestEvalString_MultipleVariablesWithStepMapOnLast(t *testing.T) {
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
			name:  "step references processed with last variable set",
			input: "${X} and ${Y} with log at ${deploy.stdout}",
			varSets: []map[string]string{
				{"X": "1", "Y": "2"},
				{"Z": "3"}, // Different variable, X and Y should remain from first set
			},
			expected: "1 and 2 with log at /logs/deploy.out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Build options with multiple variable sets
			opts := []EvalOption{}
			for _, vars := range tt.varSets {
				opts = append(opts, WithVariables(vars))
			}
			// Add StepMap
			opts = append(opts, WithStepMap(stepMap))

			result, err := EvalString(ctx, tt.input, opts...)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestExpandReferencesWithSteps_SearchAcrossOutputs tests the specific case where
// a field is not directly in outputs but needs to be found by parsing JSON in each output
func TestExpandReferencesWithSteps_SearchAcrossOutputs(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		input   string
		stepMap map[string]StepInfo
		want    string
	}{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandReferencesWithSteps(ctx, tt.input, map[string]string{}, tt.stepMap)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Helper function to create string pointer
func ptrString(s string) *string {
	return &s
}
