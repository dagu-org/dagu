package filedag

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/grep"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

var _ models.DAGStore = (*Storage)(nil)

// Option is a functional option for configuring the DAG repository
type Option func(*Options)

// Options contains configuration options for the DAG repository
type Options struct {
	FlagsBaseDir string                        // Base directory for flag store
	FileCache    *fileutil.Cache[*digraph.DAG] // Optional cache for DAG objects
	SearchPaths  []string                      // Additional search paths for DAG files
}

// WithFileCache returns a DAGRepositoryOption that sets the file cache for DAG objects
func WithFileCache(cache *fileutil.Cache[*digraph.DAG]) Option {
	return func(o *Options) {
		o.FileCache = cache
	}
}

// WithFlagsBaseDir returns a DAGRepositoryOption that sets the base directory for flag store
func WithFlagsBaseDir(dir string) Option {
	return func(o *Options) {
		o.FlagsBaseDir = dir
	}
}

// WithSearchPaths returns a DAGRepositoryOption that sets additional search paths for DAG files
func WithSearchPaths(paths []string) Option {
	return func(o *Options) {
		o.SearchPaths = paths
	}
}

// New creates a new DAG store implementation using the local filesystem
func New(baseDir string, opts ...Option) models.DAGStore {
	options := &Options{}
	for _, opt := range opts {
		opt(options)
	}
	if options.FlagsBaseDir == "" {
		options.FlagsBaseDir = filepath.Join(baseDir, "flags")
	}
	uniqSearchPaths := make(map[string]struct{})
	uniqSearchPaths[baseDir] = struct{}{}
	uniqSearchPaths["."] = struct{}{}
	for _, path := range options.SearchPaths {
		uniqSearchPaths[path] = struct{}{}
	}
	searchPaths := make([]string, 0, len(uniqSearchPaths))
	for path := range uniqSearchPaths {
		searchPaths = append(searchPaths, path)
	}

	return &Storage{
		baseDir:      baseDir,
		flagsBaseDir: options.FlagsBaseDir,
		fileCache:    options.FileCache,
		searchPaths:  searchPaths,
	}
}

// Storage implements the DAGRepository interface using the local filesystem
type Storage struct {
	baseDir      string                        // Base directory for DAG storage
	flagsBaseDir string                        // Base directory for flag store
	fileCache    *fileutil.Cache[*digraph.DAG] // Optional cache for DAG objects
	searchPaths  []string                      // Additional search paths for DAG files
}

// GetMetadata retrieves the metadata of a DAG by its name.
func (store *Storage) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath, err := store.locateDAG(name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate DAG %s in search paths (%v): %w", name, store.searchPaths, err)
	}

	var dag *digraph.DAG
	if store.fileCache == nil {
		dag, err = digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	} else {
		dag, err = store.fileCache.LoadLatest(filePath, func() (*digraph.DAG, error) {
			return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
		})
	}

	if err != nil {
		return nil, err
	}

	// Set the fileName to include the prefix
	dag.SetFileName(name)

	return dag, nil
}

// GetDetails retrieves the details of a DAG by its name.
func (store *Storage) GetDetails(ctx context.Context, name string, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	filePath, err := store.locateDAG(name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	var loadOpts []digraph.LoadOption
	loadOpts = append(loadOpts, opts...)
	loadOpts = append(loadOpts, digraph.WithoutEval())

	dag, err := digraph.Load(ctx, filePath, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load DAG %s: %w", name, err)
	}

	// Set the fileName to include the prefix
	dag.SetFileName(name)

	return dag, nil
}

// GetSpec retrieves the specification of a DAG by its name.
func (store *Storage) GetSpec(_ context.Context, name string) (string, error) {
	filePath, err := store.locateDAG(name)
	if err != nil {
		return "", models.ErrDAGNotFound
	}
	dat, err := os.ReadFile(filePath) // nolint:gosec
	if err != nil {
		return "", err
	}
	return string(dat), nil
}

// FileMode used for newly created DAG files
const defaultPerm os.FileMode = 0600

func (store *Storage) LoadSpec(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	// Validate the spec before saving it.
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// UpdateSpec updates the specification of a DAG by its name.
func (store *Storage) UpdateSpec(ctx context.Context, name string, spec []byte) error {
	// Validate the spec before saving it.
	dag, err := digraph.LoadYAML(ctx, spec, digraph.WithoutEval())
	if err != nil {
		return err
	}
	if err := dag.Validate(); err != nil {
		return err
	}
	filePath, err := store.locateDAG(name)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return err
	}
	if store.fileCache != nil {
		store.fileCache.Invalidate(filePath)
	}
	return nil
}

// Create creates a new DAG with the given name and specification.
func (store *Storage) Create(ctx context.Context, name string, spec []byte) error {
	if err := store.ensureDirExist(); err != nil {
		return fmt.Errorf("failed to create DAGs directory %s: %w", store.baseDir, err)
	}

	// Validate the spec before saving it.
	dag, err := digraph.LoadYAML(ctx, spec, digraph.WithoutEval())
	if err != nil {
		return err
	}
	if err := dag.Validate(); err != nil {
		return err
	}

	filePath := store.generateFilePath(name)
	if fileExists(filePath) {
		return models.ErrDAGAlreadyExists
	}

	// Create parent directories if needed
	parentDir := filepath.Dir(filePath)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return fmt.Errorf("failed to create parent directories for DAG %s: %w", name, err)
	}

	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return fmt.Errorf("failed to write DAG %s: %w", name, err)
	}
	return nil
}

// Delete deletes a DAG by its name.
func (store *Storage) Delete(_ context.Context, name string) error {
	filePath, err := store.locateDAG(name)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if err := os.Remove(filePath); err != nil {
		return err
	}
	if store.fileCache != nil {
		store.fileCache.Invalidate(filePath)
	}
	return nil
}

// ensureDirExist ensures that the base directory exists.
func (store *Storage) ensureDirExist() error {
	if !fileExists(store.baseDir) {
		if err := os.MkdirAll(store.baseDir, 0750); err != nil {
			return err
		}
	}
	return nil
}

// List lists DAGs with pagination support.
func (store *Storage) List(ctx context.Context, opts models.ListDAGsOptions) (models.PaginatedResult[*digraph.DAG], []string, error) {
	// Use ListWithPrefix with empty prefix for backward compatibility
	result, _, errs, err := store.ListWithPrefix(ctx, "", opts)
	return result, errs, err
}

// ListWithPrefix lists DAGs within a specific prefix/directory with pagination support.
// nolint: revive
func (store *Storage) ListWithPrefix(ctx context.Context, prefix string, opts models.ListDAGsOptions) (models.PaginatedResult[*digraph.DAG], []string, []string, error) {
	var dags []*digraph.DAG
	var errList []string
	var subdirs []string
	var totalCount int

	if opts.Paginator == nil {
		p := models.DefaultPaginator()
		opts.Paginator = &p
	}

	// Normalize the prefix
	prefix = fileutil.NormalizeDAGPath(prefix)

	// Determine the directory to read
	targetDir := store.baseDir
	if prefix != "" {
		targetDir = filepath.Join(store.baseDir, prefix)
	}

	entries, err := os.ReadDir(targetDir)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty result if directory doesn't exist
			return models.NewPaginatedResult(dags, totalCount, *opts.Paginator), subdirs, errList, nil
		}
		errList = append(errList, fmt.Sprintf("failed to read directory %s: %s", targetDir, err))
		return models.NewPaginatedResult(dags, totalCount, *opts.Paginator), subdirs, errList, err
	}

	// Collect subdirectories
	for _, entry := range entries {
		if entry.IsDir() {
			subdirs = append(subdirs, entry.Name())
		}
	}

	// Process DAG files
	for _, entry := range entries {
		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			continue
		}

		baseName := path.Base(entry.Name())
		nameWithoutExt := strings.TrimSuffix(baseName, path.Ext(baseName))

		// Create the full DAG name with prefix
		dagName := fileutil.JoinDAGPath(prefix, nameWithoutExt)

		if opts.Name != "" && opts.Tag == "" {
			// If tag is not provided, check before reading the file to avoid
			// unnecessary file read and parsing.
			if !containsSearchText(nameWithoutExt, opts.Name) && !containsSearchText(dagName, opts.Name) {
				// Return early if the name does not match the search text.
				continue
			}
		}

		// Read the file and parse the DAG.
		dag, err := store.GetMetadata(ctx, dagName)
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
			continue
		}

		if opts.Name != "" && !containsSearchText(nameWithoutExt, opts.Name) && !containsSearchText(dagName, opts.Name) {
			continue
		}

		if opts.Tag != "" && !containsTag(dag.Tags, opts.Tag) {
			continue
		}

		totalCount++
		if totalCount > opts.Paginator.Offset() && len(dags) < opts.Paginator.Limit() {
			dags = append(dags, dag)
		}
	}

	result := models.NewPaginatedResult(
		dags, totalCount, *opts.Paginator,
	)

	return result, subdirs, errList, nil
}

// Grep searches for a pattern in all DAGs.
func (store *Storage) Grep(ctx context.Context, pattern string) (
	ret []*models.GrepDAGsResult, errs []string, err error,
) {
	if pattern == "" {
		// return empty result if pattern is empty
		return nil, nil, nil
	}
	if err = store.ensureDirExist(); err != nil {
		errs = append(
			errs, fmt.Sprintf("failed to create DAGs directory %s", store.baseDir),
		)
		return
	}

	// Recursively search all directories
	err = store.grepRecursive(ctx, store.baseDir, "", pattern, &ret, &errs)

	return ret, errs, err
}

// grepRecursive recursively searches for a pattern in DAG files
func (store *Storage) grepRecursive(ctx context.Context, dir string, prefix string, pattern string,
	results *[]*models.GrepDAGsResult, errs *[]string) error {

	entries, err := os.ReadDir(dir)
	if err != nil {
		logger.Error(ctx, "Failed to read directory", "dir", dir, "err", err)
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively search subdirectories
			subPrefix := fileutil.JoinDAGPath(prefix, entry.Name())
			subDir := filepath.Join(dir, entry.Name())
			if err := store.grepRecursive(ctx, subDir, subPrefix, pattern, results, errs); err != nil {
				*errs = append(*errs, fmt.Sprintf("failed to search directory %s: %s", subDir, err))
			}
		} else if fileutil.IsYAMLFile(entry.Name()) {
			filePath := filepath.Join(dir, entry.Name())
			dat, err := os.ReadFile(filePath) //nolint:gosec
			if err != nil {
				logger.Error(ctx, "Failed to read DAG file", "file", filePath, "err", err)
				continue
			}
			matches, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), grep.DefaultGrepOptions)
			if err != nil {
				if errors.Is(err, grep.ErrNoMatch) {
					continue
				}
				*errs = append(*errs, fmt.Sprintf("grep %s failed: %s", filePath, err))
				continue
			}

			nameWithoutExt := strings.TrimSuffix(entry.Name(), path.Ext(entry.Name()))
			dagName := fileutil.JoinDAGPath(prefix, nameWithoutExt)

			dag, err := digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
			if err != nil {
				*errs = append(*errs, fmt.Sprintf("check %s failed: %s", filePath, err))
				continue
			}
			*results = append(*results, &models.GrepDAGsResult{
				Name:    dagName,
				DAG:     dag,
				Matches: matches,
			})
		}
	}
	return nil
}

// ToggleSuspend toggles the suspension state of a DAG.
func (store *Storage) ToggleSuspend(ctx context.Context, id string, suspend bool) error {
	if suspend {
		return store.createFlag(fileName(id))
	} else if store.IsSuspended(ctx, id) {
		return store.deleteFlag(fileName(id))
	}
	return nil
}

// IsSuspended checks if a DAG is suspended.
func (store *Storage) IsSuspended(_ context.Context, id string) bool {
	return store.flagExists(fileName(id))
}

func fileName(id string) string {
	return fmt.Sprintf("%s.suspend", normalizeFilename(id, "-"))
}

// Rename renames a DAG from oldID to newID.
func (store *Storage) Rename(_ context.Context, oldID, newID string) error {
	oldFilePath, err := store.locateDAG(oldID)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", oldID, err)
	}
	newFilePath := store.generateFilePath(newID)
	if fileExists(newFilePath) {
		return models.ErrDAGAlreadyExists
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(newFilePath)
	if err := os.MkdirAll(parentDir, 0750); err != nil {
		return fmt.Errorf("failed to create parent directory for %s: %w", newID, err)
	}

	return os.Rename(oldFilePath, newFilePath)
}

// generateFilePath generates the file path for a DAG by its name.
func (store *Storage) generateFilePath(name string) string {
	// Normalize the DAG path
	name = fileutil.NormalizeDAGPath(name)

	// If the name already contains a separator and is an absolute path, use it
	if filepath.IsAbs(name) {
		return fileutil.EnsureYAMLExtension(name)
	}

	// Otherwise, join with base directory
	filePath := fileutil.EnsureYAMLExtension(filepath.Join(store.baseDir, name))
	return filepath.Clean(filePath)
}

// locateDAG locates the DAG file by its name or path.
func (store *Storage) locateDAG(nameOrPath string) (string, error) {
	if strings.Contains(nameOrPath, string(filepath.Separator)) {
		foundPath, err := findDAGFile(nameOrPath)
		if err == nil {
			return foundPath, nil
		}
	}

	for _, dir := range store.searchPaths {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		candidatePath := filepath.Join(absDir, nameOrPath)
		foundPath, err := findDAGFile(candidatePath)
		if err == nil {
			return foundPath, nil
		}
	}

	// DAG not found
	return "", fmt.Errorf("DAG %s not found: %w", nameOrPath, os.ErrNotExist)
}

// TagList lists all unique tags from the DAGs.
func (store *Storage) TagList(ctx context.Context) ([]string, []string, error) {
	var (
		errList []string
		tagSet  = make(map[string]struct{})
	)

	// Recursively collect tags from all directories
	if err := store.collectTagsRecursive(ctx, store.baseDir, "", &tagSet, &errList); err != nil {
		errList = append(errList, fmt.Sprintf("failed to collect tags: %s", err))
		return nil, errList, err
	}

	tagList := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tagList = append(tagList, tag)
	}
	return tagList, errList, nil
}

// collectTagsRecursive recursively collects tags from DAG files
func (store *Storage) collectTagsRecursive(ctx context.Context, dir string, prefix string,
	tagSet *map[string]struct{}, errList *[]string) error {

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively collect from subdirectories
			subPrefix := fileutil.JoinDAGPath(prefix, entry.Name())
			subDir := filepath.Join(dir, entry.Name())
			if err := store.collectTagsRecursive(ctx, subDir, subPrefix, tagSet, errList); err != nil {
				*errList = append(*errList, fmt.Sprintf("failed to read directory %s: %s", subDir, err))
			}
		} else if fileutil.IsYAMLFile(entry.Name()) {
			baseName := path.Base(entry.Name())
			nameWithoutExt := strings.TrimSuffix(baseName, path.Ext(baseName))
			dagName := fileutil.JoinDAGPath(prefix, nameWithoutExt)

			parsedDAG, err := store.GetMetadata(ctx, dagName)
			if err != nil {
				*errList = append(*errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
				continue
			}

			for _, tag := range parsedDAG.Tags {
				(*tagSet)[tag] = struct{}{}
			}
		}
	}

	return nil
}

// CreateFlag creates the given file.
func (store *Storage) createFlag(file string) error {
	_ = os.MkdirAll(store.flagsBaseDir, flagPermission)
	return os.WriteFile(path.Join(store.flagsBaseDir, file), []byte{}, flagPermission)
}

// flagExists returns true if the given file exists.
func (store *Storage) flagExists(file string) bool {
	_ = os.MkdirAll(store.flagsBaseDir, flagPermission)
	_, err := os.Stat(path.Join(store.flagsBaseDir, file))
	return err == nil
}

// deleteFlag deletes the given file.
func (store *Storage) deleteFlag(file string) error {
	_ = os.MkdirAll(store.flagsBaseDir, flagPermission)
	return os.Remove(path.Join(store.flagsBaseDir, file))
}

// flagPermission is the default file permission for newly created files.
var flagPermission os.FileMode = 0750

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

// normalizeFilename normalizes a filename by replacing reserved characters with a replacement string.
func normalizeFilename(str, replacement string) string {
	s := filenameReservedRegex.ReplaceAllString(str, replacement)
	s = filenameReservedWindowsNamesRegex.ReplaceAllString(s, replacement)
	return strings.ReplaceAll(s, " ", replacement)
}

// https://github.com/sindresorhus/filename-reserved-regex/blob/master/index.js
var (
	filenameReservedRegex = regexp.MustCompile(
		`[<>:"/\\|?*\x00-\x1F]`,
	)
	filenameReservedWindowsNamesRegex = regexp.MustCompile(
		`(?i)^(con|prn|aux|nul|com[0-9]|lpt[0-9])$`,
	)
)

// findDAGFile finds the DAG file with the given file name.
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
