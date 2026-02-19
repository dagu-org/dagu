package agent

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/core/exec"
)

// Sentinel errors for skill store operations.
var (
	ErrSkillNotFound          = errors.New("skill not found")
	ErrSkillAlreadyExists     = errors.New("skill already exists")
	ErrSkillNameAlreadyExists = errors.New("skill name already exists")
	ErrInvalidSkillID         = errors.New("invalid skill ID")
)

// SkillType distinguishes skill origins.
type SkillType string

const (
	SkillTypeCustom  SkillType = "custom"
	SkillTypeBuiltin SkillType = "builtin" // reserved for future use
)

// Skill is the domain entity for a skill.
// Field names align with the Agent Skills open standard (agentskills.io).
type Skill struct {
	ID          string    `yaml:"id"                    json:"id"`
	Name        string    `yaml:"name"                  json:"name"`
	Description string    `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string    `yaml:"version,omitempty"     json:"version,omitempty"`
	Author      string    `yaml:"author,omitempty"      json:"author,omitempty"`
	Tags        []string  `yaml:"tags,omitempty"        json:"tags,omitempty"`
	Type        SkillType `yaml:"type"                  json:"type"`
	Knowledge   string    `yaml:"knowledge"             json:"knowledge"`
}

// SkillStore defines the interface for skill persistence.
// All implementations must be safe for concurrent use.
type SkillStore interface {
	Create(ctx context.Context, skill *Skill) error
	GetByID(ctx context.Context, id string) (*Skill, error)
	List(ctx context.Context) ([]*Skill, error)
	Search(ctx context.Context, opts SearchSkillsOptions) (*exec.PaginatedResult[SkillMetadata], error)
	Update(ctx context.Context, skill *Skill) error
	Delete(ctx context.Context, id string) error
}

// SkillMetadata is a lightweight skill view excluding Knowledge.
// Used for search results where loading full knowledge content is unnecessary.
type SkillMetadata struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description,omitempty"`
	Tags          []string  `json:"tags,omitempty"`
	KnowledgeSize int       `json:"knowledge_size"`
	Version       string    `json:"version,omitempty"`
	Author        string    `json:"author,omitempty"`
	Type          SkillType `json:"type"`
}

// SearchSkillsOptions configures a paginated skill search query.
type SearchSkillsOptions struct {
	Paginator  exec.Paginator      // page-based pagination (page=1, perPage=50 by default)
	Query      string              // keyword filter (case-insensitive, matches name/description/tags)
	Tags       []string            // require ALL specified tags (AND semantics)
	AllowedIDs map[string]struct{} // nil = no restriction; non-nil = restrict to these IDs
}

// ValidateSkillID validates that id is a safe, well-formed skill identifier.
func ValidateSkillID(id string) error {
	return validateSlugID(id, ErrInvalidSkillID)
}

// SkillSummary contains lightweight skill metadata for system prompt listing.
type SkillSummary struct {
	ID          string
	Name        string
	Description string
}

// ToSkillSet converts a slice of skill IDs to a set for O(1) lookups.
// Returns nil if the input is empty (meaning "all skills allowed").
func ToSkillSet(ids []string) map[string]struct{} {
	if len(ids) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set
}

// LoadSkillSummaries returns metadata for the given skill IDs.
// Unknown or inaccessible skills are silently skipped.
func LoadSkillSummaries(ctx context.Context, store SkillStore, enabledIDs []string) []SkillSummary {
	if store == nil || len(enabledIDs) == 0 {
		return nil
	}
	summaries := make([]SkillSummary, 0, len(enabledIDs))
	for _, id := range enabledIDs {
		skill, err := store.GetByID(ctx, id)
		if err != nil {
			continue
		}
		summaries = append(summaries, SkillSummary{
			ID:          skill.ID,
			Name:        skill.Name,
			Description: skill.Description,
		})
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}
