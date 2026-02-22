package agent

import (
	"context"
	"errors"

	"github.com/dagu-org/dagu/internal/core/exec"
)

// Sentinel errors for soul store operations.
var (
	ErrSoulNotFound          = errors.New("soul not found")
	ErrSoulAlreadyExists     = errors.New("soul already exists")
	ErrSoulNameAlreadyExists = errors.New("soul name already exists")
	ErrInvalidSoulID         = errors.New("invalid soul ID")
)

// Soul is the domain entity for an agent personality.
// Each soul defines identity, priorities, and communication style
// that get injected into the system prompt.
type Soul struct {
	ID          string   `yaml:"id"                    json:"id"`
	Name        string   `yaml:"name"                  json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Version     string   `yaml:"version,omitempty"     json:"version,omitempty"`
	Author      string   `yaml:"author,omitempty"      json:"author,omitempty"`
	Tags        []string `yaml:"tags,omitempty"        json:"tags,omitempty"`
	Content     string   `yaml:"content"               json:"content"`
}

// SoulStore defines the interface for soul persistence.
// All implementations must be safe for concurrent use.
type SoulStore interface {
	Create(ctx context.Context, soul *Soul) error
	GetByID(ctx context.Context, id string) (*Soul, error)
	List(ctx context.Context) ([]*Soul, error)
	Search(ctx context.Context, opts SearchSoulsOptions) (*exec.PaginatedResult[SoulMetadata], error)
	Update(ctx context.Context, soul *Soul) error
	Delete(ctx context.Context, id string) error
}

// SoulMetadata is a lightweight soul view excluding Content.
// Used for search results where loading full content is unnecessary.
type SoulMetadata struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	ContentSize int      `json:"content_size"`
	Version     string   `json:"version,omitempty"`
	Author      string   `json:"author,omitempty"`
}

// SearchSoulsOptions configures a paginated soul search query.
type SearchSoulsOptions struct {
	Paginator exec.Paginator
	Query     string
	Tags      []string
}

// ValidateSoulID validates that id is a safe, well-formed soul identifier.
func ValidateSoulID(id string) error {
	return validateSlugID(id, ErrInvalidSoulID)
}
