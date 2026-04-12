// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package frontend

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dagucloud/dagu/internal/agent"
	"github.com/dagucloud/dagu/internal/service/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubAgentConfigStore struct {
	cfg     *agent.Config
	loadErr error
}

func (s *stubAgentConfigStore) Load(_ context.Context) (*agent.Config, error) {
	return s.cfg, s.loadErr
}

func (s *stubAgentConfigStore) Save(_ context.Context, cfg *agent.Config) error {
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
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Tools["patch"] = false

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
		err := hook(context.Background(), makeInfo("patch", `{"path":"a"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "disabled")
	})

	t.Run("returns policy unavailable when config load fails", func(t *testing.T) {
		t.Parallel()

		hook := newAgentPolicyHook(&stubAgentConfigStore{loadErr: assert.AnError}, nil, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"echo ok"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy unavailable")
	})

	t.Run("returns unavailable when config store is nil", func(t *testing.T) {
		t.Parallel()

		hook := newAgentPolicyHook(nil, nil, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"echo ok"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "policy unavailable")
	})

	t.Run("allows bash when user approves denied command", func(t *testing.T) {
		t.Parallel()
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
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
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
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
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
		err := hook(context.Background(), agent.ToolExecInfo{
			ToolName: "bash",
			Input:    json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SafeMode: false,
		})
		require.NoError(t, err)
	})

	t.Run("allows bash matching allow rule", func(t *testing.T) {
		t.Parallel()
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "allow_git",
				Pattern: "^git\\s+",
				Action:  agent.BashRuleActionAllow,
			},
		}

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"git status"}`))
		require.NoError(t, err)
	})

	t.Run("blocks bash when deny behavior is block", func(t *testing.T) {
		t.Parallel()
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}
		cfg.ToolPolicy.Bash.DenyBehavior = agent.BashDenyBehaviorBlock

		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, nil, nil)
		err := hook(context.Background(), makeInfo("bash", `{"command":"rm -rf /tmp/x"}`))
		require.Error(t, err)
		assert.Contains(t, err.Error(), "denied")
	})
}

func TestAgentPolicyHook_AuditGating(t *testing.T) {
	t.Parallel()

	makeStore := func() *stubAgentConfigStore {
		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}
		cfg.ToolPolicy.Bash.DenyBehavior = agent.BashDenyBehaviorBlock
		return &stubAgentConfigStore{cfg: cfg}
	}

	makeInfo := func() agent.ToolExecInfo {
		return agent.ToolExecInfo{
			ToolName:  "bash",
			Input:     json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SessionID: "sess-123",
			User: agent.UserIdentity{
				UserID:    "user-1",
				Username:  "alice",
				IPAddress: "127.0.0.1",
			},
		}
	}

	t.Run("skips denied audit events when audit is disabled", func(t *testing.T) {
		t.Parallel()

		store := &mockAuditStore{}
		hook := newAgentPolicyHook(makeStore(), audit.New(store), func() bool { return false })

		err := hook(context.Background(), makeInfo())
		require.Error(t, err)
		assert.Empty(t, store.entries)
	})

	t.Run("logs denied audit events when audit is enabled", func(t *testing.T) {
		t.Parallel()

		store := &mockAuditStore{}
		hook := newAgentPolicyHook(makeStore(), audit.New(store), func() bool { return true })

		err := hook(context.Background(), makeInfo())
		require.Error(t, err)
		require.Len(t, store.entries, 1)
		assert.Equal(t, auditActionToolPolicyDenied, store.entries[0].Action)
	})

	t.Run("skips approval override audit events when audit is disabled", func(t *testing.T) {
		t.Parallel()

		cfg := agent.DefaultConfig()
		cfg.ToolPolicy.Bash.Rules = []agent.BashRule{
			{
				Name:    "deny_rm",
				Pattern: "^rm\\s+",
				Action:  agent.BashRuleActionDeny,
			},
		}

		store := &mockAuditStore{}
		hook := newAgentPolicyHook(&stubAgentConfigStore{cfg: cfg}, audit.New(store), func() bool { return false })

		err := hook(context.Background(), agent.ToolExecInfo{
			ToolName: "bash",
			Input:    json.RawMessage(`{"command":"rm -rf /tmp/x"}`),
			SafeMode: true,
			RequestCommandApproval: func(_ context.Context, _ string, _ string) (bool, error) {
				return true, nil
			},
		})
		require.NoError(t, err)
		assert.Empty(t, store.entries)
	})
}
