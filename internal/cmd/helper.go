// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bytes"
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/parser"

	"github.com/dagu-org/dagu/internal/cmn/logger"
	"github.com/dagu-org/dagu/internal/cmn/logger/tag"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
)

// parseTriggerTypeParam parses and validates the trigger-type flag from the command context.
// Returns TriggerTypeUnknown (zero value) if the flag is empty, otherwise validates
// that the provided value is a known trigger type.
func parseTriggerTypeParam(ctx *Context) (core.TriggerType, error) {
	triggerTypeStr, err := ctx.StringParam("trigger-type")
	if err != nil {
		logger.Debug(ctx, "Failed to read trigger-type flag", tag.Error(err))
	}
	if triggerTypeStr == "" {
		return core.TriggerTypeUnknown, nil
	}

	triggerType := core.ParseTriggerType(triggerTypeStr)
	if triggerType == core.TriggerTypeUnknown {
		return core.TriggerTypeUnknown, fmt.Errorf(
			"invalid trigger-type %q: must be one of scheduler, manual, webhook, subdag, retry, catchup",
			triggerTypeStr,
		)
	}

	return triggerType, nil
}

// restoreDAGFromStatus restores a DAG from a previous run's status and YAML.
// It restores params from the status, loads dotenv, and rebuilds fields excluded
// from JSON serialization (env, shell, workingDir, registryAuths, etc.).
func restoreDAGFromStatus(ctx context.Context, dag *core.DAG, status *exec.DAGRunStatus) (*core.DAG, error) {
	dag.Params = quoteParamValues(status.ParamsList)
	dag.LoadDotEnv(ctx)
	return rebuildDAGFromYAML(ctx, dag)
}

// quoteParamValues quotes the value portion of each parameter so that
// values containing spaces survive re-parsing by parseStringParams.
//
// ParamsList stores unquoted "key=value" strings (produced by paramPair.String()),
// but the rebuild path feeds them back through parseStringParams which splits
// on whitespace. Quoting each value prevents that re-split.
func quoteParamValues(params []string) []string {
	quoted := make([]string, len(params))
	for i, p := range params {
		if k, v, ok := strings.Cut(p, "="); ok {
			quoted[i] = k + "=" + strconv.Quote(v)
		} else {
			quoted[i] = strconv.Quote(p)
		}
	}
	return quoted
}

// rebuildDAGFromYAML rebuilds a DAG from its YamlData using the spec loader.
// This populates fields excluded from JSON serialization (json:"-") and must be
// called after LoadDotEnv() so dotenv values are available during rebuild.
//
// The function preserves all JSON-serialized fields from the original DAG and
// only copies JSON-excluded fields (Env, Params, ParamsJSON, SMTP, SSH,
// RegistryAuths) from the rebuilt DAG.
func rebuildDAGFromYAML(ctx context.Context, dag *core.DAG) (*core.DAG, error) {
	if len(dag.YamlData) == 0 {
		return dag, nil
	}

	// Build env map from dag.Env (includes dotenv values if LoadDotEnv was called).
	buildEnv := make(map[string]string, len(dag.Env))
	for _, env := range dag.Env {
		if k, v, ok := strings.Cut(env, "="); ok {
			buildEnv[k] = v
		}
	}

	loadOpts := []spec.LoadOption{
		spec.WithParams(dag.Params),
		spec.WithBuildEnv(buildEnv),
		spec.SkipSchemaValidation(),
	}

	if dag.Name != "" {
		loadOpts = append(loadOpts, spec.WithName(dag.Name))
	}

	fresh, err := spec.LoadYAML(ctx, dag.YamlData, loadOpts...)
	if err != nil {
		return nil, err
	}

	// Copy only fields excluded from JSON serialization (json:"-").
	// All other fields (Queue, WorkerSelector, HandlerOn, Steps, Tags, etc.)
	// are already correctly stored in dag.json and must be preserved.
	dag.Env = fresh.Env
	dag.Params = fresh.Params
	dag.ParamsJSON = fresh.ParamsJSON
	dag.SMTP = fresh.SMTP
	dag.SSH = fresh.SSH
	dag.RegistryAuths = fresh.RegistryAuths

	core.InitializeDefaults(dag)

	return dag, nil
}

// --- YAML sync helpers ---
//
// syncYAMLData updates dag.YamlData to reflect runtime modifications to Tags,
// Queue, and Name that were applied after the DAG was loaded from its YAML file.
//
// Call this after all overrides are applied and before the DAG is persisted or
// dispatched. For multi-document YAML, only the first document is patched;
// remaining documents are preserved verbatim via the AST.
//
// When no overrides differ from the YAML content, YamlData is left byte-equal
// to the original (fast path — zero reformatting). On any error, YamlData is
// left unchanged.
//
// Synced fields: Tags, Queue, Name.
// Mutation sites: enqueue.go (queue, tags), start.go (name, tags).
func syncYAMLData(dag *core.DAG) error {
	if len(dag.YamlData) == 0 {
		return nil
	}

	// Decode first document as an ordered map to preserve key ordering.
	var firstDoc yaml.MapSlice
	if err := yaml.NewDecoder(bytes.NewReader(dag.YamlData)).Decode(&firstDoc); err != nil {
		return fmt.Errorf("decode first document: %w", err)
	}

	// Fast path: check whether any field actually differs.
	// Tags: normalize both sides through core.NewTags for case handling.
	yamlNorm := core.NewTags(extractTagStringsFromMapSlice(firstDoc)).Strings()
	dagNorm := dag.Tags.Strings()
	tagsChanged := !slices.Equal(yamlNorm, dagNorm)
	if tagsChanged {
		// Order-independent comparison as a fallback.
		slices.Sort(yamlNorm)
		slices.Sort(dagNorm)
		tagsChanged = !slices.Equal(yamlNorm, dagNorm)
	}

	// Name: only compare if YAML already has a "name" key.
	nameChanged := false
	if yamlName, hasName := getMapSliceString(firstDoc, "name"); hasName {
		nameChanged = dag.Name != "" && dag.Name != yamlName
	}

	// Queue: check existing key and detect runtime addition.
	queueChanged := false
	if yamlQueue, hasQueue := getMapSliceString(firstDoc, "queue"); hasQueue {
		queueChanged = dag.Queue != "" && dag.Queue != yamlQueue
	} else {
		queueChanged = dag.Queue != ""
	}

	if !tagsChanged && !nameChanged && !queueChanged {
		return nil
	}

	// Patch the ordered map.
	if tagsChanged {
		if dag.Tags == nil {
			removeMapSliceKey(&firstDoc, "tags")
		} else {
			setMapSliceValue(&firstDoc, "tags", dag.Tags.Strings())
		}
	}
	if nameChanged {
		setMapSliceValue(&firstDoc, "name", dag.Name)
	}
	if queueChanged {
		setMapSliceValue(&firstDoc, "queue", dag.Queue)
	}

	// Marshal patched first document.
	patched, err := yaml.Marshal(firstDoc)
	if err != nil {
		return fmt.Errorf("marshal patched document: %w", err)
	}

	// Check for multi-document YAML using the parser for safe boundary detection.
	// parser.ParseBytes correctly treats "---" inside block scalars as content.
	file, err := parser.ParseBytes(dag.YamlData, 0)
	if err != nil || len(file.Docs) <= 1 {
		dag.YamlData = patched
		return nil
	}

	// Reassemble: patched first doc + remaining docs from AST (verbatim).
	var buf bytes.Buffer
	buf.Grow(len(dag.YamlData))
	buf.Write(patched)
	for _, doc := range file.Docs[1:] {
		buf.WriteString("---\n")
		buf.WriteString(doc.String())
		buf.WriteString("\n")
	}
	dag.YamlData = buf.Bytes()
	return nil
}

// extractTagStringsFromMapSlice extracts tag strings from the "tags" field of a
// MapSlice, handling all 4 YAML formats that the spec loader accepts:
// space-separated string, map, array of strings, and array of maps.
func extractTagStringsFromMapSlice(ms yaml.MapSlice) []string {
	raw, ok := getMapSliceValue(ms, "tags")
	if !ok {
		return nil
	}

	switch v := raw.(type) {
	case string:
		// Space-separated: "foo=bar zoo=baz"
		if fields := strings.Fields(v); len(fields) > 0 {
			return fields
		}
		return nil

	case []any:
		var tags []string
		for _, item := range v {
			switch t := item.(type) {
			case string:
				tags = append(tags, t)
			default:
				tags = append(tags, mapToTagStrings(t)...)
			}
		}
		return tags

	default:
		if t := mapToTagStrings(v); len(t) > 0 {
			return t
		}
	}

	return nil
}

// mapToTagStrings converts a map-like value (yaml.MapSlice or map[string]any)
// to "key=value" tag strings.
func mapToTagStrings(v any) []string {
	switch m := v.(type) {
	case yaml.MapSlice:
		tags := make([]string, 0, len(m))
		for _, mi := range m {
			tags = append(tags, fmt.Sprintf("%v=%v", mi.Key, mi.Value))
		}
		return tags
	case map[string]any:
		tags := make([]string, 0, len(m))
		for k, val := range m {
			tags = append(tags, fmt.Sprintf("%v=%v", k, val))
		}
		return tags
	}
	return nil
}

// mapSliceKeyEquals checks if a MapItem's key matches a string.
func mapSliceKeyEquals(key any, search string) bool {
	s, ok := key.(string)
	if ok {
		return s == search
	}
	return fmt.Sprint(key) == search
}

// getMapSliceValue returns the value for a key in a MapSlice.
func getMapSliceValue(ms yaml.MapSlice, key string) (any, bool) {
	for _, item := range ms {
		if mapSliceKeyEquals(item.Key, key) {
			return item.Value, true
		}
	}
	return nil, false
}

// getMapSliceString returns the string value for a key, or ("", false) if
// the key is missing or the value is not a string.
func getMapSliceString(ms yaml.MapSlice, key string) (string, bool) {
	v, ok := getMapSliceValue(ms, key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// setMapSliceValue updates an existing key or appends a new entry.
func setMapSliceValue(ms *yaml.MapSlice, key string, value any) {
	for i := range *ms {
		if mapSliceKeyEquals((*ms)[i].Key, key) {
			(*ms)[i].Value = value
			return
		}
	}
	*ms = append(*ms, yaml.MapItem{Key: key, Value: value})
}

// removeMapSliceKey removes a key from the MapSlice.
func removeMapSliceKey(ms *yaml.MapSlice, key string) {
	for i := range *ms {
		if mapSliceKeyEquals((*ms)[i].Key, key) {
			*ms = slices.Delete(*ms, i, i+1)
			return
		}
	}
}

// extractDAGName extracts the DAG name from a file path or name.
// If the input is a file path (.yaml or .yml), it loads the DAG metadata
// to extract the name. Otherwise, it returns the input as-is.
func extractDAGName(ctx *Context, name string) (string, error) {
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		return name, nil
	}

	dagStore, err := ctx.dagStore(dagStoreConfig{})
	if err != nil {
		return "", fmt.Errorf("failed to initialize DAG store: %w", err)
	}

	dag, err := dagStore.GetMetadata(ctx, name)
	if err != nil {
		return "", fmt.Errorf("failed to read DAG metadata from file %s: %w", name, err)
	}

	return dag.Name, nil
}
