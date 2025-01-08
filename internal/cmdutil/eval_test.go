package cmdutil

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEvalStringFields(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("NESTED_VAR", "nested_value")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("NESTED_VAR")

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
	os.Setenv("TEST_VAR", "test_value")
	os.Setenv("NESTED_VAR", "deep_nested_value")
	defer os.Unsetenv("TEST_VAR")
	defer os.Unsetenv("NESTED_VAR")

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

	os.Setenv("TEST_JSON_VAR", `{"bar": "World"}`)
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
