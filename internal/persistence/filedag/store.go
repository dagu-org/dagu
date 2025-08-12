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
	"sort"
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
	if store.fileCache == nil {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	}
	return store.fileCache.LoadLatest(filePath, func() (*digraph.DAG, error) {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	})
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

	dat, err := digraph.Load(ctx, filePath, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to load DAG %s: %w", name, err)
	}
	return dat, nil
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
func (store *Storage) Create(_ context.Context, name string, spec []byte) error {
	if err := store.ensureDirExist(); err != nil {
		return fmt.Errorf("failed to create DAGs directory %s: %w", store.baseDir, err)
	}
	filePath := store.generateFilePath(name)
	if fileExists(filePath) {
		return models.ErrDAGAlreadyExists
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
	var allDags []*digraph.DAG
	var errList []string

	if opts.Paginator == nil {
		p := models.DefaultPaginator()
		opts.Paginator = &p
	}

	entries, err := os.ReadDir(store.baseDir)
	if err != nil {
		errList = append(errList, fmt.Sprintf("failed to read directory %s: %s", store.baseDir, err))
		return models.NewPaginatedResult([]*digraph.DAG{}, 0, *opts.Paginator), errList, err
	}

	// First, collect all matching DAGs
	for _, entry := range entries {
		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			continue
		}

		baseName := path.Base(entry.Name())
		dagName := strings.TrimSuffix(baseName, path.Ext(baseName))
		if opts.Name != "" && opts.Tag == "" {
			// If tag is not provided, check before reading the file to avoid
			// unnecessary file read and parsing.
			if !containsSearchText(dagName, opts.Name) {
				// Return early if the name does not match the search text.
				continue
			}
		}

		// Read the file and parse the DAG.
		// Use WithAllowBuildErrors to include DAGs with errors in the list
		filePath := filepath.Join(store.baseDir, entry.Name())
		dag, err := digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval(), digraph.WithAllowBuildErrors())
		if err != nil {
			// If it completely fails to load, skip it
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
			continue
		}

		if opts.Name != "" && !containsSearchText(dagName, opts.Name) {
			continue
		}

		if opts.Tag != "" && !containsTag(dag.Tags, opts.Tag) {
			continue
		}

		allDags = append(allDags, dag)
	}

	// Sort DAGs by name (the only supported sort field)
	sort.Slice(allDags, func(i, j int) bool {
		// Default to ascending order
		ascending := opts.Order != "desc"

		// Always sort by name (case-insensitive)
		if ascending {
			return strings.ToLower(allDags[i].Name) < strings.ToLower(allDags[j].Name)
		}
		return strings.ToLower(allDags[i].Name) > strings.ToLower(allDags[j].Name)
	})

	totalCount := len(allDags)

	// Apply pagination
	var paginatedDags []*digraph.DAG
	start := opts.Paginator.Offset()
	end := start + opts.Paginator.Limit()

	if start < len(allDags) {
		if end > len(allDags) {
			end = len(allDags)
		}
		paginatedDags = allDags[start:end]
	}

	result := models.NewPaginatedResult(
		paginatedDags, totalCount, *opts.Paginator,
	)

	return result, errList, nil
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

	entries, err := os.ReadDir(store.baseDir)
	if err != nil {
		logger.Error(ctx, "Failed to read directory", "dir", store.baseDir, "err", err)
	}

	for _, entry := range entries {
		if fileutil.IsYAMLFile(entry.Name()) {
			filePath := filepath.Join(store.baseDir, entry.Name())
			dat, err := os.ReadFile(filePath) //nolint:gosec
			if err != nil {
				logger.Error(ctx, "Failed to read DAG file", "file", entry.Name(), "err", err)
				continue
			}
			matches, err := grep.Grep(dat, fmt.Sprintf("(?i)%s", pattern), grep.DefaultGrepOptions)
			if err != nil {
				if errors.Is(err, grep.ErrNoMatch) {
					continue
				}
				errs = append(errs, fmt.Sprintf("grep %s failed: %s", entry.Name(), err))
				continue
			}
			dag, err := digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
			if err != nil {
				errs = append(errs, fmt.Sprintf("check %s failed: %s", entry.Name(), err))
				continue
			}
			ret = append(ret, &models.GrepDAGsResult{
				Name:    strings.TrimSuffix(entry.Name(), path.Ext(entry.Name())),
				DAG:     dag,
				Matches: matches,
			})
		}
	}
	return ret, errs, nil
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
	return os.Rename(oldFilePath, newFilePath)
}

// generateFilePath generates the file path for a DAG by its name.
func (store *Storage) generateFilePath(name string) string {
	if strings.Contains(name, string(filepath.Separator)) {
		filePath, err := filepath.Abs(name)
		if err == nil {
			return filePath
		}
	}
	filePath := fileutil.EnsureYAMLExtension(path.Join(store.baseDir, name))
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

	entries, err := os.ReadDir(store.baseDir)
	if err != nil {
		errList = append(errList, fmt.Sprintf("failed to read directory %s: %s", store.baseDir, err))
		return nil, errList, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			continue
		}

		baseName := path.Base(entry.Name())
		dagName := strings.TrimSuffix(baseName, path.Ext(baseName))

		parsedDAG, err := store.GetMetadata(ctx, dagName)
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", entry.Name(), err))
			continue
		}

		for _, tag := range parsedDAG.Tags {
			tagSet[tag] = struct{}{}
		}
	}

	tagList := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tagList = append(tagList, tag)
	}
	return tagList, errList, nil
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
