// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
)

const maxInt64AsUint = ^uint64(0) >> 1
const maxIntValue = int(^uint(0) >> 1)

// ResolveRuntimeParamsOptions controls how a DAG is reloaded for runtime param validation.
type ResolveRuntimeParamsOptions struct {
	BaseConfig string
}

// ResolveRuntimeParams reloads a DAG from its source with runtime params applied.
// It is intended for entry points that need the same coercion and validation path
// as execution without duplicating loader setup.
func ResolveRuntimeParams(ctx context.Context, dag *core.DAG, params any, opts ResolveRuntimeParamsOptions) (*core.DAG, error) {
	if dag == nil {
		return nil, nil
	}

	loadOpts, err := runtimeParamLoadOptions(dag, params, opts)
	if err != nil {
		return nil, err
	}

	switch {
	case dag.Location != "":
		return Load(ctx, dag.Location, loadOpts...)
	case len(dag.YamlData) > 0:
		return LoadYAML(ctx, dag.YamlData, loadOpts...)
	default:
		return nil, fmt.Errorf("DAG source is required to resolve runtime params")
	}
}

func runtimeParamLoadOptions(dag *core.DAG, params any, opts ResolveRuntimeParamsOptions) ([]LoadOption, error) {
	loadOpts := make([]LoadOption, 0, 3)

	switch value := params.(type) {
	case nil:
		loadOpts = append(loadOpts, WithParams(""))
	case string:
		loadOpts = append(loadOpts, WithParams(value))
	case []string:
		loadOpts = append(loadOpts, WithParams(value))
	default:
		return nil, fmt.Errorf("invalid runtime params type %T", params)
	}

	if dag.Name != "" {
		loadOpts = append(loadOpts, WithName(dag.Name))
	}
	if opts.BaseConfig != "" {
		loadOpts = append(loadOpts, WithBaseConfig(opts.BaseConfig))
	}
	if len(dag.BaseConfigData) > 0 {
		loadOpts = append(loadOpts, WithBaseConfigContent(dag.BaseConfigData))
	}

	return loadOpts, nil
}

func resolveLegacyEntries(plan *dagParamPlan, rawParams string, paramsList []string) ([]dagParamEntry, error) {
	overridePairs, err := parseOverridePairs(rawParams, paramsList)
	if err != nil {
		return nil, err
	}

	entries, err := applyOverridePairs(plan.entries, overridePairs)
	if err != nil {
		return nil, err
	}

	if plan.schema == nil {
		return entries, nil
	}

	entries, err = validateSchemaBackedEntries(entries, plan.schema, plan.schemaProperties, plan.schemaOrder, false, false)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func parseOverridePairs(rawParams string, paramsList []string) ([]paramPair, error) {
	noEvalCtx := BuildContext{opts: BuildOpts{Flags: BuildFlagNoEval}}
	var pairs []paramPair
	if rawParams != "" {
		parsed, err := parseParamValue(noEvalCtx, rawParams)
		if err != nil {
			return nil, core.NewValidationError("params", rawParams, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
		}
		pairs = append(pairs, parsed...)
	}
	if len(paramsList) > 0 {
		parsed, err := parseParamValue(noEvalCtx, paramsList)
		if err != nil {
			return nil, core.NewValidationError("params", paramsList, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
		}
		pairs = append(pairs, parsed...)
	}
	return pairs, nil
}

func applyOverridePairs(entries []dagParamEntry, override []paramPair) ([]dagParamEntry, error) {
	result := cloneParamEntries(entries)
	positionalIndex := 0

	for _, pair := range override {
		if pair.Name == "" {
			if len(entries) == 0 {
				result = append(result, dagParamEntry{Value: pair.Value, HasValue: true})
				continue
			}
			if positionalIndex >= len(entries) {
				return nil, fmt.Errorf("too many positional params: expected at most %d, got %d", len(entries), positionalIndex+1)
			}
			result[positionalIndex].Value = pair.Value
			result[positionalIndex].HasValue = true
			positionalIndex++
			continue
		}

		found := false
		for i := range result {
			if result[i].Name != pair.Name {
				continue
			}
			result[i].Value = pair.Value
			result[i].HasValue = true
			found = true
			break
		}
		if !found {
			result = append(result, dagParamEntry{Name: pair.Name, Value: pair.Value, HasValue: true})
		}
	}

	return result, nil
}

func runtimePairsFromEntries(entries []dagParamEntry) []paramPair {
	pairs := make([]paramPair, 0, len(entries))
	for _, entry := range entries {
		if !entry.HasValue {
			continue
		}
		pairs = append(pairs, paramPair{Name: entry.Name, Value: entry.Value})
	}
	for i := range pairs {
		if pairs[i].Name == "" {
			pairs[i].Name = strconv.Itoa(i + 1)
		}
	}
	return pairs
}

func normalizeTypedParamValue(value any, paramType string) (any, error) {
	switch paramType {
	case core.ParamDefTypeString:
		return stringifyUntypedValue(value), nil

	case core.ParamDefTypeInteger:
		switch v := value.(type) {
		case string:
			return coerceStringToType(v, paramType)
		default:
			number, err := toInt64(value)
			if err != nil {
				return nil, err
			}
			return number, nil
		}

	case core.ParamDefTypeNumber:
		switch v := value.(type) {
		case string:
			return coerceStringToType(v, paramType)
		default:
			number, err := toFloat64(value)
			if err != nil {
				return nil, err
			}
			return number, nil
		}

	case core.ParamDefTypeBoolean:
		switch v := value.(type) {
		case string:
			return coerceStringToType(v, paramType)
		case bool:
			return v, nil
		default:
			return nil, fmt.Errorf("expected a boolean")
		}

	default:
		return nil, fmt.Errorf("unsupported type %q", paramType)
	}
}

func coerceStringToType(value, paramType string) (any, error) {
	switch paramType {
	case core.ParamDefTypeString:
		return value, nil

	case core.ParamDefTypeInteger:
		number, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot coerce %q to integer", value)
		}
		return number, nil

	case core.ParamDefTypeNumber:
		number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err != nil {
			return nil, fmt.Errorf("cannot coerce %q to number", value)
		}
		return number, nil

	case core.ParamDefTypeBoolean:
		switch {
		case strings.EqualFold(value, "true"):
			return true, nil
		case strings.EqualFold(value, "false"):
			return false, nil
		default:
			return nil, fmt.Errorf("cannot coerce %q to boolean", value)
		}

	default:
		return nil, fmt.Errorf("unsupported type %q", paramType)
	}
}

func toFloat64(value any) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int8:
		return float64(v), nil
	case int16:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint8:
		return float64(v), nil
	case uint16:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	default:
		return 0, fmt.Errorf("got %T", value)
	}
}

func toInt(value any) (int, error) {
	number, err := toInt64(value)
	if err != nil {
		return 0, err
	}
	if number < -int64(maxIntValue)-1 || number > int64(maxIntValue) {
		return 0, fmt.Errorf("integer overflow")
	}
	return int(number), nil
}

func toInt64(value any) (int64, error) {
	switch v := value.(type) {
	case int:
		return int64(v), nil
	case int8:
		return int64(v), nil
	case int16:
		return int64(v), nil
	case int32:
		return int64(v), nil
	case int64:
		return v, nil
	case uint:
		return int64FromUint64(uint64(v))
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64FromUint64(v)
	case float32:
		if float32(int64(v)) != v {
			return 0, fmt.Errorf("expected an integer")
		}
		return int64(v), nil
	case float64:
		if float64(int64(v)) != v {
			return 0, fmt.Errorf("expected an integer")
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("got %T", value)
	}
}

func int64FromUint64(value uint64) (int64, error) {
	if value > maxInt64AsUint {
		return 0, fmt.Errorf("integer overflow")
	}

	number, err := strconv.ParseInt(strconv.FormatUint(value, 10), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("integer overflow")
	}
	return number, nil
}

func cloneParamEntries(entries []dagParamEntry) []dagParamEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := make([]dagParamEntry, len(entries))
	copy(cloned, entries)
	return cloned
}

func cloneParamDefs(defs []core.ParamDef) []core.ParamDef {
	if len(defs) == 0 {
		return nil
	}
	cloned := make([]core.ParamDef, len(defs))
	copy(cloned, defs)
	for i := range cloned {
		if len(cloned[i].Enum) > 0 {
			cloned[i].Enum = append([]any(nil), cloned[i].Enum...)
		}
	}
	return cloned
}

func containsTypedValue(values []any, target any) bool {
	for _, item := range values {
		if reflect.DeepEqual(item, target) {
			return true
		}
	}
	return false
}
