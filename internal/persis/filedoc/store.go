package filedoc

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedag/grep"
	"github.com/goccy/go-yaml"
)

// Verify Store implements agent.DocStore at compile time.
var _ agent.DocStore = (*Store)(nil)

const (
	docDirPermissions = 0750
	filePermissions   = 0600
)

// docFrontmatter holds the YAML fields in the doc file frontmatter.
type docFrontmatter struct {
	Title string `yaml:"title,omitempty"`
}

// Store implements a file-based doc store.
// Docs are stored as files: {baseDir}/{id}.md
// Each file contains optional YAML frontmatter (title) and a Markdown body.
// Unlike the soul store, this has no cached index — it scans the filesystem
// on every call, following the DAG store pattern.
type Store struct {
	baseDir string
}

// New creates a new file-based doc store.
func New(baseDir string) *Store {
	if err := os.MkdirAll(baseDir, docDirPermissions); err != nil {
		// Best effort — the directory may be created later.
	}
	return &Store{baseDir: baseDir}
}

// docFilePath returns the file path for a doc ID and validates the path
// stays within baseDir to prevent path traversal.
func (s *Store) docFilePath(id string) (string, error) {
	p := filepath.Join(s.baseDir, id+".md")
	cleaned := filepath.Clean(p)
	if !strings.HasPrefix(cleaned, filepath.Clean(s.baseDir)+string(filepath.Separator)) {
		return "", fmt.Errorf("filedoc: path traversal detected for id %q", id)
	}
	return cleaned, nil
}

// parseDocFile parses a doc .md file into an agent.Doc.
// The file format is optional YAML frontmatter between --- delimiters, followed by markdown body.
func parseDocFile(data []byte, id string) (*agent.Doc, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")

	var title, body string

	if strings.HasPrefix(content, "---\n") {
		rest := content[4:]

		closingIdx := strings.Index(rest, "\n---\n")
		if closingIdx == -1 {
			if strings.HasSuffix(rest, "\n---") {
				closingIdx = len(rest) - 4
			} else {
				// No closing delimiter — treat entire content as body.
				body = content
			}
		}

		if closingIdx >= 0 {
			frontmatterStr := rest[:closingIdx]

			afterDelim := closingIdx + 5 // len("\n---\n")
			if afterDelim <= len(rest) {
				body = rest[afterDelim:]
			}

			var fm docFrontmatter
			if err := yaml.Unmarshal([]byte(frontmatterStr), &fm); err != nil {
				// Invalid frontmatter — treat entire content as body.
				body = content
			} else {
				title = fm.Title
			}
		}
	} else {
		body = content
	}

	if title == "" {
		title = titleFromID(id)
	}

	return &agent.Doc{
		ID:      id,
		Title:   title,
		Content: strings.TrimRight(body, "\n"),
	}, nil
}

// marshalDocFile produces the doc file content with optional frontmatter.
func marshalDocFile(title, content string) []byte {
	var buf bytes.Buffer
	if title != "" {
		fm := docFrontmatter{Title: title}
		fmBytes, err := yaml.Marshal(fm)
		if err == nil {
			buf.WriteString("---\n")
			buf.Write(fmBytes)
			buf.WriteString("---\n")
		}
	}
	if content != "" {
		buf.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			buf.WriteString("\n")
		}
	}
	return buf.Bytes()
}

// titleFromID derives a display title from a doc ID.
// E.g., "runbooks/deploy-guide" → "deploy-guide"
func titleFromID(id string) string {
	parts := strings.Split(id, "/")
	return parts[len(parts)-1]
}

// List returns a paginated tree of doc nodes.
func (s *Store) List(_ context.Context, page, perPage int) (*exec.PaginatedResult[*agent.DocTreeNode], error) {
	tree, err := s.buildTree()
	if err != nil {
		return nil, err
	}

	pg := exec.NewPaginator(page, perPage)
	total := len(tree)
	offset := pg.Offset()
	if offset > total {
		offset = total
	}
	end := min(offset+pg.Limit(), total)
	pageItems := tree[offset:end]

	result := exec.NewPaginatedResult(pageItems, total, pg)
	return &result, nil
}

// ListFlat returns a paginated flat list of doc metadata sorted alphabetically.
func (s *Store) ListFlat(_ context.Context, page, perPage int) (*exec.PaginatedResult[agent.DocMetadata], error) {
	var items []agent.DocMetadata

	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return nil
		}
		id := strings.TrimSuffix(relPath, ".md")

		data, err := os.ReadFile(path) //nolint:gosec // path constructed from baseDir
		if err != nil {
			return nil
		}

		doc, err := parseDocFile(data, id)
		if err != nil {
			return nil
		}

		items = append(items, agent.DocMetadata{
			ID:    doc.ID,
			Title: doc.Title,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filedoc: failed to walk directory: %w", err)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})

	pg := exec.NewPaginator(page, perPage)
	total := len(items)
	offset := pg.Offset()
	if offset > total {
		offset = total
	}
	end := min(offset+pg.Limit(), total)
	pageItems := items[offset:end]

	result := exec.NewPaginatedResult(pageItems, total, pg)
	return &result, nil
}

// Get retrieves a doc by its ID.
func (s *Store) Get(_ context.Context, id string) (*agent.Doc, error) {
	if err := agent.ValidateDocID(id); err != nil {
		return nil, err
	}

	filePath, err := s.docFilePath(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath) //nolint:gosec // path validated by docFilePath
	if err != nil {
		if os.IsNotExist(err) {
			return nil, agent.ErrDocNotFound
		}
		return nil, fmt.Errorf("filedoc: failed to read file %s: %w", filePath, err)
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("filedoc: failed to stat file %s: %w", filePath, err)
	}

	doc, err := parseDocFile(data, id)
	if err != nil {
		return nil, fmt.Errorf("filedoc: failed to parse doc %s: %w", id, err)
	}

	doc.CreatedAt = info.ModTime().UTC().Format(time.RFC3339)
	doc.UpdatedAt = info.ModTime().UTC().Format(time.RFC3339)

	return doc, nil
}

// Create creates a new doc file.
func (s *Store) Create(_ context.Context, id, content string) error {
	if err := agent.ValidateDocID(id); err != nil {
		return err
	}

	filePath, err := s.docFilePath(id)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); err == nil {
		return agent.ErrDocAlreadyExists
	}

	// Ensure parent directories exist.
	if err := os.MkdirAll(filepath.Dir(filePath), docDirPermissions); err != nil {
		return fmt.Errorf("filedoc: failed to create parent directories: %w", err)
	}

	data := marshalDocFile("", content)
	if err := fileutil.WriteFileAtomic(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("filedoc: failed to write file: %w", err)
	}

	return nil
}

// Update modifies an existing doc file.
func (s *Store) Update(_ context.Context, id, content string) error {
	if err := agent.ValidateDocID(id); err != nil {
		return err
	}

	filePath, err := s.docFilePath(id)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return agent.ErrDocNotFound
	}

	// Preserve existing title from frontmatter.
	existingData, err := os.ReadFile(filePath) //nolint:gosec // path validated
	if err != nil {
		return fmt.Errorf("filedoc: failed to read existing file: %w", err)
	}
	existingDoc, err := parseDocFile(existingData, id)
	if err != nil {
		existingDoc = &agent.Doc{}
	}

	title := existingDoc.Title
	if title == titleFromID(id) {
		title = "" // Don't write default title to frontmatter.
	}

	data := marshalDocFile(title, content)
	if err := fileutil.WriteFileAtomic(filePath, data, filePermissions); err != nil {
		return fmt.Errorf("filedoc: failed to write file: %w", err)
	}

	return nil
}

// Delete removes a doc file and cleans up empty parent directories.
func (s *Store) Delete(_ context.Context, id string) error {
	if err := agent.ValidateDocID(id); err != nil {
		return err
	}

	filePath, err := s.docFilePath(id)
	if err != nil {
		return err
	}

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return agent.ErrDocNotFound
	}

	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("filedoc: failed to delete file: %w", err)
	}

	// Clean up empty parent directories up to baseDir.
	s.cleanEmptyParents(filepath.Dir(filePath))

	return nil
}

// Rename moves a doc from oldID to newID.
func (s *Store) Rename(_ context.Context, oldID, newID string) error {
	if err := agent.ValidateDocID(oldID); err != nil {
		return err
	}
	if err := agent.ValidateDocID(newID); err != nil {
		return err
	}

	oldPath, err := s.docFilePath(oldID)
	if err != nil {
		return err
	}
	newPath, err := s.docFilePath(newID)
	if err != nil {
		return err
	}

	if _, err := os.Stat(oldPath); os.IsNotExist(err) {
		return agent.ErrDocNotFound
	}
	if _, err := os.Stat(newPath); err == nil {
		return agent.ErrDocAlreadyExists
	}

	if err := os.MkdirAll(filepath.Dir(newPath), docDirPermissions); err != nil {
		return fmt.Errorf("filedoc: failed to create target directories: %w", err)
	}

	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("filedoc: failed to rename file: %w", err)
	}

	s.cleanEmptyParents(filepath.Dir(oldPath))

	return nil
}

// Search searches all docs for the given query pattern.
func (s *Store) Search(_ context.Context, query string) ([]*agent.DocSearchResult, error) {
	var results []*agent.DocSearchResult

	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		data, err := os.ReadFile(path) //nolint:gosec // path constructed from baseDir
		if err != nil {
			return nil
		}

		matches, err := grep.Grep(data, query, grep.DefaultGrepOptions)
		if err != nil {
			return nil // no match or error — skip
		}

		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return nil
		}
		id := strings.TrimSuffix(relPath, ".md")

		doc, parseErr := parseDocFile(data, id)
		title := id
		if parseErr == nil {
			title = doc.Title
		}

		results = append(results, &agent.DocSearchResult{
			ID:      id,
			Title:   title,
			Matches: matches,
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filedoc: failed to search: %w", err)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].ID < results[j].ID
	})

	return results, nil
}

// buildTree builds a tree of DocTreeNode from the filesystem.
func (s *Store) buildTree() ([]*agent.DocTreeNode, error) {
	root := make(map[string]*agent.DocTreeNode)
	var topLevel []*agent.DocTreeNode

	err := filepath.WalkDir(s.baseDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == s.baseDir {
			return nil
		}

		relPath, err := filepath.Rel(s.baseDir, path)
		if err != nil {
			return nil
		}

		if d.IsDir() {
			node := &agent.DocTreeNode{
				ID:       relPath,
				Name:     d.Name(),
				Type:     "directory",
				Children: []*agent.DocTreeNode{},
			}
			root[relPath] = node

			parentDir := filepath.Dir(relPath)
			if parentDir == "." {
				topLevel = append(topLevel, node)
			} else if parent, ok := root[parentDir]; ok {
				parent.Children = append(parent.Children, node)
			}
			return nil
		}

		if filepath.Ext(path) != ".md" {
			return nil
		}

		id := strings.TrimSuffix(relPath, ".md")

		data, readErr := os.ReadFile(path) //nolint:gosec // path constructed from baseDir
		var title string
		if readErr == nil {
			if doc, parseErr := parseDocFile(data, id); parseErr == nil {
				title = doc.Title
			}
		}
		if title == "" {
			title = titleFromID(id)
		}

		node := &agent.DocTreeNode{
			ID:    id,
			Name:  strings.TrimSuffix(d.Name(), ".md"),
			Title: title,
			Type:  "file",
		}

		parentDir := filepath.Dir(relPath)
		if parentDir == "." {
			topLevel = append(topLevel, node)
		} else if parent, ok := root[parentDir]; ok {
			parent.Children = append(parent.Children, node)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("filedoc: failed to build tree: %w", err)
	}

	sortTreeNodes(topLevel)

	return topLevel, nil
}

// sortTreeNodes sorts nodes: directories first, then files, alphabetically within each group.
func sortTreeNodes(nodes []*agent.DocTreeNode) {
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "directory"
		}
		return nodes[i].Name < nodes[j].Name
	})
	for _, node := range nodes {
		if len(node.Children) > 0 {
			sortTreeNodes(node.Children)
		}
	}
}

// cleanEmptyParents removes empty parent directories up to baseDir.
func (s *Store) cleanEmptyParents(dir string) {
	for dir != s.baseDir && strings.HasPrefix(dir, s.baseDir) {
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) > 0 {
			return
		}
		if err := os.Remove(dir); err != nil {
			return
		}
		dir = filepath.Dir(dir)
	}
}
