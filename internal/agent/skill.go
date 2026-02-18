package agent

import (
	"context"
	"errors"
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
	Update(ctx context.Context, skill *Skill) error
	Delete(ctx context.Context, id string) error
}

// ValidateSkillID validates that id is a safe, well-formed skill identifier.
func ValidateSkillID(id string) error {
	return validateSlugID(id, ErrInvalidSkillID)
}
