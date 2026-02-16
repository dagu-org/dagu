package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"
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
	ID            string    `yaml:"id"                    json:"id"`
	SchemaVersion int       `yaml:"schema_version"        json:"schemaVersion"`
	Name          string    `yaml:"name"                  json:"name"`
	Description   string    `yaml:"description,omitempty" json:"description,omitempty"`
	Version       string    `yaml:"version,omitempty"     json:"version,omitempty"`
	Author        string    `yaml:"author,omitempty"      json:"author,omitempty"`
	Tags          []string  `yaml:"tags,omitempty"        json:"tags,omitempty"`
	Type          SkillType `yaml:"type"                  json:"type"`
	Knowledge     string    `yaml:"knowledge"             json:"knowledge"`
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

// validSkillIDRegexp matches a valid skill ID slug: lowercase alphanumeric segments separated by hyphens.
var validSkillIDRegexp = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

const maxSkillIDLength = 128

// ValidateSkillID validates that id is a safe, well-formed skill identifier.
// It must be a non-empty slug (lowercase alphanumeric segments separated by hyphens)
// and at most 128 characters. This prevents path traversal and other injection attacks.
func ValidateSkillID(id string) error {
	if id == "" {
		return ErrInvalidSkillID
	}
	if len(id) > maxSkillIDLength {
		return fmt.Errorf("%w: exceeds maximum length of %d", ErrInvalidSkillID, maxSkillIDLength)
	}
	if !validSkillIDRegexp.MatchString(id) {
		return fmt.Errorf("%w: must match pattern [a-z0-9]+(-[a-z0-9]+)*", ErrInvalidSkillID)
	}
	return nil
}
