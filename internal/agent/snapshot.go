// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/dagucloud/dagu/internal/core"
)

const (
	// SnapshotVersion is the current snapshot envelope version.
	SnapshotVersion = 1
	// DefaultSnapshotMaxBytes is the maximum compressed payload size for a worker snapshot.
	DefaultSnapshotMaxBytes = 4 * 1024 * 1024
	// maxUncompressedSnapshotBytes bounds worker-side gzip expansion during decode.
	maxUncompressedSnapshotBytes = DefaultSnapshotMaxBytes
)

var (
	ErrSnapshotStoreReadOnly     = errors.New("snapshot store is read-only")
	ErrUnsupportedSnapshotWire   = errors.New("unsupported snapshot version")
	ErrSnapshotConfigUnavailable = errors.New("agent config store not available")
	ErrSnapshotModelsUnavailable = errors.New("agent model store not available")
)

// Snapshot carries execution-scoped agent settings for distributed workers.
type Snapshot struct {
	Version int             `json:"version"`
	Config  *Config         `json:"config,omitempty"`
	Models  []*ModelConfig  `json:"models,omitempty"`
	Souls   []*Soul         `json:"souls,omitempty"`
	Memory  *MemorySnapshot `json:"memory,omitempty"`
}

// MemorySnapshot carries read-only memory content for distributed workers.
type MemorySnapshot struct {
	Global string            `json:"global,omitempty"`
	PerDAG map[string]string `json:"perDAG,omitempty"`
}

// SnapshotStores groups store dependencies used to build a worker snapshot.
type SnapshotStores struct {
	ConfigStore ConfigStore
	ModelStore  ModelStore
	SoulStore   SoulStore
	MemoryStore MemoryStore
}

// DAGResolver resolves a DAG by name for snapshot graph traversal.
type DAGResolver func(ctx context.Context, name string) (*core.DAG, error)

// SnapshotBuildOptions configure worker snapshot construction.
type SnapshotBuildOptions struct {
	ResolveDAG DAGResolver
	MaxBytes   int
}

// SnapshotReadOnlyMemoryStore marks a memory store as read-only execution context.
type SnapshotReadOnlyMemoryStore interface {
	MemoryReadOnly() bool
}

// MarshalSnapshot serializes and compresses a worker snapshot.
func MarshalSnapshot(snapshot *Snapshot) ([]byte, error) {
	if snapshot == nil {
		return nil, nil
	}

	copy := *snapshot
	copy.Version = SnapshotVersion

	raw, err := json.Marshal(&copy)
	if err != nil {
		return nil, fmt.Errorf("marshal agent snapshot: %w", err)
	}

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		_ = zw.Close()
		return nil, fmt.Errorf("compress agent snapshot: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("compress agent snapshot: %w", err)
	}

	return buf.Bytes(), nil
}

// UnmarshalSnapshot decompresses and parses a worker snapshot.
func UnmarshalSnapshot(payload []byte) (*Snapshot, error) {
	if len(payload) == 0 {
		return nil, nil
	}

	zr, err := gzip.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("open agent snapshot: %w", err)
	}
	defer func() {
		_ = zr.Close()
	}()

	raw, err := io.ReadAll(io.LimitReader(zr, maxUncompressedSnapshotBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read agent snapshot: %w", err)
	}
	if len(raw) > maxUncompressedSnapshotBytes {
		return nil, fmt.Errorf("agent snapshot exceeds max uncompressed size: > %d bytes", maxUncompressedSnapshotBytes)
	}

	var snapshot Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, fmt.Errorf("decode agent snapshot: %w", err)
	}
	if snapshot.Version != SnapshotVersion {
		return nil, fmt.Errorf("%w: got %d want %d", ErrUnsupportedSnapshotWire, snapshot.Version, SnapshotVersion)
	}

	return &snapshot, nil
}

// BuildSnapshotForDAG builds a bounded worker snapshot for the given DAG graph.
// It returns nil, nil when the DAG graph does not contain agent steps.
func BuildSnapshotForDAG(
	ctx context.Context,
	dag *core.DAG,
	stores SnapshotStores,
	opts SnapshotBuildOptions,
) ([]byte, error) {
	if dag == nil {
		return nil, fmt.Errorf("dag is required")
	}

	reqs, err := collectSnapshotRequirements(ctx, dag, opts.ResolveDAG)
	if err != nil {
		return nil, err
	}
	if !reqs.hasAgentSteps {
		return nil, nil
	}

	if stores.ConfigStore == nil {
		return nil, ErrSnapshotConfigUnavailable
	}
	if stores.ModelStore == nil {
		return nil, ErrSnapshotModelsUnavailable
	}

	cfg, err := stores.ConfigStore.Load(ctx)
	if err != nil {
		return nil, fmt.Errorf("load agent config for snapshot: %w", err)
	}
	if cfg == nil {
		return nil, fmt.Errorf("agent config store returned nil config")
	}

	if reqs.usesDefaultModel && cfg.DefaultModelID == "" {
		return nil, fmt.Errorf("default agent model is required for distributed agent execution")
	}
	if cfg.DefaultModelID != "" {
		reqs.modelIDs[cfg.DefaultModelID] = struct{}{}
	}
	snapshot := &Snapshot{
		Config: cfg,
	}

	models, err := snapshotModels(ctx, stores.ModelStore, reqs.sortedModelIDs())
	if err != nil {
		return nil, err
	}
	snapshot.Models = models

	if len(reqs.soulIDs) > 0 {
		if stores.SoulStore == nil {
			return nil, fmt.Errorf("agent soul store not available for distributed agent execution")
		}
		souls, err := snapshotSouls(ctx, stores.SoulStore, reqs.sortedSoulIDs())
		if err != nil {
			return nil, err
		}
		snapshot.Souls = souls
	}

	if reqs.needsMemory {
		if stores.MemoryStore == nil {
			return nil, fmt.Errorf("agent memory store not available for distributed agent execution")
		}
		memory, err := snapshotMemory(ctx, stores.MemoryStore, reqs.sortedMemoryDAGNames())
		if err != nil {
			return nil, err
		}
		snapshot.Memory = memory
	}

	data, err := MarshalSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	maxBytes := opts.MaxBytes
	if maxBytes <= 0 {
		maxBytes = DefaultSnapshotMaxBytes
	}
	if len(data) > maxBytes {
		return nil, fmt.Errorf("agent snapshot exceeds max size: %d > %d bytes", len(data), maxBytes)
	}

	return data, nil
}

// NeedsSnapshotForDAG reports whether the DAG graph contains any agent steps
// that require a distributed agent snapshot.
func NeedsSnapshotForDAG(ctx context.Context, dag *core.DAG, resolve DAGResolver) (bool, error) {
	if dag == nil {
		return false, nil
	}
	reqs, err := collectSnapshotRequirements(ctx, dag, resolve)
	if err != nil {
		return false, err
	}
	return reqs.hasAgentSteps, nil
}

func snapshotModels(ctx context.Context, store ModelStore, ids []string) ([]*ModelConfig, error) {
	models := make([]*ModelConfig, 0, len(ids))
	for _, id := range ids {
		model, err := store.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load model %q for snapshot: %w", id, err)
		}
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		return models[i].Name < models[j].Name
	})
	return models, nil
}

func snapshotSouls(ctx context.Context, store SoulStore, ids []string) ([]*Soul, error) {
	souls := make([]*Soul, 0, len(ids))
	for _, id := range ids {
		soul, err := store.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("load soul %q for snapshot: %w", id, err)
		}
		souls = append(souls, soul)
	}
	sort.Slice(souls, func(i, j int) bool {
		return souls[i].Name < souls[j].Name
	})
	return souls, nil
}

func snapshotMemory(ctx context.Context, store MemoryStore, dagNames []string) (*MemorySnapshot, error) {
	global, err := store.LoadGlobalMemory(ctx)
	if err != nil {
		return nil, fmt.Errorf("load global memory for snapshot: %w", err)
	}

	perDAG := make(map[string]string, len(dagNames))
	for _, dagName := range dagNames {
		content, err := store.LoadDAGMemory(ctx, dagName)
		if err != nil {
			return nil, fmt.Errorf("load DAG memory %q for snapshot: %w", dagName, err)
		}
		perDAG[dagName] = content
	}

	return &MemorySnapshot{
		Global: global,
		PerDAG: perDAG,
	}, nil
}

type snapshotRequirements struct {
	visited          map[string]struct{}
	dagNames         map[string]struct{}
	memoryDAGNames   map[string]struct{}
	modelIDs         map[string]struct{}
	soulIDs          map[string]struct{}
	hasAgentSteps    bool
	usesDefaultModel bool
	needsMemory      bool
}

func newSnapshotRequirements() *snapshotRequirements {
	return &snapshotRequirements{
		visited:        make(map[string]struct{}),
		dagNames:       make(map[string]struct{}),
		memoryDAGNames: make(map[string]struct{}),
		modelIDs:       make(map[string]struct{}),
		soulIDs:        make(map[string]struct{}),
	}
}

func collectSnapshotRequirements(ctx context.Context, dag *core.DAG, resolve DAGResolver) (*snapshotRequirements, error) {
	reqs := newSnapshotRequirements()
	if err := reqs.walk(ctx, dag, resolve); err != nil {
		return nil, err
	}
	return reqs, nil
}

func (r *snapshotRequirements) walk(ctx context.Context, dag *core.DAG, resolve DAGResolver) error {
	if dag == nil {
		return nil
	}
	if _, ok := r.visited[dag.Name]; ok {
		return nil
	}
	r.visited[dag.Name] = struct{}{}
	r.dagNames[dag.Name] = struct{}{}

	for _, step := range dag.Steps {
		if err := r.collectStep(ctx, dag, &step, resolve); err != nil {
			return err
		}
	}

	handlers := []*core.Step{
		dag.HandlerOn.Init,
		dag.HandlerOn.Failure,
		dag.HandlerOn.Success,
		dag.HandlerOn.Abort,
		dag.HandlerOn.Exit,
		dag.HandlerOn.Wait,
	}
	for _, handler := range handlers {
		if err := r.collectStep(ctx, dag, handler, resolve); err != nil {
			return err
		}
	}

	localNames := make([]string, 0, len(dag.LocalDAGs))
	for name := range dag.LocalDAGs {
		localNames = append(localNames, name)
	}
	sort.Strings(localNames)
	for _, name := range localNames {
		if err := r.walk(ctx, dag.LocalDAGs[name], resolve); err != nil {
			return err
		}
	}
	return nil
}

func (r *snapshotRequirements) collectStep(ctx context.Context, dag *core.DAG, step *core.Step, resolve DAGResolver) error {
	if step == nil {
		return nil
	}

	if step.Agent != nil {
		r.hasAgentSteps = true
		if step.Agent.Model == "" {
			r.usesDefaultModel = true
		} else {
			r.modelIDs[step.Agent.Model] = struct{}{}
		}
		if soulID := strings.TrimSpace(step.Agent.Soul); soulID != "" {
			r.soulIDs[soulID] = struct{}{}
		}
		if step.Agent.Memory != nil && step.Agent.Memory.Enabled {
			r.needsMemory = true
			if dag != nil && dag.Name != "" {
				r.memoryDAGNames[dag.Name] = struct{}{}
			}
		}
	}

	if step.SubDAG == nil {
		return nil
	}

	target := strings.TrimSpace(step.SubDAG.Name)
	if target == "" || strings.Contains(target, "${") {
		return nil
	}

	if dag != nil && dag.LocalDAGs != nil {
		if local := dag.LocalDAGs[target]; local != nil {
			return r.walk(ctx, local, resolve)
		}
	}

	if resolve == nil {
		return nil
	}
	child, err := resolve(ctx, target)
	if err != nil {
		return fmt.Errorf("resolve subdag %q for snapshot: %w", target, err)
	}
	return r.walk(ctx, child, resolve)
}

func (r *snapshotRequirements) sortedModelIDs() []string {
	return sortedKeys(r.modelIDs)
}

func (r *snapshotRequirements) sortedSoulIDs() []string {
	return sortedKeys(r.soulIDs)
}

func (r *snapshotRequirements) sortedMemoryDAGNames() []string {
	return sortedKeys(r.memoryDAGNames)
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
