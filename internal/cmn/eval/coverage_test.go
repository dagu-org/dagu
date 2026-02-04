package eval

import (
	"context"
	"os"
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

// --- shellExpandPhase coverage ---

func TestShellExpandPhase_FallbackOnError(t *testing.T) {
	ctx := context.Background()
	// Input with command substitution syntax that causes UnexpectedCommandError
	opts := NewOptions()
	t.Setenv("TESTVAR", "value123")

	result, err := shellExpandPhase(ctx, "$(echo hello) $TESTVAR", opts)
	require.NoError(t, err)
	// Falls back to ExpandEnvContext which preserves $(echo hello)
	assert.Contains(t, result, "value123")
}

// --- expandWithShellContext coverage ---

func TestExpandWithShellContext_ShellDisabledEnvEnabled(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandShell = false
	opts.ExpandEnv = true
	t.Setenv("MYVAR", "myval")

	result, err := expandWithShellContext(ctx, "$MYVAR", opts)
	require.NoError(t, err)
	assert.Equal(t, "myval", result)
}

func TestExpandWithShellContext_ShellDisabledEnvDisabled(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	opts.ExpandShell = false
	opts.ExpandEnv = false

	result, err := expandWithShellContext(ctx, "$MYVAR", opts)
	require.NoError(t, err)
	assert.Equal(t, "$MYVAR", result)
}

func TestExpandWithShellContext_EmptyInput(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	result, err := expandWithShellContext(ctx, "", opts)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestExpandWithShellContext_UnexpectedCommand(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()
	t.Setenv("KEEP", "kept")

	// $(command) triggers UnexpectedCommandError and falls back to ExpandEnvContext
	result, err := expandWithShellContext(ctx, "$(echo x) $KEEP", opts)
	require.NoError(t, err)
	assert.Contains(t, result, "kept")
}

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
	// OS vars in scope should be skipped, falling to os.LookupEnv
	t.Setenv("OSVAR", "live_os_value")
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("OSVAR", "frozen_value", EnvSourceOS)
	r := &resolver{scope: scope}

	val, ok := r.resolveForShell("OSVAR")
	assert.True(t, ok)
	assert.Equal(t, "live_os_value", val)
}

func TestResolveForShell_OSEnvFallback(t *testing.T) {
	t.Setenv("TESTOSVAR", "osval")
	r := &resolver{}

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

func TestResolveJSONSource_FromOSEnv(t *testing.T) {
	t.Setenv("JSONOSVAR", `{"b":2}`)
	r := &resolver{}

	val, ok := r.resolveJSONSource("JSONOSVAR")
	assert.True(t, ok)
	assert.Equal(t, `{"b":2}`, val)
}

func TestResolveJSONSource_NotFound(t *testing.T) {
	r := &resolver{}
	_, ok := r.resolveJSONSource("NOPE_NOT_HERE_12345")
	assert.False(t, ok)
}

// --- Object coverage ---

func TestObject_ErrorPropagation(t *testing.T) {
	ctx := context.Background()
	type S struct {
		Field string
	}
	input := S{Field: "`nonexistent_cmd_abc123`"}
	_, err := Object(ctx, input, map[string]string{})
	assert.Error(t, err)
}

// --- resolveJSONPath coverage ---

func TestResolveJSONPath_ParseError(t *testing.T) {
	ctx := context.Background()
	// Invalid jq path syntax
	_, ok := resolveJSONPath(ctx, "VAR", `{"a":1}`, ".[invalid")
	assert.False(t, ok)
}

func TestResolveJSONPath_NoResult(t *testing.T) {
	ctx := context.Background()
	// "empty" produces zero results from the iterator
	_, ok := resolveJSONPath(ctx, "VAR", `{"a":1}`, "empty")
	assert.False(t, ok)
}

func TestResolveJSONPath_ErrorResult(t *testing.T) {
	ctx := context.Background()
	// .bar on a non-object produces an error result
	_, ok := resolveJSONPath(ctx, "VAR", `"not_an_object"`, ".bar.baz")
	assert.False(t, ok)
}

// --- resolveStepProperty coverage ---

func TestResolveStepProperty_EmptyStderr(t *testing.T) {
	ctx := context.Background()
	stepMap := map[string]StepInfo{
		"step1": {Stdout: "out", Stderr: "", ExitCode: "0"},
	}
	_, ok := resolveStepProperty(ctx, "step1", ".stderr", stepMap)
	assert.False(t, ok)
}

func TestResolveStepProperty_DefaultProperty(t *testing.T) {
	ctx := context.Background()
	stepMap := map[string]StepInfo{
		"step1": {Stdout: "out", ExitCode: "0"},
	}
	// Unknown property
	_, ok := resolveStepProperty(ctx, "step1", ".unknown_prop", stepMap)
	assert.False(t, ok)
}

// --- expandReferences short submatch ---

func TestExpandReferences_ShortSubmatch(t *testing.T) {
	ctx := context.Background()
	// Exercise the $VAR.path syntax (no braces)
	dataMap := map[string]string{
		"DATA": `{"key":"val"}`,
	}
	result := ExpandReferences(ctx, "$DATA.key", dataMap)
	assert.Equal(t, "val", result)
}

// --- buildShellCommand coverage ---

func TestBuildShellCommand_PowerShell(t *testing.T) {
	cmd := buildShellCommand("powershell", "Get-Date")
	assert.Equal(t, "powershell", cmd.Path)
	assert.Contains(t, cmd.Args, "-Command")
}

func TestBuildShellCommand_Cmd(t *testing.T) {
	cmd := buildShellCommand("cmd.exe", "dir")
	assert.Contains(t, cmd.Args, "/c")
}

func TestBuildShellCommand_EmptyShell(t *testing.T) {
	cmd := buildShellCommand("", "echo hi")
	assert.Contains(t, cmd.Args, "-c")
}

// --- runCommandWithContext coverage ---

func TestRunCommandWithContext_WithScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("CMD_TEST_VAR", "from_scope", EnvSourceDAGEnv)
	// Need PATH to find echo
	scope = scope.WithEntry("PATH", os.Getenv("PATH"), EnvSourceOS)
	ctx := WithEnvScope(context.Background(), scope)

	result, err := runCommandWithContext(ctx, "echo hello")
	require.NoError(t, err)
	assert.Equal(t, "hello", result)
}

// --- envscope expandWithLookup single-quoted ---

func TestExpandWithLookup_SingleQuoted(t *testing.T) {
	lookup := func(key string) (string, bool) {
		if key == "FOO" {
			return "bar", true
		}
		return "", false
	}
	result := expandWithLookup("'$FOO' stays", lookup)
	assert.Equal(t, "'$FOO' stays", result)
}

// --- EnvScope.Debug no parent ---

func TestEnvScope_Debug_NoParent(t *testing.T) {
	// NewEnvScope with includeOS=true has entries but no parent
	s := NewEnvScope(nil, true)
	debug := s.Debug()
	assert.Contains(t, debug, "EnvScope{")
	assert.NotContains(t, debug, "parent: <yes>")
}

// --- collectBySource nil ---

func TestCollectBySource_NilScope(t *testing.T) {
	var s *EnvScope
	result := s.AllBySource(EnvSourceDAGEnv)
	assert.Empty(t, result)
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

// --- String with ExpandEnv but expandWithShellContext fails ---

func TestString_ShellExpandFallback(t *testing.T) {
	t.Setenv("FBVAR", "fbval")
	ctx := context.Background()

	// $(cmd) triggers UnexpectedCommandError in shell expansion, falls back to ExpandEnvContext
	result, err := String(ctx, "$(echo x) $FBVAR")
	require.NoError(t, err)
	assert.Contains(t, result, "fbval")
}

// --- Pipeline execute with disabled phase ---

func TestPipeline_DisabledPhases(t *testing.T) {
	ctx := context.Background()
	t.Setenv("PVAR", "pval")

	// Substitute=false, ExpandEnv=false — only variable + quoted-ref phases run
	result, err := String(ctx, "`echo nope` $PVAR",
		WithoutSubstitute(),
		WithoutExpandEnv(),
		WithVariables(map[string]string{"X": "y"}),
	)
	require.NoError(t, err)
	// Command substitution not run, env expansion not run
	assert.Contains(t, result, "`echo nope`")
	assert.Contains(t, result, "$PVAR")
}

// --- expandQuotedRefs with step map ---

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

// --- resolveReference with JSON from scope ---

func TestResolveReference_JSONFromScope(t *testing.T) {
	scope := NewEnvScope(nil, false)
	scope = scope.WithEntry("JDATA", `{"x":"y"}`, EnvSourceDAGEnv)
	r := &resolver{scope: scope}

	val, ok := r.resolveReference(context.Background(), "JDATA", ".x")
	assert.True(t, ok)
	assert.Equal(t, "y", val)
}

// --- resolveReference JSON from OS env ---

func TestResolveReference_JSONFromOSEnv(t *testing.T) {
	t.Setenv("OSJSON", `{"a":"b"}`)
	r := &resolver{}

	val, ok := r.resolveReference(context.Background(), "OSJSON", ".a")
	assert.True(t, ok)
	assert.Equal(t, "b", val)
}

// --- resolveReference not found ---

func TestResolveReference_NotFound(t *testing.T) {
	r := &resolver{}
	_, ok := r.resolveReference(context.Background(), "NOPE12345", ".x")
	assert.False(t, ok)
}

// --- Debug nil scope ---

func TestEnvScope_Debug_NilScope(t *testing.T) {
	var s *EnvScope
	debug := s.Debug()
	assert.Equal(t, "EnvScope{nil}", debug)
}

// --- collectBySource with parent chain ---

func TestCollectBySource_WithParent(t *testing.T) {
	parent := NewEnvScope(nil, false)
	parent = parent.WithEntry("PKEY", "pval", EnvSourceDAGEnv)
	child := NewEnvScope(parent, false)
	child = child.WithEntry("CKEY", "cval", EnvSourceDAGEnv)

	result := child.AllBySource(EnvSourceDAGEnv)
	assert.Equal(t, "pval", result["PKEY"])
	assert.Equal(t, "cval", result["CKEY"])
}

// --- shellExpandPhase with real error (non-UnexpectedCommand) ---

func TestShellExpandPhase_NonCommandError(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	// ${UNSET_XYZ_99:?msg} causes expand.Literal to return a non-UnexpectedCommandError
	// because the :? operator demands the var be set
	result, err := shellExpandPhase(ctx, "${UNSET_XYZ_99:?required}", opts)
	require.NoError(t, err)
	// shellExpandPhase catches the error and falls back to ExpandEnvContext
	assert.Contains(t, result, "UNSET_XYZ_99")
}

// --- expandWithShellContext non-UnexpectedCommand error ---

func TestExpandWithShellContext_NonUnexpectedCommandError(t *testing.T) {
	ctx := context.Background()
	opts := NewOptions()

	// ${UNSET_VAR_ABC:?msg} triggers a non-UnexpectedCommand error from expand.Literal
	_, err := expandWithShellContext(ctx, "${UNSET_VAR_ABC:?required}", opts)
	assert.Error(t, err)
}
