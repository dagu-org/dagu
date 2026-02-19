package agent

import (
	"encoding/json"
	"fmt"

	"github.com/dagu-org/dagu/internal/core/exec"
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
	Query   string   `json:"query,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Page    int      `json:"page,omitempty"`
	PerPage int      `json:"per_page,omitempty"`
}

// NewSearchSkillsTool creates a tool that searches available skills by keyword or tag.
// allowedSkills restricts which skill IDs are visible. If nil, all skills are returned.
func NewSearchSkillsTool(store SkillStore, allowedSkills map[string]struct{}) *AgentTool {
	return &AgentTool{
		Tool: llm.Tool{
			Type: "function",
			Function: llm.ToolFunction{
				Name:        "search_skills",
				Description: "Search for available skills by keyword or tag. Returns paginated skill summaries (ID, name, description, tags) without loading full knowledge. Use this to discover skills when you need domain expertise.",
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
						"page": map[string]any{
							"type":        "integer",
							"description": "Page number for pagination (default: 1).",
						},
						"per_page": map[string]any{
							"type":        "integer",
							"description": "Number of results per page (default: 50, max: 200).",
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

// searchSkillsOutput is the structured output for paginated search results.
type searchSkillsOutput struct {
	TotalCount  int             `json:"total_count"`
	CurrentPage int             `json:"current_page"`
	TotalPages  int             `json:"total_pages"`
	HasNextPage bool            `json:"has_next_page"`
	Results     []SkillMetadata `json:"results"`
}

// makeSearchSkillsRun returns a ToolFunc that searches skills from the store.
func makeSearchSkillsRun(store SkillStore, allowedSkills map[string]struct{}) ToolFunc {
	return func(ctx ToolContext, input json.RawMessage) ToolOut {
		var params searchSkillsInput
		if err := json.Unmarshal(input, &params); err != nil {
			return toolError("invalid input: %v", err)
		}

		pg := exec.NewPaginator(params.Page, params.PerPage)

		result, err := store.Search(ctx.Context, SearchSkillsOptions{
			Paginator:  pg,
			Query:      params.Query,
			Tags:       params.Tags,
			AllowedIDs: allowedSkills,
		})
		if err != nil {
			return toolError("failed to list skills: %v", err)
		}

		if result.TotalCount == 0 {
			msg := "No skills found"
			if params.Query != "" {
				msg += fmt.Sprintf(" matching %q", params.Query)
			}
			if len(params.Tags) > 0 {
				msg += fmt.Sprintf(" with tags %v", params.Tags)
			}
			return ToolOut{Content: msg}
		}

		output := searchSkillsOutput{
			TotalCount:  result.TotalCount,
			CurrentPage: result.CurrentPage,
			TotalPages:  result.TotalPages,
			HasNextPage: result.HasNextPage,
			Results:     result.Items,
		}

		out, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return toolError("failed to format results: %v", err)
		}
		return ToolOut{
			Content: fmt.Sprintf("Found %d skill(s):\n%s", result.TotalCount, string(out)),
		}
	}
}
