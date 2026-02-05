package eval

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- resolveForShell coverage ---

func TestResolveForShell_FromScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("SCOPEVAR", "scopeval", EnvSourceDAGEnv)
	r := &resolver{scope: scope}

	val, ok := r.resolveForShell("SCOPEVAR")
	assert.True(t, ok)
	assert.Equal(t, "scopeval", val)
}

func TestResolveForShell_SkipsOSScope(t *testing.T) {
	t.Setenv("OSVAR", "live_os_value")
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("OSVAR", "frozen_value", EnvSourceOS)
	r := &resolver{scope: scope, expandOS: true}

	val, ok := r.resolveForShell("OSVAR")
	assert.True(t, ok)
	assert.Equal(t, "live_os_value", val)
}

func TestResolveForShell_OSEnvFallback(t *testing.T) {
	t.Setenv("TESTOSVAR", "osval")
	r := &resolver{expandOS: true}

	val, ok := r.resolveForShell("TESTOSVAR")
	assert.True(t, ok)
	assert.Equal(t, "osval", val)
}

func TestResolveForShell_NotFound(t *testing.T) {
	r := &resolver{}
	_, ok := r.resolveForShell("DEFINITELY_NOT_SET_EVER_12345")
	assert.False(t, ok)
}

// --- resolveJSONSource coverage ---

func TestResolveJSONSource_FromScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("JSONVAR", `{"a":1}`, EnvSourceDAGEnv)
	r := &resolver{scope: scope}

	val, ok := r.resolveJSONSource("JSONVAR")
	assert.True(t, ok)
	assert.Equal(t, `{"a":1}`, val)
}

func TestResolveJSONSource_FromScopeWithExpandOS(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("JSONVAR", `{"a":1}`, EnvSourceDAGEnv)
	r := &resolver{scope: scope, expandOS: true}

	val, ok := r.resolveJSONSource("JSONVAR")
	assert.True(t, ok)
	assert.Equal(t, `{"a":1}`, val)
}

func TestResolveJSONSource_FromOSEnv(t *testing.T) {
	t.Setenv("JSONOSVAR", `{"b":2}`)
	r := &resolver{expandOS: true}

	val, ok := r.resolveJSONSource("JSONOSVAR")
	assert.True(t, ok)
	assert.Equal(t, `{"b":2}`, val)
}

func TestResolveJSONSource_NotFound(t *testing.T) {
	r := &resolver{}
	_, ok := r.resolveJSONSource("NOPE_NOT_HERE_12345")
	assert.False(t, ok)
}

// --- resolver.resolve from scope ---

func TestResolver_Resolve_FromScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("SCOPEVAR", "val", EnvSourceDAGEnv)
	r := &resolver{scope: scope}

	val, ok := r.resolve("SCOPEVAR")
	assert.True(t, ok)
	assert.Equal(t, "val", val)
}

func TestResolver_Resolve_SkipsOSSource(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("OSONLY", "frozen", EnvSourceOS)
	r := &resolver{scope: scope}

	_, ok := r.resolve("OSONLY")
	assert.False(t, ok)
}

// --- resolveReference coverage ---

func TestResolveReference_JSONFromScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("JDATA", `{"x":"y"}`, EnvSourceDAGEnv)
	r := &resolver{scope: scope}

	val, ok := r.resolveReference(context.Background(), "JDATA", ".x")
	assert.True(t, ok)
	assert.Equal(t, "y", val)
}

func TestResolveReference_JSONFromOSEnv(t *testing.T) {
	t.Setenv("OSJSON", `{"a":"b"}`)
	r := &resolver{expandOS: true}

	val, ok := r.resolveReference(context.Background(), "OSJSON", ".a")
	assert.True(t, ok)
	assert.Equal(t, "b", val)
}

func TestResolveReference_NotFound(t *testing.T) {
	r := &resolver{}
	_, ok := r.resolveReference(context.Background(), "NOPE12345", ".x")
	assert.False(t, ok)
}

// --- ExpandOS resolver tests ---

func TestResolveForShell_WithoutExpandOS(t *testing.T) {
	t.Setenv("SHELL_TEST_VAR", "os_value")
	r := &resolver{expandOS: false}

	_, ok := r.resolveForShell("SHELL_TEST_VAR")
	assert.False(t, ok, "should not find OS var when expandOS=false")
}

func TestResolveForShell_WithExpandOS(t *testing.T) {
	t.Setenv("SHELL_TEST_VAR", "os_value")
	r := &resolver{expandOS: true}

	val, ok := r.resolveForShell("SHELL_TEST_VAR")
	assert.True(t, ok)
	assert.Equal(t, "os_value", val)
}

func TestResolveJSONSource_WithoutExpandOS(t *testing.T) {
	t.Setenv("JSON_OS_VAR", `{"a":1}`)

	// OS env should not be found
	r := &resolver{expandOS: false}
	_, ok := r.resolveJSONSource("JSON_OS_VAR")
	assert.False(t, ok)

	// OS-sourced scope entries should not be found
	scope := NewEnvScope(nil, false).
		WithEntry("SCOPE_OS", `{"b":2}`, EnvSourceOS)
	r2 := &resolver{scope: scope, expandOS: false}
	_, ok = r2.resolveJSONSource("SCOPE_OS")
	assert.False(t, ok)

	// Non-OS scope entries should still be found
	scope2 := NewEnvScope(nil, false).
		WithEntry("SCOPE_DAG", `{"c":3}`, EnvSourceDAGEnv)
	r3 := &resolver{scope: scope2, expandOS: false}
	val, ok := r3.resolveJSONSource("SCOPE_DAG")
	assert.True(t, ok)
	assert.Equal(t, `{"c":3}`, val)
}

// --- expandReferences short submatch ---

func TestExpandReferences_ShortSubmatch(t *testing.T) {
	ctx := context.Background()
	dataMap := map[string]string{
		"DATA": `{"key":"val"}`,
	}
	result := ExpandReferences(ctx, "$DATA.key", dataMap)
	assert.Equal(t, "val", result)
}

// --- replaceVars ---

func TestReplaceVars(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "BasicSubstitution",
			template: "${FOO}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "ShortSyntax",
			template: "$FOO",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "BAR",
		},
		{
			name:     "NoSubstitution",
			template: "$FOO_",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "$FOO_",
		},
		{
			name:     "InMiddleOfString",
			template: "prefix $FOO suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix BAR suffix",
		},
		{
			name:     "InMiddleOfStringAndNoSubstitution",
			template: "prefix $FOO1 suffix",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "prefix $FOO1 suffix",
		},
		{
			name:     "MissingVar",
			template: "${MISSING}",
			vars:     map[string]string{"FOO": "BAR"},
			want:     "${MISSING}",
		},
		{
			name:     "MultipleVars",
			template: "$FOO ${BAR} $BAZ",
			vars: map[string]string{
				"FOO": "1",
				"BAR": "2",
				"BAZ": "3",
			},
			want: "1 2 3",
		},
		{
			name:     "NestedVarsNotSupported",
			template: "${FOO${BAR}}",
			vars:     map[string]string{"FOO": "1", "BAR": "2"},
			want:     "${FOO${BAR}}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &resolver{variables: []map[string]string{tt.vars}}
			got := r.replaceVars(tt.template)
			if got != tt.want {
				t.Errorf("replaceVars() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestReplaceVars_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		template string
		vars     map[string]string
		want     string
	}{
		{
			name:     "SingleQuotesPreserved",
			template: "'$FOO'",
			vars:     map[string]string{"FOO": "bar"},
			want:     "'$FOO'",
		},
		{
			name:     "SingleQuotesPreservedWithBraces",
			template: "'${FOO}'",
			vars:     map[string]string{"FOO": "bar"},
			want:     "'${FOO}'",
		},
		{
			name:     "EmptyVariableName",
			template: "${}",
			vars:     map[string]string{"": "value"},
			want:     "${}",
		},
		{
			name:     "UnderscoreInVarName",
			template: "${FOO_BAR}",
			vars:     map[string]string{"FOO_BAR": "value"},
			want:     "value",
		},
		{
			name:     "NumberInVarName",
			template: "${FOO123}",
			vars:     map[string]string{"FOO123": "value"},
			want:     "value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &resolver{variables: []map[string]string{tt.vars}}
			got := r.replaceVars(tt.template)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestReplaceVarsWithScope(t *testing.T) {
	tests := []struct {
		name     string
		template string
		scope    *EnvScope
		want     string
	}{
		{
			name:     "UserDefinedVarExpanded",
			template: "Hello ${NAME}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("NAME", "World", EnvSourceDAGEnv)
			}(),
			want: "Hello World",
		},
		{
			name:     "OSVarNotExpanded",
			template: "Path is $PATH",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, true)
				return s
			}(),
			want: "Path is $PATH",
		},
		{
			name:     "JSONPathSkipped",
			template: "${step.stdout}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("step", "ignored", EnvSourceDAGEnv)
			}(),
			want: "${step.stdout}",
		},
		{
			name:     "SingleQuotedSkipped",
			template: "'${NAME}' stays",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("NAME", "World", EnvSourceDAGEnv)
			}(),
			want: "'${NAME}' stays",
		},
		{
			name:     "MixedExpansion",
			template: "User ${USER} env $HOME",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, true)
				return s.WithEntry("USER", "alice", EnvSourceDAGEnv)
			}(),
			want: "User alice env $HOME",
		},
		{
			name:     "StepEnvExpanded",
			template: "${STEP_VAR}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("STEP_VAR", "step_value", EnvSourceStepEnv)
			}(),
			want: "step_value",
		},
		{
			name:     "SecretExpanded",
			template: "key=${SECRET}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("SECRET", "s3cr3t", EnvSourceSecret)
			}(),
			want: "key=s3cr3t",
		},
		{
			name:     "NilScope",
			template: "${VAR}",
			scope:    nil,
			want:     "${VAR}",
		},
		{
			name:     "MissingVar",
			template: "${MISSING}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				return s.WithEntry("OTHER", "value", EnvSourceDAGEnv)
			}(),
			want: "${MISSING}",
		},
		{
			name:     "MultipleVars",
			template: "${A} ${B} ${C}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				s = s.WithEntry("A", "1", EnvSourceDAGEnv)
				s = s.WithEntry("B", "2", EnvSourceStepEnv)
				s = s.WithEntry("C", "3", EnvSourceSecret)
				return s
			}(),
			want: "1 2 3",
		},
		{
			name:     "ShortAndBracedSyntax",
			template: "$FOO and ${BAR}",
			scope: func() *EnvScope {
				s := NewEnvScope(nil, false)
				s = s.WithEntry("FOO", "foo", EnvSourceDAGEnv)
				s = s.WithEntry("BAR", "bar", EnvSourceDAGEnv)
				return s
			}(),
			want: "foo and bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &resolver{scope: tt.scope}
			got := r.replaceVars(tt.template)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- extractVarKey ---

func TestExtractVarKey(t *testing.T) {
	tests := []struct {
		name   string
		match  string
		wantK  string
		wantOK bool
	}{
		{
			name:   "BracedVar",
			match:  "${FOO}",
			wantK:  "FOO",
			wantOK: true,
		},
		{
			name:   "SimpleDollarVar",
			match:  "$FOO",
			wantK:  "FOO",
			wantOK: true,
		},
		{
			name:   "SingleQuotedBraced",
			match:  "'${FOO}'",
			wantK:  "",
			wantOK: false,
		},
		{
			name:   "SingleQuotedSimple",
			match:  "'$FOO'",
			wantK:  "",
			wantOK: false,
		},
		{
			name:   "VarWithUnderscore",
			match:  "${FOO_BAR}",
			wantK:  "FOO_BAR",
			wantOK: true,
		},
		{
			name:   "VarWithNumbers",
			match:  "$VAR123",
			wantK:  "VAR123",
			wantOK: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotK, gotOK := extractVarKey(tt.match)
			assert.Equal(t, tt.wantK, gotK)
			assert.Equal(t, tt.wantOK, gotOK)
		})
	}
}

// --- ExpandReferences ---

func TestExpandReferences(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		want    string
	}{
		{
			name:  "BasicReplacementWithCurlyBraces",
			input: "Hello: ${FOO.bar}",
			dataMap: map[string]string{
				"FOO": `{"bar": "World"}`,
			},
			want: "Hello: World",
		},
		{
			name:  "BasicReplacementWithSingleDollarSign",
			input: "Output => $FOO.value",
			dataMap: map[string]string{
				"FOO": `{"value": "SingleDollarWorks"}`,
			},
			want: "Output => SingleDollarWorks",
		},
		{
			name:  "MissingKeyInDataMap",
			input: "Hello: ${BAR.xyz}",
			dataMap: map[string]string{
				"FOO": `{"bar":"zzz"}`,
			},
			want: "Hello: ${BAR.xyz}",
		},
		{
			name:  "InvalidJSONInDataMap",
			input: "Test => ${FOO.bar}",
			dataMap: map[string]string{
				"FOO": `{"bar":`,
			},
			want: "Test => ${FOO.bar}",
		},
		{
			name:  "NestedSubPathExtraction",
			input: "Deep => ${FOO.level1.level2}",
			dataMap: map[string]string{
				"FOO": `{"level1": {"level2":"DeepValue"}}`,
			},
			want: "Deep => DeepValue",
		},
		{
			name:  "NonExistentSubPathInValidJSON",
			input: "Data => ${FOO.bar.baz}",
			dataMap: map[string]string{
				"FOO": `{"bar":"NotAnObject"}`,
			},
			want: "Data => ${FOO.bar.baz}",
		},
		{
			name:  "MultiplePlaceholdersIncludingSingleDollarForm",
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
			name:    "LookupFromEnvironment",
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

func TestExpandReferencesWithSteps(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		stepMap map[string]StepInfo
		want    string
	}{
		{
			name:    "BasicStepIDStdoutReference",
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
			name:    "StepIDStderrReference",
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
			name:    "StepIDExitCodeReference",
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
			name:    "MultipleStepReferences",
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
			name:    "UnknownStepIDLeavesAsIs",
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
			name:    "UnknownPropertyLeavesAsIs",
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
			name:  "StepIDPrecedenceOverVariable",
			input: "Value: ${download.stdout}",
			dataMap: map[string]string{
				"download": `{"stdout": "from-variable"}`,
			},
			stepMap: map[string]StepInfo{
				"download": {
					Stdout: "/tmp/logs/download.out",
				},
			},
			want: "Value: /tmp/logs/download.out",
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

func TestExpandReferencesWithSteps_Extended(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		dataMap map[string]string
		stepMap map[string]StepInfo
		want    string
	}{
		{
			name:    "StepStdoutReference",
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
			name:    "StepStderrReference",
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
			name:    "StepExitCodeReference",
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
			name:    "MissingStepReference",
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
			name:    "EmptyStepProperty",
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
			name:    "NilStepMap",
			input:   "No steps: ${step1.stdout}",
			dataMap: map[string]string{},
			stepMap: nil,
			want:    "No steps: ${step1.stdout}",
		},
		{
			name:  "RegularVariableTakesPrecedence",
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
			name:    "DollarSignWithoutBraces",
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
			name:    "MultipleStepReferences",
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
		{
			name:    "StepStdoutReferenceWithSlice",
			input:   "Slice: ${step1.stdout:0:4}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "abcdef",
				},
			},
			want: "Slice: abcd",
		},
		{
			name:    "StepStdoutReferenceWithOffsetOnly",
			input:   "Tail: ${step1.stdout:3}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "abcdef",
				},
			},
			want: "Tail: def",
		},
		{
			name:    "StepStdoutReferenceWithSliceBeyondLength",
			input:   "Beyond: ${step1.stdout:10:3}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "abc",
				},
			},
			want: "Beyond: ",
		},
		{
			name:    "StepStdoutReferenceWithInvalidSlice",
			input:   "Invalid: ${step1.stdout:-1:2}",
			dataMap: map[string]string{},
			stepMap: map[string]StepInfo{
				"step1": {
					Stdout: "abcdef",
				},
			},
			want: "Invalid: ${step1.stdout:-1:2}",
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
