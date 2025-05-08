package localdag

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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

var _ models.DAGRepository = (*storage)(nil)

// Option is a functional option for configuring the DAG repository
type Option func(*Options)

// Options contains configuration options for the DAG repository
type Options struct {
	FlagsBaseDir string                        // Base directory for flag storage
	FileCache    *fileutil.Cache[*digraph.DAG] // Optional cache for DAG objects
	SearchPaths  []string                      // Additional search paths for DAG files
}

// WithFileCache returns a DAGRepositoryOption that sets the file cache for DAG objects
func WithFileCache(cache *fileutil.Cache[*digraph.DAG]) Option {
	return func(o *Options) {
		o.FileCache = cache
	}
}

// WithFlagsBaseDir returns a DAGRepositoryOption that sets the base directory for flag storage
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

// storage implements the DAGRepository interface using the local filesystem
type storage struct {
	baseDir      string                        // Base directory for DAG storage
	flagsBaseDir string                        // Base directory for flag storage
	fileCache    *fileutil.Cache[*digraph.DAG] // Optional cache for DAG objects
	searchPaths  []string                      // Additional search paths for DAG files
}

// New creates a new DAG store implementation using the local filesystem
func New(baseDir string, opts ...Option) models.DAGRepository {
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

	return &storage{
		baseDir:      baseDir,
		flagsBaseDir: options.FlagsBaseDir,
		fileCache:    options.FileCache,
		searchPaths:  searchPaths,
	}
}

// GetMetadata retrieves the metadata of a DAG by its name.
func (sto *storage) GetMetadata(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath, err := sto.locateDAG(name)
	if err != nil {
		return nil, fmt.Errorf("failed to locate DAG %s in search paths (%v): %w", name, sto.searchPaths, err)
	}
	if sto.fileCache == nil {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	}
	return sto.fileCache.LoadLatest(filePath, func() (*digraph.DAG, error) {
		return digraph.Load(ctx, filePath, digraph.OnlyMetadata(), digraph.WithoutEval())
	})
}

// GetDetails retrieves the details of a DAG by its name.
func (sto *storage) GetDetails(ctx context.Context, name string) (*digraph.DAG, error) {
	filePath, err := sto.locateDAG(name)
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
func (sto *storage) GetSpec(_ context.Context, name string) (string, error) {
	filePath, err := sto.locateDAG(name)
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

func (sto *storage) LoadSpec(ctx context.Context, spec []byte, opts ...digraph.LoadOption) (*digraph.DAG, error) {
	// Validate the spec before saving it.
	opts = append(slices.Clone(opts), digraph.WithoutEval())
	return digraph.LoadYAML(ctx, spec, opts...)
}

// UpdateSpec updates the specification of a DAG by its name.
func (sto *storage) UpdateSpec(ctx context.Context, name string, spec []byte) error {
	// Validate the spec before saving it.
	dag, err := digraph.LoadYAML(ctx, spec, digraph.WithoutEval())
	if err != nil {
		return err
	}
	if err := dag.Validate(); err != nil {
		return err
	}
	filePath, err := sto.locateDAG(name)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", name, err)
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return err
	}
	if sto.fileCache != nil {
		sto.fileCache.Invalidate(filePath)
	}
	return nil
}

// Create creates a new DAG with the given name and specification.
func (d *storage) Create(_ context.Context, name string, spec []byte) error {
	if err := d.ensureDirExist(); err != nil {
		return fmt.Errorf("failed to create DAGs directory %s: %w", d.baseDir, err)
	}
	filePath := d.generateFilePath(name)
	if fileExists(filePath) {
		return models.ErrDAGAlreadyExists
	}
	if err := os.WriteFile(filePath, spec, defaultPerm); err != nil {
		return fmt.Errorf("failed to write DAG %s: %w", name, err)
	}
	return nil
}

// Delete deletes a DAG by its name.
func (d *storage) Delete(_ context.Context, name string) error {
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
func (d *storage) ensureDirExist() error {
	if !fileExists(d.baseDir) {
		if err := os.MkdirAll(d.baseDir, 0750); err != nil {
			return err
		}
	}
	return nil
}

// List lists DAGs with pagination support.
func (d *storage) List(ctx context.Context, opts models.ListOptions) (models.PaginatedResult[*digraph.DAG], []string, error) {
	var dags []*digraph.DAG
	var errList []string
	var totalCount int

	if opts.Paginator == nil {
		p := models.DefaultPaginator()
		opts.Paginator = &p
	}

	err := filepath.WalkDir(d.baseDir, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || !fileutil.IsYAMLFile(entry.Name()) {
			return nil
		}

		baseName := path.Base(entry.Name())
		dagName := strings.TrimSuffix(baseName, path.Ext(baseName))
		if opts.Name != "" && opts.Tag == "" {
			// If tag is not provided, check before reading the file to avoid
			// unnecessary file read and parsing.
			if !containsSearchText(dagName, opts.Name) {
				// Return early if the name does not match the search text.
				return nil
			}
		}

		// Read the file and parse the DAG.
		dag, err := d.GetMetadata(ctx, dagName)
		if err != nil {
			errList = append(errList, fmt.Sprintf("reading %s failed: %s", dagName, err))
			return nil
		}

		if opts.Name != "" && !containsSearchText(dagName, opts.Name) {
			return nil
		}

		if opts.Tag != "" && !containsTag(dag.Tags, opts.Tag) {
			return nil
		}

		totalCount++
		if totalCount > opts.Paginator.Offset() && len(dags) < opts.Paginator.Limit() {
			dags = append(dags, dag)
		}

		return nil
	})

	result := models.NewPaginatedResult(
		dags, totalCount, *opts.Paginator,
	)
	if err != nil {
		errList = append(errList, err.Error())
	}

	return result, errList, err
}

// Grep searches for a pattern in all DAGs.
func (d *storage) Grep(ctx context.Context, pattern string) (
	ret []*models.GrepResult, errs []string, err error,
) {
	if pattern == "" {
		// return empty result if pattern is empty
		return nil, nil, nil
	}
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
			ret = append(ret, &models.GrepResult{
				Name:    strings.TrimSuffix(entry.Name(), path.Ext(entry.Name())),
				DAG:     dag,
				Matches: matches,
			})
		}
	}
	return ret, errs, nil
}

func (d *storage) ToggleSuspend(ctx context.Context, id string, suspend bool) error {
	if suspend {
		return d.createFlag(fileName(id))
	} else if d.IsSuspended(ctx, id) {
		return d.deleteFlag(fileName(id))
	}
	return nil
}

func (d *storage) IsSuspended(_ context.Context, id string) bool {
	return d.flagExists(fileName(id))
}

func fileName(id string) string {
	return fmt.Sprintf("%s.suspend", normalizeFilename(id, "-"))
}

// Rename renames a DAG from oldID to newID.
func (d *storage) Rename(_ context.Context, oldID, newID string) error {
	oldFilePath, err := d.locateDAG(oldID)
	if err != nil {
		return fmt.Errorf("failed to locate DAG %s: %w", oldID, err)
	}
	newFilePath := d.generateFilePath(newID)
	if fileExists(newFilePath) {
		return models.ErrDAGAlreadyExists
	}
	return os.Rename(oldFilePath, newFilePath)
}

// generateFilePath generates the file path for a DAG by its name.
func (d *storage) generateFilePath(name string) string {
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
func (d *storage) locateDAG(nameOrPath string) (string, error) {
	if strings.Contains(nameOrPath, string(filepath.Separator)) {
		foundPath, err := findDAGFile(nameOrPath)
		if err == nil {
			return foundPath, nil
		}
	}

	for _, dir := range d.searchPaths {
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
func (d *storage) TagList(ctx context.Context) ([]string, []string, error) {
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
			return nil
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

// CreateFlag creates the given file.
func (s *storage) createFlag(file string) error {
	_ = os.MkdirAll(s.flagsBaseDir, flagPermission)
	return os.WriteFile(path.Join(s.flagsBaseDir, file), []byte{}, flagPermission)
}

// flagExists returns true if the given file exists.
func (s *storage) flagExists(file string) bool {
	_ = os.MkdirAll(s.flagsBaseDir, flagPermission)
	_, err := os.Stat(path.Join(s.flagsBaseDir, file))
	return err == nil
}

// deleteFlag deletes the given file.
func (s *storage) deleteFlag(file string) error {
	_ = os.MkdirAll(s.flagsBaseDir, flagPermission)
	return os.Remove(path.Join(s.flagsBaseDir, file))
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
