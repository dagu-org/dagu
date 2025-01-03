package local

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/persistence"
	"github.com/dagu-org/dagu/internal/persistence/filecache"
	"github.com/dagu-org/dagu/internal/persistence/grep"
)

var _ persistence.DAGStore = (*dagStoreImpl)(nil)

type DAGStoreOption func(*DAGStoreOptions)

type DAGStoreOptions struct {
	FileCache *filecache.Cache[*digraph.DAG]
}

func WithFileCache(cache *filecache.Cache[*digraph.DAG]) DAGStoreOption {
	return func(o *DAGStoreOptions) {
		o.FileCache = cache
	}
}

type dagStoreImpl struct {
	baseDir   string
	fileCache *filecache.Cache[*digraph.DAG]
}

func NewDAGStore(dir string, opts ...DAGStoreOption) persistence.DAGStore {
	options := &DAGStoreOptions{}
	for _, opt := range opts {
		opt(options)
	}

	return &dagStoreImpl{
		baseDir:   dir,
		fileCache: options.FileCache,
	}
}

// GetMetadata retrieves the metadata of a DAG by its name.
func (d *dagStoreImpl) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath, err := d.locateDAG(name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if d.fileCache == nil {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	}
	return d.fileCache.LoadLatest(filePath, func() (*digraph.DAG, error) {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	})
}

// GetDetails retrieves the details of a DAG by its name.
func (d *dagStoreImpl) GetDetails(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath, err := d.locateDAG(name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	dat, err := digraph.Load(ctx, filePath, digraph.WithoutEval())
	if err != nil {
		return nil, fmt.Errorf("failed to load DAG %s: %w", name, err)
	}
	return dat, nil
}

// GetSpec retrieves the specification of a DAG by its name.
func (d *dagStoreImpl) GetSpec(_ context.Context, name string) (string, error) {
	filePath, err := d.locateDAG(name)
	if err != nil {
		return "", fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	dat, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

// TODO: use 0600 // nolint: gosec
const defaultPerm os.FileMode = 0744

// UpdateSpec updates the specification of a DAG by its name.
func (d *dagStoreImpl) UpdateSpec(ctx context.Context, name string, spec []byte) error {
	// Validate the spec before saving it.
	_, err := digraph.LoadYAML(ctx, spec, digraph.WithoutEval())
	if err != nil {
		return err
	}
	filePath, err := d.locateDAG(name)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return err
	}
	if d.fileCache != nil {
		d.fileCache.Invalidate(filePath)
	}
	return nil
}

var errDAGFileAlreadyExists = errors.New("the DAG file already exists")

// Create creates a new DAG with the given name and specification.
func (d *dagStoreImpl) Create(_ context.Context, name string, spec []byte) (string, error) {
	if err := d.ensureDirExist(); err != nil {
		return "", fmt.Errorf("failed to create DAGs directory %s: %w", d.baseDir, err)
	}
	filePath := d.generateFilePath(name)
	if fileExists(filePath) {
		return "", fmt.Errorf("%w: %s", errDAGFileAlreadyExists, filePath)
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return "", fmt.Errorf("failed to write DAG %s: %w", name, err)
	}
	return name, nil
}

// Delete deletes a DAG by its name.
func (d *dagStoreImpl) Delete(_ context.Context, name string) error {
	filePath, err := d.locateDAG(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if err := os.Remove(filePath); err != nil {
		return err
	}
	if d.fileCache != nil {
		d.fileCache.Invalidate(filePath)
	}
	return nil
}

// ensureDirExist ensures that the base directory exists.
func (d *dagStoreImpl) ensureDirExist() error {
	if !fileExists(d.baseDir) {
		if err := os.MkdirAll(d.baseDir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// ListPagination lists DAGs with pagination support.
func (d *dagStoreImpl) ListPagination(ctx context.Context, params persistence.DAGListPaginationArgs) (*persistence.DagListPaginationResult, error) {
	var (
		dagList []*digraph.DAG
		errList []string
		count   int
	)

	if err := filepath.WalkDir(d.baseDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			return nil
		}

		baseName := path.Base(entry.Name())
		dagName := strings.TrimSuffix(baseName, path.Ext(baseName))
		if params.Name != "" && params.Tag == "" {
			// If tag is not provided, check before reading the file to avoid
			// unnecessary file read and parsing.
			if !containsSearchText(dagName, params.Name) {
				// Return early if the name does not match the search text.
				return nil
			}
		}

		// Read the file and parse the DAG.
		parsedDAG, err := d.GetMetadata(ctx, dagName)
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
			return nil
		}

		if params.Name != "" && !containsSearchText(dagName, params.Name) {
			return nil
		}

		if params.Tag != "" && !containsTag(parsedDAG.Tags, params.Tag) {
			return nil
		}

		count++
		if count > (params.Page-1)*params.Limit && len(dagList) < params.Limit {
			dagList = append(dagList, parsedDAG)
		}

		return nil
	}); err != nil {
		return &persistence.DagListPaginationResult{
			DagList:   dagList,
			Count:     count,
			ErrorList: append(errList, err.Error()),
		}, err
	}

	return &persistence.DagListPaginationResult{
		DagList:   dagList,
		Count:     count,
		ErrorList: errList,
	}, nil
}

// List lists all DAGs.
func (d *dagStoreImpl) List(ctx context.Context) (ret []*digraph.DAG, errs []string, err error) {
	if err = d.ensureDirExist(); err != nil {
		errs = append(errs, err.Error())
		return
	}
	entries, err := os.ReadDir(d.baseDir)
	if err != nil {
		errs = append(errs, err.Error())
		return
	}
	for _, entry := range entries {
		if fileutil.IsYAMLFile(entry.Name()) {
			dat, err := d.GetMetadata(ctx, entry.Name())
			if err == nil {
				ret = append(ret, dat)
			} else {
				errs = append(errs, fmt.Sprintf(
					"reading %s failed: %s", entry.Name(), err),
				)
			}
		}
	}
	return ret, errs, nil
}

// Grep searches for a pattern in all DAGs.
func (d *dagStoreImpl) Grep(ctx context.Context, pattern string) (
	ret []*persistence.GrepResult, errs []string, err error,
) {
	if err = d.ensureDirExist(); err != nil {
		errs = append(
			errs, fmt.Sprintf("failed to create DAGs directory %s", d.baseDir),
		)
		return
	}

	entries, err := os.ReadDir(d.baseDir)
	if err != nil {
		logger.Error(ctx, "Failed to read directory", "dir", d.baseDir, "err", err)
	}

	for _, entry := range entries {
		if fileutil.IsYAMLFile(entry.Name()) {
			filePath := filepath.Join(d.baseDir, entry.Name())
			dat, err := os.ReadFile(filePath)
			if err != nil {
				logger.Error(ctx, "Failed to read DAG file", "file", entry.Name(), "err", err)
				continue
			}
			matches, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), grep.DefaultOptions)
			if err != nil {
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", entry.Name(), err))
				continue
			}
			dag, err := digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
			if err != nil {
				errs = append(errs, fmt.Sprintf("check %s failed: %s", entry.Name(), err))
				continue
			}
			ret = append(ret, &persistence.GrepResult{
				Name:    strings.TrimSuffix(entry.Name(), path.Ext(entry.Name())),
				DAG:     dag,
				Matches: matches,
			})
		}
	}
	return ret, errs, nil
}

// Rename renames a DAG from oldID to newID.
func (d *dagStoreImpl) Rename(_ context.Context, oldID, newID string) error {
	oldFilePath, err := d.locateDAG(oldID)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", oldID, err)
	}
	newFilePath := d.generateFilePath(newID)
	if fileExists(newFilePath) {
		return fmt.Errorf("%w: %s", errDAGFileAlreadyExists, newFilePath)
	}
	return os.Rename(oldFilePath, newFilePath)
}

// generateFilePath generates the file path for a DAG by its name.
func (d *dagStoreImpl) generateFilePath(name string) string {
	if strings.Contains(name, string(filepath.Separator)) {
		filePath, err := filepath.Abs(name)
		if err == nil {
			return filePath
		}
	}
	filePath := fileutil.EnsureYAMLExtension(path.Join(d.baseDir, name))
	return filepath.Clean(filePath)
}

// locateDAG locates the DAG file by its name or path.
func (d *dagStoreImpl) locateDAG(nameOrPath string) (string, error) {
	if strings.Contains(nameOrPath, string(filepath.Separator)) {
		foundPath, err := findDAGFile(nameOrPath)
		if err == nil {
			return foundPath, nil
		}
	}

	searchPaths := []string{".", d.baseDir}
	for _, dir := range searchPaths {
		candidatePath := filepath.Join(dir, nameOrPath)
		foundPath, err := findDAGFile(candidatePath)
		if err == nil {
			return foundPath, nil
		}
	}

	// DAG not found
	return "", fmt.Errorf("workflow %s not found: %w", nameOrPath, os.ErrNotExist)
}

// findDAGFile finds the sub workflow file with the given name.
func findDAGFile(name string) (string, error) {
	ext := path.Ext(name)
	switch ext {
	case ".yaml", ".yml":
		if fileutil.FileExists(name) {
			return filepath.Abs(name)
		}
	default:
		// try all supported extensions
		for _, ext := range fileutil.ValidYAMLExtensions {
			if fileutil.FileExists(name + ext) {
				return filepath.Abs(name + ext)
			}
		}
	}
	return "", fmt.Errorf("file %s not found: %w", name, os.ErrNotExist)
}

// TagList lists all unique tags from the DAGs.
func (d *dagStoreImpl) TagList(ctx context.Context) ([]string, []string, error) {
	var (
		errList []string
		tagSet  = make(map[string]struct{})
	)

	if err := filepath.WalkDir(d.baseDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			return nil
		}

		parsedDAG, err := d.GetMetadata(ctx, entry.Name())
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", entry.Name(), err))
		}

		for _, tag := range parsedDAG.Tags {
			tagSet[tag] = struct{}{}
		}

		return nil
	}); err != nil {
		return nil, append(errList, err.Error()), err
	}

	tagList := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tagList = append(tagList, tag)
	}
	return tagList, errList, nil
}

// containsSearchText checks if the text contains the search string (case-insensitive).
func containsSearchText(text string, search string) bool {
	return strings.Contains(strings.ToLower(text), strings.ToLower(search))
}

// containsTag checks if the tags contain the search tag (case-insensitive).
func containsTag(tags []string, searchTag string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, searchTag) {
			return true
		}
	}

	return false
}

// fileExists checks if a file exists.
func fileExists(file string) bool {
	_, err := os.Stat(file)
	return !os.IsNotExist(err)
}
