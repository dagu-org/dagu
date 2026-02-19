package agent

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/llm"
)

func init() {
	RegisterTool(ToolRegistration{
		Name:           "search_skills",
		Label:          "Search Skills",
		Description:    "Discover available skills by keyword or tag",
		DefaultEnabled: true,
		Factory: func(cfg ToolConfig) *AgentTool {
			if cfg.SkillStore == nil {
				return nil
			}
			return NewSearchSkillsTool(cfg.SkillStore, cfg.AllowedSkills)
		},
	})
}

// searchSkillsInput is the expected JSON input for the search_skills tool.
type searchSkillsInput struct {
	Query string   `json:"query,omitempty"`
	Tags  []string `json:"tags,omitempty"`
}

// NewSearchSkillsTool creates a tool that searches available skills by keyword or tag.
// allowedSkills restricts which skill IDs are visible. If nil, all skills are returned.
func NewSearchSkillsTool(store SkillStore, allowedSkills map[string]struct{}) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "search_skills",
				Description: "Search for available skills by keyword or tag. Returns skill summaries (ID, name, description, tags) without loading full knowledge. Use this to discover skills when you need domain expertise.",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Optional keyword to filter skills. Matches against name, description, and tags (case-insensitive).",
						},
						"tags": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
							"description": "Optional list of tags to filter by. Skills must have ALL specified tags to match.",
						},
					},
				},
			},
		},
		Run: makeSearchSkillsRun(store, allowedSkills),
		Audit: &AuditInfo{
			Action:          "skill_search",
			DetailExtractor: ExtractFields("query", "tags"),
		},
	}
}

// skillResult is the output format for a single skill in search results.
type skillResult struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// makeSearchSkillsRun returns a ToolFunc that searches skills from the store.
func makeSearchSkillsRun(store SkillStore, allowedSkills map[string]struct{}) ToolFunc {
	return func(ctx ToolContext, input json.RawMessage) ToolOut {
		var params searchSkillsInput
		if err := json.Unmarshal(input, &params); err != nil {
			return toolError("invalid input: %v", err)
		}

		allSkills, err := store.List(ctx.Context)
		if err != nil {
			return toolError("failed to list skills: %v", err)
		}

		queryLower := strings.ToLower(params.Query)
		var results []skillResult

		for _, skill := range allSkills {
			// Enforce allowed skills restriction.
			if allowedSkills != nil {
				if _, ok := allowedSkills[skill.ID]; !ok {
					continue
				}
			}

			// Apply tag filter: skill must have ALL specified tags.
			if len(params.Tags) > 0 && !hasAllTags(skill.Tags, params.Tags) {
				continue
			}

			// Apply keyword filter against name, description, and tags.
			if queryLower != "" && !matchesQuery(skill, queryLower) {
				continue
			}

			results = append(results, skillResult{
				ID:          skill.ID,
				Name:        skill.Name,
				Description: skill.Description,
				Tags:        skill.Tags,
			})
		}

		if len(results) == 0 {
			msg := "No skills found"
			if params.Query != "" {
				msg += fmt.Sprintf(" matching %q", params.Query)
			}
			if len(params.Tags) > 0 {
				msg += fmt.Sprintf(" with tags %v", params.Tags)
			}
			return ToolOut{Content: msg}
		}

		out, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return toolError("failed to format results: %v", err)
		}
		return ToolOut{
			Content: fmt.Sprintf("Found %d skill(s):\n%s", len(results), string(out)),
		}
	}
}

// hasAllTags returns true if skillTags contains all of the required tags (case-insensitive).
func hasAllTags(skillTags, required []string) bool {
	tagSet := make(map[string]struct{}, len(skillTags))
	for _, t := range skillTags {
		tagSet[strings.ToLower(t)] = struct{}{}
	}
	for _, req := range required {
		if _, ok := tagSet[strings.ToLower(req)]; !ok {
			return false
		}
	}
	return true
}

// matchesQuery checks if a skill matches a case-insensitive query against name, description, and tags.
func matchesQuery(skill *Skill, queryLower string) bool {
	if strings.Contains(strings.ToLower(skill.Name), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(skill.Description), queryLower) {
		return true
	}
	for _, tag := range skill.Tags {
		if strings.Contains(strings.ToLower(tag), queryLower) {
			return true
		}
	}
	return false
}
