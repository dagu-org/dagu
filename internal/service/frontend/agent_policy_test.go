package frontend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/agent/iface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAgentConfigStore struct {
	cfg     *iface.Config
	loadErr error
}

func (s *stubAgentConfigStore) Load(_ context.Context) (*iface.Config, error) {
	return s.cfg, s.loadErr
}

func (s *stubAgentConfigStore) Save(_ context.Context, cfg *iface.Config) error {
	s.cfg = cfg
	return nil
}

func (s *stubAgentConfigStore) IsEnabled(_ context.Context) bool {
	return s.cfg != nil && s.cfg.Enabled
}

func TestAgentPolicyHook(t *testing.T) {
	t.Parallel()

	makeInfo := func(tool string, input string) agent.ToolExecInfo {
		return agent.ToolExecInfo{
			ToolName: tool,
			Input:    json.RawMessage(input),
		}
	}

	t.Run("blocks disabled non-bash tool", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Tools["patch"] = false

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), makeInfo("patch", `{"path":"a"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("returns policy unavailable when config load fails", func(t *testing.T) {
		t.Parallel()

		hook := newAgentPolicyHook(&stubAgentConfigStore{loadErr: assert.AnError}, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"echo ok"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy unavailable")
	})

	t.Run("returns unavailable when config store is nil", func(t *testing.T) {
		t.Parallel()

		hook := newAgentPolicyHook(nil, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"echo ok"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy unavailable")
	})

	t.Run("allows bash when user approves denied command", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []iface.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  iface.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), agent.ToolExecInfo{
			ToolName: "bash",
			Input:    json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SafeMode: true,
			RequestCommandApproval: func(_ context.Context, _ string, _ string) (bool, error) {
				return true, nil
			},
		})
		require.NoError(t, err)
	})

	t.Run("blocks bash when user rejects denied command", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []iface.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  iface.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), agent.ToolExecInfo{
			ToolName: "bash",
			Input:    json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SafeMode: true,
			RequestCommandApproval: func(_ context.Context, _ string, _ string) (bool, error) {
				return false, nil
			},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "denied")
	})

	t.Run("allows denied command without prompt when safe mode is off", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []iface.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  iface.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), agent.ToolExecInfo{
			ToolName: "bash",
			Input:    json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SafeMode: false,
		})
		require.NoError(t, err)
	})

	t.Run("allows bash matching allow rule", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []iface.BashRule{
			{
				Name:    "allow_git",
				Pattern: "^git\\s+",
				Action:  iface.BashRuleActionAllow,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"git status"}`))
		require.NoError(t, err)
	})

	t.Run("blocks bash when deny behavior is block", func(t *testing.T) {
		t.Parallel()
		cfg := iface.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []iface.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  iface.BashRuleActionDeny,
			},
		}
		cfg.ToolPolicy.Bash.DenyBehavior = iface.BashDenyBehaviorBlock

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"rm -rf /tmp/x"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "denied")
	})
}
