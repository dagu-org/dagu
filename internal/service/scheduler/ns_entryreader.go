package scheduler

import (
	"context"
	"sync"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/persis"
	"github.com/dagu-org/dagu/internal/persis/filens"
	"github.com/dagu-org/dagu/internal/runtime"
)

// NamespaceEntryReader implements EntryReader for multi-namespace support.
// It discovers all namespaces and aggregates scheduled jobs from each.
type NamespaceEntryReader struct {
	nsStore     *filens.Store
	factory     *persis.Factory
	dagExecutor *DAGExecutor
	executable  string

	lock    sync.Mutex
	readers map[string]*entryReaderImpl // namespace ID -> reader
	quit    chan struct{}
}

// NewNamespaceEntryReader creates a new namespace-aware entry reader.
// It uses the namespace store to discover namespaces and the factory to create
// namespace-scoped stores for each namespace.
func NewNamespaceEntryReader(
	nsStore *filens.Store,
	factory *persis.Factory,
	dagExecutor *DAGExecutor,
	executable string,
) EntryReader {
	return &NamespaceEntryReader{
		nsStore:     nsStore,
		factory:     factory,
		dagExecutor: dagExecutor,
		executable:  executable,
		readers:     make(map[string]*entryReaderImpl),
		quit:        make(chan struct{}),
	}
}

// Init initializes entry readers for all namespaces.
func (r *NamespaceEntryReader) Init(ctx context.Context) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// List all namespaces
	namespaces, err := r.nsStore.List(ctx)
	if err != nil {
		logger.Error(ctx, "Failed to list namespaces", tag.Error(err))
		return err
	}

	logger.Info(ctx, "Initializing namespace entry readers", tag.Count(len(namespaces)))

	// Create an entry reader for each namespace
	for _, ns := range namespaces {
		if err := r.initNamespaceReader(ctx, ns.ID, ns.Name); err != nil {
			logger.Error(ctx, "Failed to initialize entry reader for namespace",
				tag.Error(err),
				tag.Namespace(ns.Name),
			)
			// Continue with other namespaces
			continue
		}
	}

	return nil
}

// initNamespaceReader creates and initializes an entry reader for a specific namespace.
func (r *NamespaceEntryReader) initNamespaceReader(ctx context.Context, nsID, nsName string) error {
	stores := r.factory.ForNamespace(nsID)

	// Create a runtime manager for this namespace
	drm := runtime.NewManager(stores.DAGRuns, stores.Procs, nil)

	// Create the entry reader for this namespace
	dagsDir := r.factory.DAGsDir(nsID)
	reader := &entryReaderImpl{
		targetDir:   dagsDir,
		namespace:   nsName,
		lock:        sync.Mutex{},
		registry:    map[string]*dagEntry{},
		dagStore:    stores.DAGs,
		dagRunMgr:   drm,
		executable:  r.executable,
		dagExecutor: r.dagExecutor,
		quit:        make(chan struct{}),
	}

	if err := reader.initWithNamespace(ctx, nsName); err != nil {
		return err
	}

	r.readers[nsID] = reader
	logger.Info(ctx, "Initialized entry reader for namespace",
		tag.Namespace(nsName),
		tag.Dir(dagsDir),
	)

	return nil
}

// Start starts watching all namespace directories for changes.
func (r *NamespaceEntryReader) Start(ctx context.Context) {
	r.lock.Lock()
	readers := make([]*entryReaderImpl, 0, len(r.readers))
	for _, reader := range r.readers {
		readers = append(readers, reader)
	}
	r.lock.Unlock()

	// Start all readers in separate goroutines
	var wg sync.WaitGroup
	for _, reader := range readers {
		wg.Add(1)
		go func(rdr *entryReaderImpl) {
			defer wg.Done()
			rdr.Start(ctx)
		}(reader)
	}

	// Wait for quit signal
	select {
	case <-r.quit:
	case <-ctx.Done():
	}

	// Stop all readers
	r.lock.Lock()
	for _, reader := range r.readers {
		reader.Stop()
	}
	r.lock.Unlock()

	wg.Wait()
}

// Stop stops watching all namespace directories.
func (r *NamespaceEntryReader) Stop() {
	r.lock.Lock()
	defer r.lock.Unlock()

	select {
	case <-r.quit:
		// Already closed
	default:
		close(r.quit)
	}

	for _, reader := range r.readers {
		reader.Stop()
	}
}

// Next returns the next scheduled jobs from all namespaces.
func (r *NamespaceEntryReader) Next(ctx context.Context, now time.Time) ([]*ScheduledJob, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	var allJobs []*ScheduledJob

	for nsID, reader := range r.readers {
		jobs, err := reader.Next(ctx, now)
		if err != nil {
			logger.Error(ctx, "Failed to get next jobs for namespace",
				tag.Error(err),
				tag.Namespace(nsID),
			)
			continue
		}
		allJobs = append(allJobs, jobs...)
	}

	return allJobs, nil
}
