// Package persis provides persistence layer components for Dagu.
package persis

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/persis/filedag"
	"github.com/dagu-org/dagu/internal/persis/filedagrun"
	"github.com/dagu-org/dagu/internal/persis/fileproc"
	"github.com/dagu-org/dagu/internal/persis/filequeue"
)

// NamespaceStores contains all stores scoped to a specific namespace.
type NamespaceStores struct {
	// DAGs provides DAG definition storage
	DAGs exec.DAGStore
	// DAGRuns provides DAG run history storage
	DAGRuns exec.DAGRunStore
	// Queue provides queue storage
	Queue exec.QueueStore
	// Procs provides process tracking storage
	Procs exec.ProcStore
}

// Factory creates namespace-scoped store instances.
// It manages the base directories and creates stores with namespace-specific paths.
type Factory struct {
	// configDir is the base configuration directory (e.g., ~/.config/dagu)
	configDir string
	// dataDir is the base data directory (e.g., ~/.local/share/dagu/data)
	dataDir string
	// logDir is the base log directory (e.g., ~/.local/share/dagu/logs)
	logDir string

	// Optional caches for performance
	dagCache       *fileutil.Cache[*core.DAG]
	dagRunCache    *fileutil.Cache[*exec.DAGRunStatus]
	dagStoreOpts   []filedag.Option
	dagRunOpts     []filedagrun.DAGRunStoreOption
}

// FactoryOption is a functional option for configuring the Factory.
type FactoryOption func(*Factory)

// WithDAGCache sets the cache for DAG objects.
func WithDAGCache(cache *fileutil.Cache[*core.DAG]) FactoryOption {
	return func(f *Factory) {
		f.dagCache = cache
	}
}

// WithDAGRunCache sets the cache for DAG run status objects.
func WithDAGRunCache(cache *fileutil.Cache[*exec.DAGRunStatus]) FactoryOption {
	return func(f *Factory) {
		f.dagRunCache = cache
	}
}

// WithDAGStoreOptions sets additional options for DAG store creation.
func WithDAGStoreOptions(opts ...filedag.Option) FactoryOption {
	return func(f *Factory) {
		f.dagStoreOpts = opts
	}
}

// WithDAGRunStoreOptions sets additional options for DAG run store creation.
func WithDAGRunStoreOptions(opts ...filedagrun.DAGRunStoreOption) FactoryOption {
	return func(f *Factory) {
		f.dagRunOpts = opts
	}
}

// NewFactory creates a new Factory with the specified base directories.
func NewFactory(configDir, dataDir, logDir string, opts ...FactoryOption) *Factory {
	f := &Factory{
		configDir: configDir,
		dataDir:   dataDir,
		logDir:    logDir,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// ForNamespace creates namespace-scoped stores for the given namespace internal ID.
// The nsID is the internal UUID of the namespace, not the namespace name.
// This allows namespace renaming without moving files on disk.
func (f *Factory) ForNamespace(nsID string) *NamespaceStores {
	return &NamespaceStores{
		DAGs:    f.createDAGStore(nsID),
		DAGRuns: f.createDAGRunStore(nsID),
		Queue:   f.createQueueStore(nsID),
		Procs:   f.createProcStore(nsID),
	}
}

// createDAGStore creates a namespace-scoped DAG store.
func (f *Factory) createDAGStore(nsID string) exec.DAGStore {
	dagsDir := f.DAGsDir(nsID)
	flagsDir := f.FlagsDir(nsID)

	opts := []filedag.Option{
		filedag.WithFlagsBaseDir(flagsDir),
		filedag.WithSkipExamples(true), // Don't create examples in namespaced dirs
	}

	if f.dagCache != nil {
		opts = append(opts, filedag.WithFileCache(f.dagCache))
	}

	// Add any additional options
	opts = append(opts, f.dagStoreOpts...)

	return filedag.New(dagsDir, opts...)
}

// createDAGRunStore creates a namespace-scoped DAG run store.
func (f *Factory) createDAGRunStore(nsID string) exec.DAGRunStore {
	dagRunsDir := f.DAGRunsDir(nsID)

	opts := []filedagrun.DAGRunStoreOption{}

	if f.dagRunCache != nil {
		opts = append(opts, filedagrun.WithHistoryFileCache(f.dagRunCache))
	}

	// Add any additional options
	opts = append(opts, f.dagRunOpts...)

	return filedagrun.New(dagRunsDir, opts...)
}

// createQueueStore creates a namespace-scoped queue store.
func (f *Factory) createQueueStore(nsID string) exec.QueueStore {
	queueDir := f.QueueDir(nsID)
	return filequeue.New(queueDir)
}

// createProcStore creates a namespace-scoped process store.
func (f *Factory) createProcStore(nsID string) exec.ProcStore {
	procsDir := f.ProcsDir(nsID)
	return fileproc.New(procsDir)
}

// Path accessors for namespace-specific directories

// NamespacesDir returns the directory containing all namespace metadata files.
func (f *Factory) NamespacesDir() string {
	return filepath.Join(f.configDir, "namespaces")
}

// NamespaceConfigDir returns the config directory for a specific namespace.
func (f *Factory) NamespaceConfigDir(nsID string) string {
	return filepath.Join(f.configDir, "namespaces", nsID)
}

// NamespaceConfigFile returns the service config file path for a namespace.
func (f *Factory) NamespaceConfigFile(nsID string) string {
	return filepath.Join(f.NamespaceConfigDir(nsID), "config.yaml")
}

// NamespaceBaseConfigFile returns the DAG base config file path for a namespace.
func (f *Factory) NamespaceBaseConfigFile(nsID string) string {
	return filepath.Join(f.NamespaceConfigDir(nsID), "base.yaml")
}

// NamespaceDataDir returns the data directory for a specific namespace.
func (f *Factory) NamespaceDataDir(nsID string) string {
	return filepath.Join(f.dataDir, "namespaces", nsID)
}

// DAGsDir returns the DAGs directory for a namespace.
func (f *Factory) DAGsDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "dags")
}

// DAGRunsDir returns the DAG runs directory for a namespace.
func (f *Factory) DAGRunsDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "dag-runs")
}

// QueueDir returns the queue directory for a namespace.
func (f *Factory) QueueDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "queue")
}

// ProcsDir returns the process tracking directory for a namespace.
func (f *Factory) ProcsDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "procs")
}

// FlagsDir returns the suspension flags directory for a namespace.
func (f *Factory) FlagsDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "flags")
}

// LogsDir returns the logs directory for a namespace.
func (f *Factory) LogsDir(nsID string) string {
	return filepath.Join(f.logDir, "namespaces", nsID)
}

// AuditDir returns the audit logs directory for a namespace.
func (f *Factory) AuditDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "audit")
}

// WebhooksDir returns the webhooks directory for a namespace.
func (f *Factory) WebhooksDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "webhooks")
}

// APIKeysDir returns the API keys directory for a namespace.
func (f *Factory) APIKeysDir(nsID string) string {
	return filepath.Join(f.NamespaceDataDir(nsID), "apikeys")
}

// ConfigDir returns the base config directory.
func (f *Factory) ConfigDir() string {
	return f.configDir
}

// DataDir returns the base data directory.
func (f *Factory) DataDir() string {
	return f.dataDir
}

// LogDir returns the base log directory.
func (f *Factory) LogDir() string {
	return f.logDir
}

// String returns a string representation of the factory for debugging.
func (f *Factory) String() string {
	return fmt.Sprintf("Factory{configDir=%s, dataDir=%s, logDir=%s}",
		f.configDir, f.dataDir, f.logDir)
}
