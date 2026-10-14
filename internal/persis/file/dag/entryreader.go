// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package dag

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dagucloud/dagu/v2/internal/cmn/filenotify"
	"github.com/dagucloud/dagu/v2/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger"
	"github.com/dagucloud/dagu/v2/internal/cmn/logger/tag"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/pagination"
	"github.com/dagucloud/dagu/v2/internal/persis"

	"github.com/fsnotify/fsnotify"
)

// entryReaderImpl manages DAGs on local filesystem.
type entryReaderImpl struct {
	baseConfigPath         string
	workspaceBaseConfigDir string
	baseConfigState        string
	baseWatchPaths         []baseWatchPath
	targetDir              string
	registry               map[string]*ir.DAG
	stamps                 map[string]dagFileStamp
	watchedDirs            map[string]struct{}
	lock                   sync.Mutex
	dagRepository          *persis.DAGRepository
	dagSource              *dagFileSource
	watcher                filenotify.FileWatcher
	recursive              bool
	quit                   chan struct{}
	closeOnce              sync.Once
	events                 chan persis.DAGChangeEvent
}

type dagFileStamp struct {
	size    int64
	modTime int64
}

type registryState struct {
	dags   map[string]*ir.DAG
	stamps map[string]dagFileStamp
	issues []string
}

// NewFileEntryReader creates a filesystem DAG entry reader.
func NewFileEntryReader(dir string, dagRepository *persis.DAGRepository, recursive bool, baseConfigPath, workspaceBaseConfigDir string) *entryReaderImpl {
	return &entryReaderImpl{
		targetDir:              dir,
		baseConfigPath:         baseConfigPath,
		workspaceBaseConfigDir: workspaceBaseConfigDir,
		registry:               make(map[string]*ir.DAG),
		stamps:                 make(map[string]dagFileStamp),
		watchedDirs:            make(map[string]struct{}),
		dagRepository:          dagRepository,
		dagSource:              newDAGFileSource(dir, dagRepository),
		recursive:              recursive,
		quit:                   make(chan struct{}),
		events:                 make(chan persis.DAGChangeEvent, 64),
	}
}

// Init loads the initial DAG registry and starts watching the target directory.
func (er *entryReaderImpl) Init(ctx context.Context) error {
	for _, path := range []*string{&er.targetDir, &er.baseConfigPath, &er.workspaceBaseConfigDir} {
		if *path != "" {
			absolute, err := filepath.Abs(*path)
			if err != nil {
				return err
			}
			*path = absolute
		}
	}
	er.baseConfigState = describeBaseConfigStateSet(er.baseConfigPath, er.workspaceBaseConfigDir)
	if er.recursive {
		return er.initRecursive(ctx)
	}

	er.lock.Lock()
	defer er.lock.Unlock()
	er.watcher = filenotify.New(time.Minute)
	if err := er.addInitialWatches([]string{er.targetDir}); err != nil {
		_ = er.watcher.Close()
		return err
	}
	if err := er.initialize(ctx); err != nil {
		_ = er.watcher.Close()
		return fmt.Errorf("failed to initialize DAGs: %w", err)
	}
	return nil
}

// Start forwards watcher events into registry updates until the reader stops.
func (er *entryReaderImpl) Start(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(ctx, "Entry reader watcher panicked", tag.Error(fmt.Errorf("%v", r)))
		}
	}()
	er.watch(ctx)
}

const registryRefreshDelay = 75 * time.Millisecond

func (er *entryReaderImpl) watch(ctx context.Context) {
	var refreshTimer *time.Timer
	var refresh <-chan time.Time
	scheduleRefresh := func() {
		if refreshTimer == nil {
			refreshTimer = time.NewTimer(registryRefreshDelay)
			refresh = refreshTimer.C
			return
		}
		if !refreshTimer.Stop() {
			select {
			case <-refreshTimer.C:
			default:
			}
		}
		refreshTimer.Reset(registryRefreshDelay)
		refresh = refreshTimer.C
	}
	defer func() {
		if refreshTimer != nil {
			refreshTimer.Stop()
		}
	}()

	for {
		select {
		case <-er.quit:
			return
		case <-ctx.Done():
			return
		case event, ok := <-er.watcher.Events():
			if !ok {
				return
			}
			if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				// Native watches disappear with their directory, even if its name is reused.
				for dir := range er.watchedDirs {
					if pathWithin(dir, event.Name) {
						_ = er.watcher.Remove(dir)
						delete(er.watchedDirs, dir)
					}
				}
			}
			// A symlink target or its link directory can change without an event
			// named after the configured base file.
			baseState := describeBaseConfigStateSet(er.baseConfigPath, er.workspaceBaseConfigDir)
			if baseState != er.baseConfigState {
				er.baseConfigState = baseState
				scheduleRefresh()
				continue
			}
			switch {
			case er.isBaseConfigEvent(event):
				scheduleRefresh()
			case !pathWithin(event.Name, er.targetDir), pathWithin(event.Name, er.workspaceBaseConfigDir):
				continue
			case er.recursive:
				if needsRecursiveRefresh(event) {
					scheduleRefresh()
				}
			case fileutil.IsYAMLFile(event.Name):
				er.handleFSEvent(ctx, event)
			}
		case <-refresh:
			refresh = nil
			if err := er.refreshRegistry(ctx); err != nil {
				logger.Error(ctx, "Failed to refresh DAG registry", tag.Error(err))
			}
		case err, ok := <-er.watcher.Errors():
			if !ok {
				return
			}
			logger.Error(ctx, "Watcher error", tag.Error(err))
		}
	}
}

func needsRecursiveRefresh(event fsnotify.Event) bool {
	if fileutil.IsYAMLFile(event.Name) {
		return true
	}
	return event.Op&(fsnotify.Create|fsnotify.Remove|fsnotify.Rename) != 0
}

// handleFSEvent processes a filesystem event and emits a persis.DAGChangeEvent.
func (er *entryReaderImpl) handleFSEvent(ctx context.Context, event fsnotify.Event) {
	fileName := filepath.Base(event.Name)

	if event.Op&(fsnotify.Create|fsnotify.Write) != 0 {
		er.reloadDAGFile(ctx, fileName, event.Name)
		return
	}

	if event.Op&(fsnotify.Rename|fsnotify.Remove) != 0 {
		snapshot, err := er.dagSource.snapshot(ctx, fileName)
		if err != nil {
			logger.Error(ctx, "DAG load failed",
				tag.Error(err),
				tag.File(event.Name))
			return
		}
		if snapshot.exists {
			er.applyDAGFileSnapshot(ctx, fileName, snapshot)
			logger.Info(ctx, "DAG added/updated", tag.Name(fileName))
			return
		}

		er.removeDAGFile(ctx, fileName)
	}
}

// reloadDAGFile reloads a create/write event when the file still snapshots as present.
func (er *entryReaderImpl) reloadDAGFile(ctx context.Context, fileName, eventName string) {
	snapshot, err := er.dagSource.snapshot(ctx, fileName)
	if err != nil {
		logger.Error(ctx, "DAG load failed",
			tag.Error(err),
			tag.File(eventName))
		return
	}
	if !snapshot.exists {
		return
	}

	er.applyDAGFileSnapshot(ctx, fileName, snapshot)
	logger.Info(ctx, "DAG added/updated", tag.Name(fileName))
}

// applyDAGFileSnapshot stores a loaded DAG and emits the matching add/update events.
func (er *entryReaderImpl) applyDAGFileSnapshot(ctx context.Context, fileName string, snapshot dagFileSnapshot) {
	dag := snapshot.dag
	// Determine add vs update by checking registry before updating
	er.lock.Lock()
	oldDAG, existed := er.registry[fileName]
	var oldDAGName string
	if existed && oldDAG.Name != dag.Name {
		oldDAGName = oldDAG.Name
	}
	er.registry[fileName] = dag
	if er.stamps == nil {
		er.stamps = make(map[string]dagFileStamp)
	}
	er.stamps[fileName] = snapshot.stamp
	er.lock.Unlock()

	// If the DAG name changed, emit delete for the old name first
	if oldDAGName != "" {
		er.sendEvent(ctx, persis.DAGChangeEvent{
			Type: persis.DAGChangeDeleted,
			DAGEntry: persis.DAGEntry{
				DefinitionID: definitionIDForFile(fileName),
				DAG:          oldDAG,
			},
		})
	}

	changeType := persis.DAGChangeAdded
	if existed && oldDAGName == "" {
		changeType = persis.DAGChangeUpdated
	}
	er.sendEvent(ctx, persis.DAGChangeEvent{
		Type: changeType,
		DAGEntry: persis.DAGEntry{
			DefinitionID: definitionIDForFile(fileName),
			DAG:          dag,
		},
	})
}

// removeDAGFile drops a confirmed-absent DAG file from the registry.
func (er *entryReaderImpl) removeDAGFile(ctx context.Context, fileName string) {
	// Capture DAG name from registry before deleting
	er.lock.Lock()
	dag, existed := er.registry[fileName]
	delete(er.registry, fileName)
	delete(er.stamps, fileName)
	er.lock.Unlock()

	if existed && dag != nil {
		er.sendEvent(ctx, persis.DAGChangeEvent{
			Type: persis.DAGChangeDeleted,
			DAGEntry: persis.DAGEntry{
				DefinitionID: definitionIDForFile(fileName),
				DAG:          dag,
			},
		})
	}
	logger.Info(ctx, "DAG removed", tag.Name(fileName))
}

// sendEvent sends a persis.DAGChangeEvent on the channel.
// Returns immediately if the entry reader is shutting down or the context is cancelled.
func (er *entryReaderImpl) sendEvent(ctx context.Context, event persis.DAGChangeEvent) {
	if er.events == nil {
		return
	}
	select {
	case er.events <- event:
	case <-er.quit:
	case <-ctx.Done():
	}
}

// Stop closes the watcher and prevents future event sends.
func (er *entryReaderImpl) Stop() {
	er.lock.Lock()
	defer er.lock.Unlock()

	er.closeOnce.Do(func() {
		close(er.quit)
		if er.watcher != nil {
			_ = er.watcher.Close()
		}
	})
}

// Entries returns the currently loaded DAG metadata.
func (er *entryReaderImpl) Entries() []persis.DAGEntry {
	er.lock.Lock()
	defer er.lock.Unlock()

	entries := make([]persis.DAGEntry, 0, len(er.registry))
	for fileName, dag := range er.registry {
		entries = append(entries, persis.DAGEntry{DefinitionID: definitionIDForFile(fileName), DAG: dag})
	}
	return entries
}

func (er *entryReaderImpl) Events() <-chan persis.DAGChangeEvent {
	return er.events
}

func definitionIDForFile(fileName string) string {
	base := filepath.Base(filepath.FromSlash(fileName))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func (er *entryReaderImpl) initRecursive(ctx context.Context) error {
	er.watcher = filenotify.New(time.Minute)

	scan, err := Discover(er.targetDir, DiscoveryOptions{Recursive: true})
	if err != nil {
		_ = er.watcher.Close()
		return fmt.Errorf("failed to initialize recursive DAGs: %w", err)
	}
	if err := er.addInitialWatches(scan.Dirs); err != nil {
		_ = er.watcher.Close()
		return err
	}

	state, err := er.loadRegistry(ctx)
	if err != nil {
		_ = er.watcher.Close()
		return fmt.Errorf("failed to initialize recursive DAGs: %w", err)
	}
	for _, issue := range state.issues {
		logger.Error(ctx, "DAG excluded from scheduler", tag.Error(errors.New(issue)))
	}

	er.lock.Lock()
	er.registry = state.dags
	er.stamps = state.stamps
	er.lock.Unlock()
	return nil
}

func (er *entryReaderImpl) refreshRegistry(ctx context.Context) error {
	scan, err := Discover(er.targetDir, DiscoveryOptions{Recursive: er.recursive})
	if err != nil {
		return err
	}
	er.syncWatches(ctx, scan.Dirs)

	state, err := er.loadRegistry(ctx)
	if err != nil {
		return err
	}
	for _, issue := range state.issues {
		logger.Error(ctx, "DAG excluded from scheduler", tag.Error(errors.New(issue)))
	}

	events := er.replaceRegistry(state)
	for _, event := range events {
		er.sendEvent(ctx, event)
	}
	return nil
}

func (er *entryReaderImpl) syncWatches(ctx context.Context, dirs []string) {
	dirs, err := er.withBaseWatchDirs(dirs)
	if err != nil {
		logger.Error(ctx, "Failed to discover base config watches", tag.Error(err))
		return
	}
	next := make(map[string]struct{}, len(dirs))
	for _, dir := range dirs {
		next[dir] = struct{}{}
		if _, exists := er.watchedDirs[dir]; exists {
			continue
		}
		if err := er.watcher.Add(dir); err != nil {
			logger.Error(ctx, "Failed to watch DAG directory", tag.Dir(dir), tag.Error(err))
			continue
		}
		er.watchedDirs[dir] = struct{}{}
	}

	for dir := range er.watchedDirs {
		if _, exists := next[dir]; exists {
			continue
		}
		_ = er.watcher.Remove(dir)
		delete(er.watchedDirs, dir)
	}
}

func (er *entryReaderImpl) loadRegistry(ctx context.Context) (registryState, error) {
	paginator := pagination.NewPaginator(1, math.MaxInt)
	result, issues, err := er.dagRepository.List(ctx, persis.DAGListOptions{Paginator: &paginator})
	if err != nil {
		return registryState{}, err
	}

	dags := make(map[string]*ir.DAG, len(result.Items))
	stamps := make(map[string]dagFileStamp, len(result.Items))
	for _, listedDAG := range result.Items {
		if er.isBaseConfigEvent(fsnotify.Event{Name: listedDAG.Location}) {
			continue
		}
		if len(listedDAG.BuildErrors) > 0 {
			issues = append(issues,
				fmt.Sprintf("reading %s failed: %s", listedDAG.FileName(), errors.Join(listedDAG.BuildErrors...)))
			continue
		}

		relPath, err := filepath.Rel(er.targetDir, listedDAG.Location)
		if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
			issues = append(issues,
				fmt.Sprintf("DAG path is outside the discovery directory: %s", listedDAG.Location))
			continue
		}
		key := filepath.ToSlash(relPath)
		locator := key
		if !strings.Contains(locator, "/") {
			locator = "./" + locator
		}
		// Stat before loading: a write that lands in between must still look
		// like a change to the next refresh.
		info, err := os.Stat(listedDAG.Location)
		if err != nil {
			issues = append(issues, fmt.Sprintf("reading %s failed: %s", key, err))
			continue
		}
		dag, err := er.dagRepository.GetMetadata(ctx, locator)
		if err != nil {
			issues = append(issues, fmt.Sprintf("reading %s failed: %s", key, err))
			continue
		}

		dags[key] = dag
		stamps[key] = dagFileStamp{size: info.Size(), modTime: info.ModTime().UnixNano()}
	}
	sort.Strings(issues)
	return registryState{
		dags:   dags,
		stamps: stamps,
		issues: issues,
	}, nil
}

func (er *entryReaderImpl) replaceRegistry(state registryState) []persis.DAGChangeEvent {
	er.lock.Lock()
	defer er.lock.Unlock()

	oldKeys := sortedRegistryKeys(er.registry)
	newKeys := sortedRegistryKeys(state.dags)
	events := make([]persis.DAGChangeEvent, 0)
	for _, key := range oldKeys {
		if _, exists := state.dags[key]; exists {
			continue
		}
		if oldDAG := er.registry[key]; oldDAG != nil {
			events = append(events, persis.DAGChangeEvent{
				Type: persis.DAGChangeDeleted,
				DAGEntry: persis.DAGEntry{
					DefinitionID: definitionIDForFile(key),
					DAG:          oldDAG,
				},
			})
		}
	}
	for _, key := range newKeys {
		dag := state.dags[key]
		oldDAG, existed := er.registry[key]
		if !existed {
			events = append(events, persis.DAGChangeEvent{
				Type: persis.DAGChangeAdded,
				DAGEntry: persis.DAGEntry{
					DefinitionID: definitionIDForFile(key),
					DAG:          dag,
				},
			})
			continue
		}
		if oldDAG.Name != dag.Name {
			events = append(events,
				persis.DAGChangeEvent{Type: persis.DAGChangeDeleted, DAGEntry: persis.DAGEntry{DefinitionID: definitionIDForFile(key), DAG: oldDAG}},
				persis.DAGChangeEvent{Type: persis.DAGChangeAdded, DAGEntry: persis.DAGEntry{DefinitionID: definitionIDForFile(key), DAG: dag}},
			)
			continue
		}
		if er.stamps[key] != state.stamps[key] || !bytes.Equal(oldDAG.BaseConfigData, dag.BaseConfigData) {
			events = append(events, persis.DAGChangeEvent{
				Type: persis.DAGChangeUpdated,
				DAGEntry: persis.DAGEntry{
					DefinitionID: definitionIDForFile(key),
					DAG:          dag,
				},
			})
		}
	}

	er.registry = state.dags
	er.stamps = state.stamps
	return events
}

func sortedRegistryKeys(registry map[string]*ir.DAG) []string {
	keys := make([]string, 0, len(registry))
	for key := range registry {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// initialize loads existing YAML files through the same stable snapshot path as watcher events.
func (er *entryReaderImpl) initialize(ctx context.Context) error {
	// Note: This method expects the caller to already hold er.lock
	logger.Info(ctx, "Loading DAGs", tag.Dir(er.targetDir))
	fis, err := os.ReadDir(er.targetDir)
	if err != nil {
		logger.Error(ctx, "Failed to read DAG directory",
			tag.Dir(er.targetDir),
			tag.Error(err),
		)
		return err
	}

	var dags []string
	for _, fi := range fis {
		if fileutil.IsYAMLFile(fi.Name()) && !er.isBaseConfigEvent(fsnotify.Event{Name: filepath.Join(er.targetDir, fi.Name())}) {
			snapshot, err := er.dagSource.snapshot(ctx, fi.Name())
			if err != nil {
				logger.Error(ctx, "DAG load failed",
					tag.Error(err),
					tag.Name(fi.Name()))
				continue
			}
			if !snapshot.exists {
				continue
			}
			er.registry[fi.Name()] = snapshot.dag
			er.stamps[fi.Name()] = snapshot.stamp
			dags = append(dags, fi.Name())
		}
	}

	logger.Debug(ctx, "DAGs loaded", slog.String("dags", strings.Join(dags, ",")))
	return nil
}
