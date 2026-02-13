package frontend

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/service/audit"
)

const (
	auditActionToolPolicyDenied   = "tool_policy_denied"
	auditActionToolPolicyOverride = "tool_policy_override"
	maxAuditSegmentLen            = 512
)

// newAgentPolicyHook returns a before-tool hook that enforces agent tool policy.
func newAgentPolicyHook(configStore agent.ConfigStore, auditSvc *audit.Service) agent.BeforeToolExecHookFunc {
	return func(ctx context.Context, info agent.ToolExecInfo) error {
		if configStore == nil {
			return fmt.Errorf("agent policy unavailable")
		}

		cfg, err := configStore.Load(ctx)
		if err != nil || cfg == nil {
			logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, map[string]any{
				"tool_name": info.ToolName,
				"reason":    "failed to load policy configuration",
			})
			return fmt.Errorf("policy unavailable")
		}

		if err := agent.ValidateToolPolicy(cfg.ToolPolicy); err != nil {
			logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, map[string]any{
				"tool_name": info.ToolName,
				"reason":    "invalid policy configuration",
			})
			return fmt.Errorf("invalid policy configuration")
		}

		policy := agent.ResolveToolPolicy(cfg.ToolPolicy)

		if !agent.IsToolEnabledResolved(policy, info.ToolName) {
			logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, map[string]any{
				"tool_name": info.ToolName,
				"reason":    "tool disabled",
			})
			return fmt.Errorf("tool %q is disabled by policy", info.ToolName)
		}

		if info.ToolName != "bash" {
			return nil
		}

		decision, err := agent.EvaluateBashPolicy(policy, info.Input)
		if err != nil {
			logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, map[string]any{
				"tool_name": info.ToolName,
				"reason":    "bash policy evaluation failed",
			})
			return fmt.Errorf("bash policy evaluation failed: %w", err)
		}
		if decision.Allowed {
			return nil
		}

		details := map[string]any{
			"tool_name":       info.ToolName,
			"reason":          decision.Reason,
			"rule_name":       decision.RuleName,
			"command_segment": truncatePolicyAuditText(decision.Segment),
		}

		if decision.DenyBehavior == agent.BashDenyBehaviorAskUser && info.RequestCommandApproval != nil {
			reason := strings.TrimSpace(strings.Join([]string{
				decision.Reason,
				decision.RuleName,
			}, " "))
			approved, approvalErr := info.RequestCommandApproval(ctx, decision.Command, reason)
			if approvalErr != nil {
				details["approval_result"] = "error"
				logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, details)
				return fmt.Errorf("bash command denied by policy (approval failed: %w)", approvalErr)
			}
			if approved {
				details["approval_result"] = "approved"
				logPolicyEvent(auditSvc, info, auditActionToolPolicyOverride, details)
				return nil
			}
			details["approval_result"] = "rejected"
			logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, details)
			return fmt.Errorf("bash command denied by policy")
		}

		details["approval_result"] = "blocked"
		logPolicyEvent(auditSvc, info, auditActionToolPolicyDenied, details)
		return fmt.Errorf("bash command denied by policy")
	}
}

func logPolicyEvent(auditSvc *audit.Service, info agent.ToolExecInfo, action string, details map[string]any) {
	if auditSvc == nil {
		return
	}
	if details == nil {
		details = map[string]any{}
	}
	details["session_id"] = info.SessionID
	if _, ok := details["tool_name"]; !ok {
		details["tool_name"] = info.ToolName
	}

	payload, _ := json.Marshal(details)
	entry := audit.NewEntry(audit.CategoryAgent, action, info.UserID, info.Username).
		WithDetails(string(payload)).
		WithIPAddress(info.IPAddress)
	_ = auditSvc.Log(context.Background(), entry)
}

func truncatePolicyAuditText(s string) string {
	if len(s) <= maxAuditSegmentLen {
		return s
	}
	return s[:maxAuditSegmentLen] + "...(truncated)"
}
