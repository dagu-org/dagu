package agent

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/dagu-org/dagu/internal/core/exec"
)

// Sentinel errors for doc store operations.
var (
	ErrDocNotFound      = errors.New("doc not found")
	ErrDocAlreadyExists = errors.New("doc already exists")
	ErrInvalidDocID     = errors.New("invalid doc ID")
)

// Doc is the domain entity for a markdown document.
type Doc struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// DocMetadata is a lightweight doc view excluding Content.
type DocMetadata struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// DocTreeNode represents a file or directory in the doc tree.
type DocTreeNode struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Title    string         `json:"title,omitempty"`
	Type     string         `json:"type"` // "file" or "directory"
	Children []*DocTreeNode `json:"children,omitempty"`
}

// DocSearchResult holds a doc ID/title and its grep matches.
type DocSearchResult struct {
	ID      string        `json:"id"`
	Title   string        `json:"title"`
	Matches []*exec.Match `json:"matches"`
}

// DocStore defines the interface for doc persistence.
type DocStore interface {
	List(ctx context.Context, page, perPage int) (*exec.PaginatedResult[*DocTreeNode], error)
	ListFlat(ctx context.Context, page, perPage int) (*exec.PaginatedResult[DocMetadata], error)
	Get(ctx context.Context, id string) (*Doc, error)
	Create(ctx context.Context, id, content string) error
	Update(ctx context.Context, id, content string) error
	Delete(ctx context.Context, id string) error
	Rename(ctx context.Context, oldID, newID string) error
	Search(ctx context.Context, query string) ([]*DocSearchResult, error)
}

// validDocIDRegexp matches a valid doc ID: segments separated by slashes.
// Each segment starts with alphanumeric and can contain alphanumeric, underscore, dot, hyphen, or space.
var validDocIDRegexp = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_. -]*(/[a-zA-Z0-9][a-zA-Z0-9_. -]*)*$`)

// maxDocIDLength is the maximum allowed length for a doc ID.
const maxDocIDLength = 256

// ValidateDocID validates that id is a safe, well-formed doc identifier.
func ValidateDocID(id string) error {
	if id == "" {
		return ErrInvalidDocID
	}
	if len(id) > maxDocIDLength {
		return fmt.Errorf("%w: exceeds maximum length of %d", ErrInvalidDocID, maxDocIDLength)
	}
	if !validDocIDRegexp.MatchString(id) {
		return fmt.Errorf("%w: must match pattern [a-zA-Z0-9][a-zA-Z0-9_. -]*(/[a-zA-Z0-9][a-zA-Z0-9_. -]*)*", ErrInvalidDocID)
	}
	return nil
}
