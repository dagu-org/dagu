package localhistory

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/digraph/scheduler"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/models"
)

// Error definitions for common issues
var (
	ErrDAGRunIDEmpty  = errors.New("DAG-run ID is empty")
	ErrTooManyResults = errors.New("too many results found")
)

var _ models.DAGRunStore = (*Store)(nil)

// Store manages DAGs status files in local Store with high performance and reliability.
type Store struct {
	baseDir           string                                // Base directory for all status files
	latestStatusToday bool                                  // Whether to only return today's status
	cache             *fileutil.Cache[*models.DAGRunStatus] // Optional cache for read operations
	maxWorkers        int                                   // Maximum number of parallel workers
}

// HistoryStoreOption defines functional options for configuring local.
type HistoryStoreOption func(*HistoryStoreOptions)

// HistoryStoreOptions holds configuration options for local.
type HistoryStoreOptions struct {
	FileCache         *fileutil.Cache[*models.DAGRunStatus] // Optional cache for status files
	LatestStatusToday bool                                  // Whether to only return today's status
	MaxWorkers        int                                   // Maximum number of parallel workers
	OperationTimeout  time.Duration                         // Timeout for operations
}

// WithHistoryFileCache sets the file cache for local.
func WithHistoryFileCache(cache *fileutil.Cache[*models.DAGRunStatus]) HistoryStoreOption {
	return func(o *HistoryStoreOptions) {
		o.FileCache = cache
	}
}

// WithLatestStatusToday sets whether to only return today's status.
func WithLatestStatusToday(latestStatusToday bool) HistoryStoreOption {
	return func(o *HistoryStoreOptions) {
		o.LatestStatusToday = latestStatusToday
	}
}

// New creates a new JSONDB instance with the specified options.
func New(baseDir string, opts ...HistoryStoreOption) models.DAGRunStore {
	options := &HistoryStoreOptions{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
	}

	for _, opt := range opts {
		opt(options)
	}

	return &Store{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
	}
}

// ListStatuses retrieves status records based on the provided options.
// It supports filtering by time range, status, and limiting the number of results.
func (store *Store) ListStatuses(ctx context.Context, opts ...models.ListDAGRunStatusesOption) ([]*models.DAGRunStatus, error) {
	// Apply options and set defaults
	options, err := prepareListOptions(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare options: %w", err)
	}

	var rootDirs []DataRoot
	if options.ExactName == "" {
		// Get all root directories
		d, err := store.listRoot(ctx, options.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to list root directories: %w", err)
		}
		rootDirs = d
	} else {
		rootDirs = append(rootDirs, NewDataRootWithPrefix(store.baseDir, options.ExactName))
	}

	// Collect and filter results
	return store.collectStatusesFromRoots(ctx, rootDirs, options)
}

// prepareListOptions processes the provided options and sets default values.
func prepareListOptions(opts []models.ListDAGRunStatusesOption) (models.ListStatusesOptions, error) {
	var options models.ListStatusesOptions

	// Apply all options
	for _, opt := range opts {
		opt(&options)
	}

	// Set default time range if not specified
	if options.From.IsZero() && options.To.IsZero() {
		options.From = models.NewUTC(time.Now().Truncate(24 * time.Hour))
	}

	// Enforce a reasonable limit on the number of results
	const maxLimit = 1000
	if options.Limit == 0 || options.Limit > maxLimit {
		options.Limit = maxLimit
	}

	return options, nil
}

// collectStatusesFromRoots gathers statuses from root directories according to the options.
func (store *Store) collectStatusesFromRoots(
	parentCtx context.Context,
	roots []DataRoot,
	opts models.ListStatusesOptions,
) ([]*models.DAGRunStatus, error) {

	if len(roots) == 0 {
		return nil, nil
	}
	maxWorkers := min(runtime.NumCPU(), len(roots))

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	var (
		resultsMu      sync.Mutex
		results        = make([]*models.DAGRunStatus, 0, opts.Limit)
		remaining      atomic.Int64
		statusesFilter = make(map[scheduler.Status]struct{})
	)

	for _, status := range opts.Statuses {
		statusesFilter[status] = struct{}{}
	}
	hasStatusFilter := len(statusesFilter) > 0

	remaining.Store(int64(opts.Limit))

	jobs := make(chan DataRoot)
	var wg sync.WaitGroup

	worker := func() {
		defer wg.Done()
		for root := range jobs {
			if ctx.Err() != nil || remaining.Load() <= 0 {
				return
			}

			dagRuns := root.listDAGRunsInRange(ctx, opts.From, opts.To, &listDAGRunsInRangeOpts{
				limit: int(remaining.Load()),
			})

			statuses := make([]*models.DAGRunStatus, 0, len(dagRuns))
			for _, dagRun := range dagRuns {
				if opts.DAGRunID != "" && !strings.Contains(dagRun.dagRunID, opts.DAGRunID) {
					continue
				}

				run, err := dagRun.LatestAttempt(ctx, store.cache)
				if err != nil {
					logger.Error(ctx, "Failed to get latest run", "err", err)
					continue
				}

				status, err := run.ReadStatus(ctx)
				if err != nil {
					logger.Error(ctx, "Failed to read status", "err", err)
					continue
				}
				if !hasStatusFilter {
					statuses = append(statuses, status)
					continue
				}
				if _, ok := statusesFilter[status.Status]; !ok {
					continue
				}
				statuses = append(statuses, status)
			}

			taken := int64(len(dagRuns))
			if d := remaining.Add(-taken); d < 0 {
				cancel()
			}

			resultsMu.Lock()
			results = append(results, statuses...)
			resultsMu.Unlock()
		}
	}

	// Start workers
	for range maxWorkers {
		wg.Add(1)
		go worker()
	}

	// Send jobs to workers
	for _, root := range roots {
		if ctx.Err() != nil || remaining.Load() <= 0 {
			break
		}
		jobs <- root
	}
	close(jobs)

	wg.Wait()

	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt > results[j].CreatedAt
	})
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// CreateAttempt creates a new history record for the specified DAG-run ID.
// If opts.Root is not nil, it creates a new history record for a child DAG-run.
// If opts.Retry is true, it creates a retry record for the specified DAG-run ID.
func (store *Store) CreateAttempt(ctx context.Context, dag *digraph.DAG, timestamp time.Time, dagRunID string, opts models.NewDAGRunAttemptOptions) (models.DAGRunAttempt, error) {
	if dagRunID == "" {
		return nil, ErrDAGRunIDEmpty
	}

	if opts.RootDAGRun != nil {
		return store.newChildRecord(ctx, dag, timestamp, dagRunID, opts)
	}

	dataRoot := NewDataRoot(store.baseDir, dag.Name)
	ts := models.NewUTC(timestamp)

	var run *DAGRun
	if opts.Retry {
		r, err := dataRoot.FindByDAGRunID(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to find execution: %w", err)
		}
		run = r
	} else {
		r, err := dataRoot.CreateDAGRun(ts, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to create run: %w", err)
		}
		run = r
	}

	record, err := run.CreateAttempt(ctx, ts, store.cache, WithDAG(dag))
	if err != nil {
		return nil, fmt.Errorf("failed to create record: %w", err)
	}

	return record, nil
}

// newChildRecord creates a new history record for a child DAG-run.
func (b *Store) newChildRecord(ctx context.Context, dag *digraph.DAG, timestamp time.Time, dagRunID string, opts models.NewDAGRunAttemptOptions) (models.DAGRunAttempt, error) {
	dataRoot := NewDataRoot(b.baseDir, opts.RootDAGRun.Name)
	root, err := dataRoot.FindByDAGRunID(ctx, opts.RootDAGRun.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find root execution: %w", err)
	}

	ts := models.NewUTC(timestamp)

	var run *DAGRun
	if opts.Retry {
		r, err := root.FindChildDAGRun(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to find child DAG-run record: %w", err)
		}
		run = r
	} else {
		r, err := root.CreateChildDAGRun(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to create child DAG-run: %w", err)
		}
		run = r
	}

	record, err := run.CreateAttempt(ctx, ts, b.cache, WithDAG(dag))
	if err != nil {
		logger.Error(ctx, "Failed to create child DAG-run record", "err", err)
		return nil, err
	}

	return record, nil
}

// RecentAttempts returns the most recent history records for the specified DAG name.
func (store *Store) RecentAttempts(ctx context.Context, dagName string, itemLimit int) []models.DAGRunAttempt {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		logger.Errorf(ctx, "Recent canceled: %v", ctx.Err())
		return nil
	default:
		// Continue with operation
	}

	if itemLimit <= 0 {
		logger.Warnf(ctx, "Invalid itemLimit %d, using default of 10", itemLimit)
		itemLimit = 10
	}

	// Get the latest matches
	root := NewDataRoot(store.baseDir, dagName)
	items := root.Latest(ctx, itemLimit)

	// Get the latest record for each item
	records := make([]models.DAGRunAttempt, 0, len(items))
	for _, item := range items {
		record, err := item.LatestAttempt(ctx, store.cache)
		if err != nil {
			logger.Error(ctx, "Failed to get latest record", "err", err)
			continue
		}
		records = append(records, record)
	}

	return records
}

// LatestAttempt returns the most recent history record for the specified DAG name.
// If latestStatusToday is true, it only returns today's status.
func (store *Store) LatestAttempt(ctx context.Context, dagName string) (models.DAGRunAttempt, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Latest canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(store.baseDir, dagName)

	if store.latestStatusToday {
		startOfDay := time.Now().Truncate(24 * time.Hour)
		startOfDayInUTC := models.NewUTC(startOfDay)

		// Get the latest execution data after the start of the day.
		exec, err := root.LatestAfter(ctx, startOfDayInUTC)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest after: %w", err)
		}

		return exec.LatestAttempt(ctx, store.cache)
	}

	// Get the latest execution data.
	latest := root.Latest(ctx, 1)
	if len(latest) == 0 {
		return nil, models.ErrNoStatusData
	}
	return latest[0].LatestAttempt(ctx, store.cache)
}

// FindAttempt finds a history record by DAG-run ID.
func (store *Store) FindAttempt(ctx context.Context, ref digraph.DAGRunRef) (models.DAGRunAttempt, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("Find canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.ID == "" {
		return nil, ErrDAGRunIDEmpty
	}

	root := NewDataRoot(store.baseDir, ref.Name)
	run, err := root.FindByDAGRunID(ctx, ref.ID)

	if err != nil {
		return nil, err
	}

	return run.LatestAttempt(ctx, store.cache)
}

// FindChildAttempt finds a child DAG-run by its ID.
// It returns the latest record for the specified child DAG-run ID.
func (store *Store) FindChildAttempt(ctx context.Context, ref digraph.DAGRunRef, childDAGRunID string) (models.DAGRunAttempt, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("FindChildDAGRun canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if ref.ID == "" {
		return nil, ErrDAGRunIDEmpty
	}

	root := NewDataRoot(store.baseDir, ref.Name)
	dagRun, err := root.FindByDAGRunID(ctx, ref.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find execution: %w", err)
	}

	childDAGRun, err := dagRun.FindChildDAGRun(ctx, childDAGRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find child DAG-run: %w", err)
	}
	return childDAGRun.LatestAttempt(ctx, store.cache)
}

// RemoveOldDAGRuns removes old history records older than the specified retention days.
// It only removes records older than the specified retention days.
// If retentionDays is negative, no files will be removed.
// If retentionDays is zero, all files will be removed.
// If retentionDays is positive, only files older than the specified number of days will be removed.
func (store *Store) RemoveOldDAGRuns(ctx context.Context, dagName string, retentionDays int) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("RemoveOld canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	if retentionDays < 0 {
		logger.Warnf(ctx, "Negative retentionDays %d, no files will be removed", retentionDays)
		return nil
	}

	root := NewDataRoot(store.baseDir, dagName)
	return root.RemoveOld(ctx, retentionDays)
}

// RenameDAGRuns renames all history records for the specified DAG name.
func (store *Store) RenameDAGRuns(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return fmt.Errorf("Rename canceled: %w", ctx.Err())
	default:
		// Continue with operation
	}

	root := NewDataRoot(store.baseDir, oldNameOrPath)
	newRoot := NewDataRoot(store.baseDir, newNameOrPath)
	return root.Rename(ctx, newRoot)
}

// listRoot lists all root directories in the base directory.
func (store *Store) listRoot(_ context.Context, include string) ([]DataRoot, error) {
	rootDirs, err := listDirsSorted(store.baseDir, false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list root directories: %w", err)
	}

	var roots []DataRoot
	for _, dir := range rootDirs {
		if include != "" && !strings.Contains(dir, include) {
			continue
		}
		if fileutil.IsDir(filepath.Join(store.baseDir, dir)) {
			root := NewDataRoot(store.baseDir, dir)
			roots = append(roots, root)
		}
	}

	return roots, nil
}
