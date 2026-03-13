// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"fmt"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

type dagParamKind uint8

const (
	dagParamKindLegacy dagParamKind = iota
	dagParamKindExternalSchema
)

type dagParamPlan struct {
	kind             dagParamKind
	entries          []dagParamEntry
	paramDefs        []core.ParamDef
	schema           *jsonschema.Resolved
	schemaOrder      []string
	schemaProperties map[string]*jsonschema.Schema
}

type dagParamEntry struct {
	Name     string
	Value    string
	HasValue bool
}

func buildDAGParamsResult(ctx BuildContext, d *dag) (*paramsResult, error) {
	plan, err := buildDAGParamPlan(ctx, d)
	if err != nil {
		return nil, err
	}

	defaultPairs := runtimePairsFromEntries(plan.entries)
	defaultParts := make([]string, 0, len(defaultPairs))
	for _, pair := range defaultPairs {
		defaultParts = append(defaultParts, pair.Escaped())
	}
	defaultParams := strings.Join(defaultParts, " ")

	finalEntries := cloneParamEntries(plan.entries)
	if ctx.opts.Has(BuildFlagValidateRuntimeParams) || ctx.opts.Parameters != "" || len(ctx.opts.ParametersList) > 0 {
		switch plan.kind {
		case dagParamKindExternalSchema:
			finalEntries, err = resolveExternalSchemaEntries(plan, ctx.opts.Parameters, ctx.opts.ParametersList)
		default:
			finalEntries, err = resolveLegacyEntries(plan, ctx.opts.Parameters, ctx.opts.ParametersList)
		}
		if err != nil {
			return nil, err
		}
	}

	finalPairs := runtimePairsFromEntries(finalEntries)
	params := make([]string, 0, len(finalPairs))
	for _, pair := range finalPairs {
		params = append(params, pair.String())
	}

	rawOverride := ctx.opts.Parameters
	if rawOverride == "" && len(ctx.opts.ParametersList) == 1 {
		rawOverride = ctx.opts.ParametersList[0]
	}
	paramsJSON, err := buildResolvedParamsJSON(finalPairs, rawOverride)
	if err != nil {
		return nil, err
	}

	return &paramsResult{
		Params:        params,
		DefaultParams: defaultParams,
		ParamDefs:     cloneParamDefs(plan.paramDefs),
		ParamsJSON:    paramsJSON,
	}, nil
}

func buildDAGParamPlan(ctx BuildContext, d *dag) (*dagParamPlan, error) {
	if extractSchemaReference(d.Params) != "" {
		if ctx.opts.Has(BuildFlagSkipSchemaValidation) {
			return buildLegacyParamPlan(extractSchemaValues(d.Params))
		}
		return buildExternalSchemaParamPlan(d.Params, d.WorkingDir, ctx.file)
	}
	return buildLegacyParamPlan(d.Params)
}

func buildLegacyParamPlan(input any) (*dagParamPlan, error) {
	noEvalCtx := BuildContext{opts: BuildOpts{Flags: BuildFlagNoEval}}
	plan := &dagParamPlan{kind: dagParamKindLegacy}

	switch v := input.(type) {
	case nil:
		return plan, nil

	case string:
		pairs, err := parseStringParams(noEvalCtx, v)
		if err != nil {
			return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
		}
		appendLegacyPairs(plan, pairs)
		return plan, nil

	case []string:
		pairs, err := parseListParams(noEvalCtx, v)
		if err != nil {
			return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
		}
		appendLegacyPairs(plan, pairs)
		return plan, nil

	case map[string]any:
		pairs, err := parseMapParams(noEvalCtx, []any{v})
		if err != nil {
			return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
		}
		appendLegacyPairs(plan, pairs)
		return plan, nil

	case []any:
		var hasInlineDefinitions bool
		for _, item := range v {
			switch value := item.(type) {
			case string:
				pairs, err := parseStringParams(noEvalCtx, value)
				if err != nil {
					return nil, core.NewValidationError("params", item, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
				}
				appendLegacyPairs(plan, pairs)

			case map[string]any:
				name, defMap, ok, err := detectInlineParamDefinition(value)
				if err != nil {
					return nil, core.NewValidationError("params", item, err)
				}
				if ok {
					hasInlineDefinitions = true
					paramDef, entry, err := parseInlineParamDefinition(name, defMap)
					if err != nil {
						return nil, core.NewValidationError("params", item, err)
					}
					plan.paramDefs = append(plan.paramDefs, paramDef)
					plan.entries = append(plan.entries, entry)
					continue
				}

				pairs, err := parseMapParams(noEvalCtx, []any{value})
				if err != nil {
					return nil, core.NewValidationError("params", item, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
				}
				appendLegacyPairs(plan, pairs)

			default:
				return nil, core.NewValidationError("params", item, fmt.Errorf("%w: %T", ErrInvalidParamValue, item))
			}
		}

		if hasInlineDefinitions {
			var err error
			plan.schema, plan.schemaProperties, plan.schemaOrder, err = compileInlineParamSchema(plan.paramDefs)
			if err != nil {
				return nil, err
			}
			plan.entries, err = validateSchemaBackedEntries(plan.entries, plan.schema, plan.schemaProperties, plan.schemaOrder, true, false)
			if err != nil {
				return nil, err
			}
		}

		return plan, nil

	default:
		return nil, core.NewValidationError("params", v, fmt.Errorf("%w: %T", ErrInvalidParamValue, v))
	}
}

func buildExternalSchemaParamPlan(input any, workingDir, dagLocation string) (*dagParamPlan, error) {
	resolvedSchema, err := resolveSchemaFromParams(input, workingDir, dagLocation)
	if err != nil {
		return nil, fmt.Errorf("failed to get JSON schema: %w", err)
	}
	if resolvedSchema == nil {
		return &dagParamPlan{kind: dagParamKindExternalSchema}, nil
	}

	values := extractSchemaValues(input)
	noEvalCtx := BuildContext{opts: BuildOpts{Flags: BuildFlagNoEval}}
	basePairs, err := parseParamValue(noEvalCtx, values)
	if err != nil {
		return nil, core.NewValidationError("params", values, fmt.Errorf("%w: %s", ErrInvalidParamValue, err))
	}

	root := resolvedSchema.Schema()
	schemaOrder := topLevelSchemaOrder(root)
	schemaProperties := map[string]*jsonschema.Schema{}
	if root != nil {
		for name, property := range root.Properties {
			schemaProperties[name] = property
		}
	}

	typedDefaults, err := schemaPairsToMap(basePairs, schemaProperties, true)
	if err != nil {
		return nil, err
	}
	typedDefaults, err = validateSchemaMap(typedDefaults, resolvedSchema, true)
	if err != nil {
		return nil, err
	}

	plan := &dagParamPlan{
		kind:             dagParamKindExternalSchema,
		schema:           resolvedSchema,
		schemaOrder:      schemaOrder,
		schemaProperties: schemaProperties,
		entries:          entriesFromTypedMap(typedDefaults, schemaOrder),
	}

	paramDefs, ok := deriveExternalSchemaParamDefs(root, typedDefaults)
	if ok {
		plan.paramDefs = paramDefs
	}

	return plan, nil
}

func appendLegacyPairs(plan *dagParamPlan, pairs []paramPair) {
	for _, pair := range pairs {
		plan.entries = append(plan.entries, dagParamEntry{
			Name:     pair.Name,
			Value:    pair.Value,
			HasValue: true,
		})
		plan.paramDefs = append(plan.paramDefs, core.ParamDef{
			Name:    pair.Name,
			Type:    core.ParamDefTypeString,
			Default: pair.Value,
		})
	}
}

func validateSchemaBackedEntries(entries []dagParamEntry, schema *jsonschema.Resolved, schemaProperties map[string]*jsonschema.Schema, schemaOrder []string, metadataMode bool, allowSchemaFallbackJSON bool) ([]dagParamEntry, error) {
	namedPairs := make([]paramPair, 0, len(entries))
	for _, entry := range entries {
		if !entry.HasValue || entry.Name == "" {
			continue
		}
		namedPairs = append(namedPairs, paramPair{Name: entry.Name, Value: entry.Value})
	}

	typedMap, err := schemaPairsToMap(namedPairs, schemaProperties, allowSchemaFallbackJSON)
	if err != nil {
		return nil, err
	}

	typedMap, err = validateSchemaMap(typedMap, schema, metadataMode)
	if err != nil {
		return nil, err
	}

	return mergeTypedMapIntoEntries(entries, typedMap, schemaOrder), nil
}
