package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "list_remote_nodes",
		Label:          "List Remote Nodes",
		Description:    "List available remote Dagu nodes for remote_agent",
		DefaultEnabled: false,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.RemoteNodeResolver == nil {
				return nil
			}
			return NewListRemoteNodesTool(cfg.RemoteNodeResolver)
		},
	})
}

type listRemoteNodesInput struct {
	NameFilter string `json:"name_filter,omitempty"`
}

// NewListRemoteNodesTool creates a tool for listing available remote Dagu nodes.
func NewListRemoteNodesTool(resolver RemoteNodeResolver) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "list_remote_nodes",
				Description: "List available remote Dagu nodes that can be targeted by remote_agent.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name_filter": map[string]any{
							"type":        "string",
							"description": "Optional substring filter to match node names",
						},
					},
				},
			},
		},
		Run: makeListRemoteNodesRun(resolver),
		Audit: &AuditInfo{
			Action:          "remote_nodes_list",
			DetailExtractor: ExtractFields("name_filter"),
		},
	}
}

func makeListRemoteNodesRun(resolver RemoteNodeResolver) ToolFunc {
	return func(ctx ToolContext, input json.RawMessage) ToolOut {
		if ctx.Role.IsSet() && !ctx.Role.CanExecute() {
			return toolError("Permission denied: list_remote_nodes requires execute permission")
		}

		var args listRemoteNodesInput
		if err := json.Unmarshal(input, &args); err != nil {
			return toolError("Failed to parse input: %v", err)
		}

		nodes, err := resolver.ListTokenAuthNodes(ctx.Context)
		if err != nil {
			return toolError("Failed to list remote nodes: %v", err)
		}

		// Apply name filter if provided.
		if args.NameFilter != "" {
			filter := strings.ToLower(args.NameFilter)
			var filtered []RemoteNodeInfo
			for _, n := range nodes {
				if strings.Contains(strings.ToLower(n.Name), filter) {
					filtered = append(filtered, n)
				}
			}
			nodes = filtered
		}

		if len(nodes) == 0 {
			return ToolOut{Content: "No remote nodes found."}
		}

		var sb strings.Builder
		fmt.Fprintf(&sb, "Found %d remote node(s):\n\n", len(nodes))
		for _, n := range nodes {
			fmt.Fprintf(&sb, "- **%s**", n.Name)
			if n.Description != "" {
				fmt.Fprintf(&sb, ": %s", n.Description)
			}
			sb.WriteByte('\n')
		}

		return ToolOut{Content: sb.String()}
	}
}
