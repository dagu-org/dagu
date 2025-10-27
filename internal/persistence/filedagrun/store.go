package filedagrun

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

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
)

// Error definitions for common issues
var (
	ErrDAGRunIDEmpty  = errors.New("dag-run ID is empty")
	ErrTooManyResults = errors.New("too many results found")
)

var _ execution.DAGRunStore = (*Store)(nil)

// Store manages DAGs status files in local Store with high performance and reliability.
type Store struct {
	baseDir           string                                   // Base directory for all status files
	latestStatusToday bool                                     // Whether to only return today's status
	cache             *fileutil.Cache[*execution.DAGRunStatus] // Optional cache for read operations
	maxWorkers        int                                      // Maximum number of parallel workers
	location          *time.Location                           // Timezone location for date calculations
}

// DAGRunStoreOption defines functional options for configuring local.
type DAGRunStoreOption func(*DAGRunStoreOptions)

// DAGRunStoreOptions holds configuration options for local.
type DAGRunStoreOptions struct {
	FileCache         *fileutil.Cache[*execution.DAGRunStatus] // Optional cache for status files
	LatestStatusToday bool                                     // Whether to only return today's status
	MaxWorkers        int                                      // Maximum number of parallel workers
	OperationTimeout  time.Duration                            // Timeout for operations
	Location          *time.Location                           // Timezone location for date calculations
}

// WithHistoryFileCache sets the file cache for local.
func WithHistoryFileCache(cache *fileutil.Cache[*execution.DAGRunStatus]) DAGRunStoreOption {
	return func(o *DAGRunStoreOptions) {
		o.FileCache = cache
	}
}

// WithLatestStatusToday sets whether to only return today's status.
func WithLatestStatusToday(latestStatusToday bool) DAGRunStoreOption {
	return func(o *DAGRunStoreOptions) {
		o.LatestStatusToday = latestStatusToday
	}
}

// WithLocation sets the timezone location for date calculations.
func WithLocation(location *time.Location) DAGRunStoreOption {
	return func(o *DAGRunStoreOptions) {
		o.Location = location
	}
}

// New creates a new JSONDB instance with the specified options.
func New(baseDir string, opts ...DAGRunStoreOption) execution.DAGRunStore {
	options := &DAGRunStoreOptions{
		LatestStatusToday: true,
		MaxWorkers:        runtime.NumCPU(),
		Location:          time.Local, // Default to local timezone
	}

	for _, opt := range opts {
		opt(options)
	}

	return &Store{
		baseDir:           baseDir,
		latestStatusToday: options.LatestStatusToday,
		cache:             options.FileCache,
		maxWorkers:        options.MaxWorkers,
		location:          options.Location,
	}
}

// ListStatuses retrieves status records based on the provided options.
// It supports filtering by time range, status, and limiting the number of results.
func (store *Store) ListStatuses(ctx context.Context, opts ...execution.ListDAGRunStatusesOption) ([]*execution.DAGRunStatus, error) {
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
func prepareListOptions(opts []execution.ListDAGRunStatusesOption) (execution.ListDAGRunStatusesOptions, error) {
	var options execution.ListDAGRunStatusesOptions

	// Apply all options
	for _, opt := range opts {
		opt(&options)
	}

	// Set default time range if not specified
	if options.From.IsZero() && options.To.IsZero() {
		options.From = execution.NewUTC(time.Now().Truncate(24 * time.Hour))
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
	opts execution.ListDAGRunStatusesOptions,
) ([]*execution.DAGRunStatus, error) {

	if len(roots) == 0 {
		return nil, nil
	}
	maxWorkers := min(runtime.NumCPU(), len(roots))

	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()

	var (
		resultsMu      sync.Mutex
		results        = make([]*execution.DAGRunStatus, 0, opts.Limit)
		remaining      atomic.Int64
		statusesFilter = make(map[core.Status]struct{})
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

			statuses := make([]*execution.DAGRunStatus, 0, len(dagRuns))
			for _, dagRun := range dagRuns {
				if opts.DAGRunID != "" && !strings.Contains(dagRun.dagRunID, opts.DAGRunID) {
					continue
				}

				run, err := dagRun.LatestAttempt(ctx, store.cache)
				if err != nil {
					if !errors.Is(err, execution.ErrNoStatusData) {
						logger.Error(ctx, "Failed to get latest run", "err", err)
					}
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
		if results[i].CreatedAt != results[j].CreatedAt {
			return results[i].CreatedAt > results[j].CreatedAt
		}
		return results[i].DAGRunID < results[j].DAGRunID
	})
	if len(results) > opts.Limit {
		results = results[:opts.Limit]
	}
	return results, nil
}

// CreateAttempt creates a new history record for the specified dag-run ID.
// If opts.Root is not nil, it creates a new history record for a sub dag-run.
// If opts.Retry is true, it creates a retry record for the specified dag-run ID.
func (store *Store) CreateAttempt(ctx context.Context, dag *core.DAG, timestamp time.Time, dagRunID string, opts execution.NewDAGRunAttemptOptions) (execution.DAGRunAttempt, error) {
	if dagRunID == "" {
		return nil, ErrDAGRunIDEmpty
	}

	if opts.RootDAGRun != nil {
		return store.newChildRecord(ctx, dag, timestamp, dagRunID, opts)
	}

	dataRoot := NewDataRoot(store.baseDir, dag.Name)
	ts := execution.NewUTC(timestamp)

	lockCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := dataRoot.Lock(lockCtx); err != nil {
		return nil, fmt.Errorf("failed to acquire lock for dag-run %s: %w", dagRunID, err)
	}
	defer func() {
		if err := dataRoot.Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock dag-run", "dagRunID", dagRunID, "err", err)
		}
	}()

	var run *DAGRun
	if opts.Retry {
		r, err := dataRoot.FindByDAGRunID(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to find execution: %w", err)
		}
		run = r
	} else {
		// Check if the dag-run already exists
		existingRun, _ := dataRoot.FindByDAGRunID(ctx, dagRunID)
		if existingRun != nil {
			// Error if the dag-run already exists
			return nil, fmt.Errorf("dag-run with ID %s already exists", dagRunID)
		}
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

// newChildRecord creates a new history record for a sub dag-run.
func (b *Store) newChildRecord(ctx context.Context, dag *core.DAG, timestamp time.Time, dagRunID string, opts execution.NewDAGRunAttemptOptions) (execution.DAGRunAttempt, error) {
	dataRoot := NewDataRoot(b.baseDir, opts.RootDAGRun.Name)
	root, err := dataRoot.FindByDAGRunID(ctx, opts.RootDAGRun.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find root execution: %w", err)
	}

	ts := execution.NewUTC(timestamp)

	var run *DAGRun
	if opts.Retry {
		r, err := root.FindSubDAGRun(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to find sub dag-run record: %w", err)
		}
		run = r
	} else {
		r, err := root.CreateSubDAGRun(ctx, dagRunID)
		if err != nil {
			return nil, fmt.Errorf("failed to create sub dag-run: %w", err)
		}
		run = r
	}

	record, err := run.CreateAttempt(ctx, ts, b.cache, WithDAG(dag))
	if err != nil {
		logger.Error(ctx, "Failed to create sub dag-run record", "err", err)
		return nil, err
	}

	return record, nil
}

// RecentAttempts returns the most recent history records for the specified DAG name.
func (store *Store) RecentAttempts(ctx context.Context, dagName string, itemLimit int) []execution.DAGRunAttempt {
	if itemLimit <= 0 {
		logger.Warnf(ctx, "Invalid itemLimit %d, using default of 10", itemLimit)
		itemLimit = 10
	}

	// Get the latest matches
	root := NewDataRoot(store.baseDir, dagName)
	items := root.Latest(ctx, itemLimit)

	// Get the latest record for each item
	records := make([]execution.DAGRunAttempt, 0, len(items))
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
func (store *Store) LatestAttempt(ctx context.Context, dagName string) (execution.DAGRunAttempt, error) {
	root := NewDataRoot(store.baseDir, dagName)

	if store.latestStatusToday {
		// Use the configured timezone to calculate "today"
		now := time.Now().In(store.location)
		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, store.location)
		startOfDayInUTC := execution.NewUTC(startOfDay)

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
		return nil, execution.ErrNoStatusData
	}
	return latest[0].LatestAttempt(ctx, store.cache)
}

// FindAttempt finds a history record by dag-run ID.
func (store *Store) FindAttempt(ctx context.Context, ref execution.DAGRunRef) (execution.DAGRunAttempt, error) {
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

// FindSubAttempt finds a sub dag-run by its ID.
// It returns the latest record for the specified sub dag-run ID.
func (store *Store) FindSubAttempt(ctx context.Context, ref execution.DAGRunRef, subDAGRunID string) (execution.DAGRunAttempt, error) {
	if ref.ID == "" {
		return nil, ErrDAGRunIDEmpty
	}

	root := NewDataRoot(store.baseDir, ref.Name)
	dagRun, err := root.FindByDAGRunID(ctx, ref.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find execution: %w", err)
	}

	subDAGRun, err := dagRun.FindSubDAGRun(ctx, subDAGRunID)
	if err != nil {
		return nil, fmt.Errorf("failed to find sub dag-run: %w", err)
	}
	return subDAGRun.LatestAttempt(ctx, store.cache)
}

// RemoveOldDAGRuns removes old history records older than the specified retention days.
// It only removes records older than the specified retention days.
// If retentionDays is negative, no files will be removed.
// If retentionDays is zero, all files will be removed.
// If retentionDays is positive, only files older than the specified number of days will be removed.
func (store *Store) RemoveOldDAGRuns(ctx context.Context, dagName string, retentionDays int) error {
	if retentionDays < 0 {
		logger.Warnf(ctx, "Negative retentionDays %d, no files will be removed", retentionDays)
		return nil
	}

	root := NewDataRoot(store.baseDir, dagName)
	return root.RemoveOld(ctx, retentionDays)
}

// RemoveDAGRun implements models.DAGRunStore.
func (store *Store) RemoveDAGRun(ctx context.Context, dagRun execution.DAGRunRef) error {
	if dagRun.ID == "" {
		return ErrDAGRunIDEmpty
	}

	root := NewDataRoot(store.baseDir, dagRun.Name)
	run, err := root.FindByDAGRunID(ctx, dagRun.ID)
	if err != nil {
		return fmt.Errorf("failed to find dag-run %s: %w", dagRun.ID, err)
	}

	if err := root.Lock(ctx); err != nil {
		return fmt.Errorf("failed to acquire lock for dag-run %s: %w", dagRun.ID, err)
	}

	defer func() {
		if err := root.Unlock(); err != nil {
			logger.Error(ctx, "Failed to unlock dag-run", "dagRunID", dagRun.ID, "err", err)
		}
	}()

	if err := run.Remove(ctx); err != nil {
		return fmt.Errorf("failed to remove dag-run %s: %w", dagRun.ID, err)
	}

	return nil
}

// RenameDAGRuns renames all history records for the specified DAG name.
func (store *Store) RenameDAGRuns(ctx context.Context, oldNameOrPath, newNameOrPath string) error {
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
