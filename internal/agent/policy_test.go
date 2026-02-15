package agent

import (
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveToolPolicy_Defaults(t *testing.T) {
	t.Parallel()

	resolved := ResolveToolPolicy(iface.ToolPolicyConfig{})

	assert.True(t, resolved.Tools[toolNameBash])
	assert.True(t, resolved.Tools[toolNameRead])
	assert.True(t, resolved.Tools[toolNamePatch])
	assert.True(t, resolved.Tools[toolNameThink])
	assert.True(t, resolved.Tools[toolNameNavigate])
	assert.True(t, resolved.Tools[toolNameReadSchema])
	assert.True(t, resolved.Tools[toolNameAskUser])
	assert.True(t, resolved.Tools[toolNameWebSearch])
	assert.Equal(t, iface.BashDefaultBehaviorAllow, resolved.Bash.DefaultBehavior)
	assert.Equal(t, iface.BashDenyBehaviorAskUser, resolved.Bash.DenyBehavior)
}

func TestValidateToolPolicy(t *testing.T) {
	t.Parallel()

	t.Run("rejects unknown tools", func(t *testing.T) {
		t.Parallel()
		err := ValidateToolPolicy(iface.ToolPolicyConfig{
			Tools: map[string]bool{"unknown_tool": true},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unknown tool")
	})

	t.Run("rejects invalid regex", func(t *testing.T) {
		t.Parallel()
		err := ValidateToolPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				Rules: []iface.BashRule{
					{Pattern: "([", Action: iface.BashRuleActionAllow},
				},
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid regex")
	})
}

func TestEvaluateBashPolicy(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`{"command":"git status"}`)

	t.Run("tool disabled denies", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Tools: map[string]bool{"bash": false},
		}, input)
		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Equal(t, iface.BashDenyBehaviorBlock, decision.DenyBehavior)
	})

	t.Run("matching allow rule permits", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				Rules: []iface.BashRule{
					{Name: "allow_git", Pattern: "^git\\s+", Action: iface.BashRuleActionAllow},
				},
			},
		}, input)
		require.NoError(t, err)
		assert.True(t, decision.Allowed)
	})

	t.Run("matching deny rule blocks", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				Rules: []iface.BashRule{
					{Name: "deny_git", Pattern: "^git\\s+", Action: iface.BashRuleActionDeny},
				},
				DenyBehavior: iface.BashDenyBehaviorAskUser,
			},
		}, input)
		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Equal(t, "deny_git", decision.RuleName)
		assert.Equal(t, iface.BashDenyBehaviorAskUser, decision.DenyBehavior)
	})

	t.Run("disabled deny rule is ignored", func(t *testing.T) {
		t.Parallel()
		disabled := false
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				Rules: []iface.BashRule{
					{Name: "deny_git_disabled", Pattern: "^git\\s+", Action: iface.BashRuleActionDeny, Enabled: &disabled},
				},
				DefaultBehavior: iface.BashDefaultBehaviorAllow,
			},
		}, input)
		require.NoError(t, err)
		assert.True(t, decision.Allowed)
	})

	t.Run("no match uses default deny", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				Rules: []iface.BashRule{
					{Name: "allow_ls", Pattern: "^ls\\b", Action: iface.BashRuleActionAllow},
				},
				DefaultBehavior: iface.BashDefaultBehaviorDeny,
			},
		}, input)
		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Contains(t, decision.Reason, "no matching allow rule")
	})

	t.Run("no match with default allow permits", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				DefaultBehavior: iface.BashDefaultBehaviorAllow,
			},
		}, input)
		require.NoError(t, err)
		assert.True(t, decision.Allowed)
	})

	t.Run("unsupported shell constructs are denied", func(t *testing.T) {
		t.Parallel()
		decision, err := EvaluateBashPolicy(iface.ToolPolicyConfig{
			Bash: iface.BashPolicyConfig{
				DenyBehavior: iface.BashDenyBehaviorAskUser,
			},
		}, json.RawMessage(`{"command":"echo $(uname -a)"}`))
		require.NoError(t, err)
		assert.False(t, decision.Allowed)
		assert.Contains(t, decision.Reason, "unsupported shell construct")
		assert.Equal(t, iface.BashDenyBehaviorAskUser, decision.DenyBehavior)
	})
}

func TestSplitShellCommandSegments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		want []string
	}{
		{
			name: "semicolon and and-or split",
			cmd:  "ls -la; git status && echo ok || echo ng",
			want: []string{"ls -la", "git status", "echo ok", "echo ng"},
		},
		{
			name: "pipe split",
			cmd:  "cat file | grep x",
			want: []string{"cat file", "grep x"},
		},
		{
			name: "operator in quotes ignored",
			cmd:  `echo "a && b"; echo 'x|y'`,
			want: []string{`echo "a && b"`, `echo 'x|y'`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := splitShellCommandSegments(tc.cmd)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestHasUnsupportedShellConstructs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		cmd  string
		want bool
	}{
		{name: "plain command", cmd: "git status", want: false},
		{name: "subshell", cmd: "echo $(date)", want: true},
		{name: "backticks", cmd: "echo `date`", want: true},
		{name: "heredoc", cmd: "cat <<EOF\nhello\nEOF", want: true},
		{name: "process substitution input", cmd: "cat <(date)", want: true},
		{name: "process substitution output", cmd: "echo hi >(cat)", want: true},
		{name: "backticks in single quote are ignored", cmd: "echo '`date`'", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, hasUnsupportedShellConstructs(tc.cmd))
		})
	}
}
