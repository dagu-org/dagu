package agent

import (
	"encoding/json"
	"fmt"

	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "use_skill",
		Label:          "Use Skill",
		Description:    "Load domain expertise from a registered skill",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.SkillStore == nil {
				return nil
			}
			return NewUseSkillTool(cfg.SkillStore, cfg.AllowedSkills)
		},
	})
}

// useSkillInput is the expected JSON input for the use_skill tool.
type useSkillInput struct {
	SkillID string `json:"skill_id"`
}

// NewUseSkillTool creates a tool that loads skill knowledge on demand.
// allowedSkills restricts which skill IDs can be loaded. If nil, all skills are allowed.
func NewUseSkillTool(store SkillStore, allowedSkills map[string]struct{}) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "use_skill",
				Description: "Load domain expertise from a registered skill. Call this when a skill listed in <available_skills> is relevant to the current task.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"skill_id": map[string]any{
							"type":        "string",
							"description": "The ID of the skill to load",
						},
					},
					"required": []string{"skill_id"},
				},
			},
		},
		Run: makeUseSkillRun(store, allowedSkills),
		Audit: &AuditInfo{
			Action:          "skill_use",
			DetailExtractor: ExtractFields("skill_id"),
		},
	}
}

// makeUseSkillRun returns a ToolFunc that loads skill knowledge from the store.
func makeUseSkillRun(store SkillStore, allowedSkills map[string]struct{}) ToolFunc {
	return func(ctx ToolContext, input json.RawMessage) ToolOut {
		var params useSkillInput
		if err := json.Unmarshal(input, &params); err != nil {
			return toolError("invalid input: %v", err)
		}
		if params.SkillID == "" {
			return toolError("skill_id is required")
		}

		// Enforce allowed skills if a restriction is set.
		if allowedSkills != nil {
			if _, ok := allowedSkills[params.SkillID]; !ok {
				return toolError("skill %q is not available in this context", params.SkillID)
			}
		}

		skill, err := store.GetByID(ctx.Context, params.SkillID)
		if err != nil {
			return toolError("failed to load skill %q: %v", params.SkillID, err)
		}

		return ToolOut{
			Content: fmt.Sprintf("<skill name=%q id=%q>\n%s\n</skill>", skill.Name, skill.ID, skill.Knowledge),
		}
	}
}
