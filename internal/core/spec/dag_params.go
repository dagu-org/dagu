// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"sort"
	"strconv"
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

func detectInlineParamDefinition(item map[string]any) (string, map[string]any, bool, error) {
	if len(item) != 1 {
		for _, value := range item {
			if _, ok := value.(map[string]any); ok {
				return "", nil, false, fmt.Errorf("inline parameter definitions must be single-key maps")
			}
		}
		return "", nil, false, nil
	}

	for name, value := range item {
		nested, ok := value.(map[string]any)
		if !ok {
			return "", nil, false, nil
		}
		return name, nested, true, nil
	}

	return "", nil, false, nil
}

func parseInlineParamDefinition(name string, raw map[string]any) (core.ParamDef, dagParamEntry, error) {
	def := core.ParamDef{
		Name: name,
		Type: core.ParamDefTypeString,
	}
	entry := dagParamEntry{Name: name}

	allowedKeys := map[string]struct{}{
		"default":     {},
		"description": {},
		"type":        {},
		"required":    {},
		"enum":        {},
		"minimum":     {},
		"maximum":     {},
		"min_length":  {},
		"max_length":  {},
		"pattern":     {},
	}

	for key := range raw {
		if _, ok := allowedKeys[key]; ok {
			continue
		}
		switch key {
		case "minLength":
			return def, entry, fmt.Errorf("invalid inline param field %q; use min_length", key)
		case "maxLength":
			return def, entry, fmt.Errorf("invalid inline param field %q; use max_length", key)
		default:
			return def, entry, fmt.Errorf("invalid inline param field %q", key)
		}
	}

	if value, ok := raw["type"]; ok {
		typeName, ok := value.(string)
		if !ok {
			return def, entry, fmt.Errorf("parameter %q type must be a string", name)
		}
		typeName = strings.TrimSpace(typeName)
		switch typeName {
		case "", core.ParamDefTypeString:
			def.Type = core.ParamDefTypeString
		case core.ParamDefTypeInteger, core.ParamDefTypeNumber, core.ParamDefTypeBoolean:
			def.Type = typeName
		default:
			return def, entry, fmt.Errorf("parameter %q has unsupported type %q", name, typeName)
		}
	}

	if value, ok := raw["description"]; ok {
		description, ok := value.(string)
		if !ok {
			return def, entry, fmt.Errorf("parameter %q description must be a string", name)
		}
		def.Description = description
	}

	if value, ok := raw["required"]; ok {
		required, ok := value.(bool)
		if !ok {
			return def, entry, fmt.Errorf("parameter %q required must be a boolean", name)
		}
		def.Required = required
	}

	if value, ok := raw["minimum"]; ok {
		number, err := toFloat64(value)
		if err != nil {
			return def, entry, fmt.Errorf("parameter %q minimum must be numeric: %w", name, err)
		}
		def.Minimum = &number
	}

	if value, ok := raw["maximum"]; ok {
		number, err := toFloat64(value)
		if err != nil {
			return def, entry, fmt.Errorf("parameter %q maximum must be numeric: %w", name, err)
		}
		def.Maximum = &number
	}

	if value, ok := raw["min_length"]; ok {
		number, err := toInt(value)
		if err != nil {
			return def, entry, fmt.Errorf("parameter %q min_length must be an integer: %w", name, err)
		}
		def.MinLength = &number
	}

	if value, ok := raw["max_length"]; ok {
		number, err := toInt(value)
		if err != nil {
			return def, entry, fmt.Errorf("parameter %q max_length must be an integer: %w", name, err)
		}
		def.MaxLength = &number
	}

	if value, ok := raw["pattern"]; ok {
		pattern, ok := value.(string)
		if !ok {
			return def, entry, fmt.Errorf("parameter %q pattern must be a string", name)
		}
		def.Pattern = &pattern
	}

	if err := validateInlineConstraintCompatibility(def); err != nil {
		return def, entry, err
	}

	if value, ok := raw["enum"]; ok {
		rawItems, ok := value.([]any)
		if !ok {
			return def, entry, fmt.Errorf("parameter %q enum must be a list", name)
		}
		def.Enum = make([]any, 0, len(rawItems))
		for _, item := range rawItems {
			normalized, err := normalizeTypedParamValue(item, def.Type)
			if err != nil {
				return def, entry, fmt.Errorf("parameter %q enum contains an invalid value: %w", name, err)
			}
			def.Enum = append(def.Enum, normalized)
		}
	}

	if value, ok := raw["default"]; ok {
		normalized, err := normalizeTypedParamValue(value, def.Type)
		if err != nil {
			return def, entry, fmt.Errorf("parameter %q default is invalid: %w", name, err)
		}
		def.Default = normalized
		entry.HasValue = true
		entry.Value = stringifyTypedValue(normalized)
	}

	if err := validateInlineDefault(def); err != nil {
		return def, entry, err
	}

	return def, entry, nil
}

func validateInlineConstraintCompatibility(def core.ParamDef) error {
	isString := def.Type == core.ParamDefTypeString
	isNumeric := def.Type == core.ParamDefTypeInteger || def.Type == core.ParamDefTypeNumber

	if !isNumeric && (def.Minimum != nil || def.Maximum != nil) {
		return fmt.Errorf("parameter %q uses minimum/maximum but type is %q", def.Name, def.Type)
	}
	if !isString && (def.MinLength != nil || def.MaxLength != nil || def.Pattern != nil) {
		return fmt.Errorf("parameter %q uses string constraints but type is %q", def.Name, def.Type)
	}
	if def.Minimum != nil && def.Maximum != nil && *def.Minimum > *def.Maximum {
		return fmt.Errorf("parameter %q minimum must be less than or equal to maximum", def.Name)
	}
	if def.MinLength != nil && def.MaxLength != nil && *def.MinLength > *def.MaxLength {
		return fmt.Errorf("parameter %q min_length must be less than or equal to max_length", def.Name)
	}

	return nil
}

func validateInlineDefault(def core.ParamDef) error {
	if def.Default == nil {
		return nil
	}

	if len(def.Enum) > 0 && !containsTypedValue(def.Enum, def.Default) {
		return fmt.Errorf("parameter %q default must be one of enum values", def.Name)
	}

	switch value := def.Default.(type) {
	case string:
		if def.MinLength != nil && len(value) < *def.MinLength {
			return fmt.Errorf("parameter %q default is shorter than min_length", def.Name)
		}
		if def.MaxLength != nil && len(value) > *def.MaxLength {
			return fmt.Errorf("parameter %q default is longer than max_length", def.Name)
		}
	case int64:
		number := float64(value)
		if def.Minimum != nil && number < *def.Minimum {
			return fmt.Errorf("parameter %q default is below minimum", def.Name)
		}
		if def.Maximum != nil && number > *def.Maximum {
			return fmt.Errorf("parameter %q default is above maximum", def.Name)
		}
	case float64:
		if def.Minimum != nil && value < *def.Minimum {
			return fmt.Errorf("parameter %q default is below minimum", def.Name)
		}
		if def.Maximum != nil && value > *def.Maximum {
			return fmt.Errorf("parameter %q default is above maximum", def.Name)
		}
	}

	return nil
}

func compileInlineParamSchema(defs []core.ParamDef) (*jsonschema.Resolved, map[string]*jsonschema.Schema, []string, error) {
	root := &jsonschema.Schema{
		Type:       "object",
		Properties: map[string]*jsonschema.Schema{},
	}
	properties := map[string]*jsonschema.Schema{}
	order := make([]string, 0, len(defs))

	for _, def := range defs {
		if def.Name == "" {
			continue
		}

		property := &jsonschema.Schema{
			Type:        def.Type,
			Description: def.Description,
		}
		if def.Default != nil {
			data, err := json.Marshal(def.Default)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to marshal default for parameter %q: %w", def.Name, err)
			}
			property.Default = data
		}
		if len(def.Enum) > 0 {
			property.Enum = append([]any(nil), def.Enum...)
		}
		if def.Minimum != nil {
			minimum := *def.Minimum
			property.Minimum = &minimum
		}
		if def.Maximum != nil {
			maximum := *def.Maximum
			property.Maximum = &maximum
		}
		if def.MinLength != nil {
			minLength := *def.MinLength
			property.MinLength = &minLength
		}
		if def.MaxLength != nil {
			maxLength := *def.MaxLength
			property.MaxLength = &maxLength
		}
		if def.Pattern != nil {
			property.Pattern = *def.Pattern
		}

		root.Properties[def.Name] = property
		properties[def.Name] = property
		order = append(order, def.Name)
		if def.Required {
			root.Required = append(root.Required, def.Name)
		}
	}

	root.PropertyOrder = append([]string(nil), order...)

	resolved, err := root.Resolve(&jsonschema.ResolveOptions{
		ValidateDefaults: true,
	})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to resolve inline parameter schema: %w", err)
	}

	return resolved, properties, order, nil
}

func deriveExternalSchemaParamDefs(root *jsonschema.Schema, defaults map[string]any) ([]core.ParamDef, bool) {
	if root == nil {
		return nil, false
	}
	if root.Type != "" && root.Type != "object" {
		return nil, false
	}
	if len(root.Properties) == 0 {
		return nil, false
	}
	if len(root.PatternProperties) > 0 || len(root.AllOf) > 0 || len(root.AnyOf) > 0 || len(root.OneOf) > 0 || root.Not != nil || root.If != nil || root.Then != nil || root.Else != nil {
		return nil, false
	}

	required := map[string]bool{}
	for _, name := range root.Required {
		required[name] = true
	}

	order := topLevelSchemaOrder(root)
	paramDefs := make([]core.ParamDef, 0, len(order))
	for _, name := range order {
		property := root.Properties[name]
		if property == nil {
			continue
		}

		paramType, ok := schemaScalarType(property)
		if !ok {
			return nil, false
		}

		def := core.ParamDef{
			Name:        name,
			Type:        paramType,
			Description: property.Description,
			Required:    required[name],
		}
		if value, ok := defaults[name]; ok {
			def.Default = value
		}
		if len(property.Enum) > 0 {
			def.Enum = append([]any(nil), property.Enum...)
		}
		if property.Minimum != nil {
			minimum := *property.Minimum
			def.Minimum = &minimum
		}
		if property.Maximum != nil {
			maximum := *property.Maximum
			def.Maximum = &maximum
		}
		if property.MinLength != nil {
			minLength := *property.MinLength
			def.MinLength = &minLength
		}
		if property.MaxLength != nil {
			maxLength := *property.MaxLength
			def.MaxLength = &maxLength
		}
		if property.Pattern != "" {
			pattern := property.Pattern
			def.Pattern = &pattern
		}

		paramDefs = append(paramDefs, def)
	}

	extraNames := make([]string, 0)
	for name := range defaults {
		if _, ok := root.Properties[name]; ok {
			continue
		}
		extraNames = append(extraNames, name)
	}
	sort.Strings(extraNames)
	for _, name := range extraNames {
		paramDefs = append(paramDefs, core.ParamDef{
			Name:    name,
			Type:    core.ParamDefTypeString,
			Default: stringifyUntypedValue(defaults[name]),
		})
	}

	return paramDefs, true
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

func resolveExternalSchemaEntries(plan *dagParamPlan, rawParams string, paramsList []string) ([]dagParamEntry, error) {
	overridePairs, err := parseOverridePairs(rawParams, paramsList)
	if err != nil {
		return nil, err
	}

	basePairs := make([]paramPair, 0, len(plan.entries))
	for _, entry := range plan.entries {
		if !entry.HasValue {
			continue
		}
		basePairs = append(basePairs, paramPair{Name: entry.Name, Value: entry.Value})
	}

	typedMap, err := schemaPairsToMap(basePairs, plan.schemaProperties, true)
	if err != nil {
		return nil, err
	}
	for _, pair := range overridePairs {
		if pair.Name == "" {
			return nil, fmt.Errorf("positional parameters are not supported when params.schema is used")
		}
		value, err := coerceSchemaPairValue(pair.Name, pair.Value, plan.schemaProperties[pair.Name], true)
		if err != nil {
			return nil, err
		}
		typedMap[pair.Name] = value
	}

	typedMap, err = validateSchemaMap(typedMap, plan.schema, false)
	if err != nil {
		return nil, err
	}

	return entriesFromTypedMap(typedMap, plan.schemaOrder), nil
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

func validateSchemaMap(values map[string]any, schema *jsonschema.Resolved, metadataMode bool) (map[string]any, error) {
	working := maps.Clone(values)
	if err := schema.ApplyDefaults(&working); err != nil {
		return nil, fmt.Errorf("failed to apply schema defaults: %w", err)
	}

	validateSchema := schema
	if metadataMode && len(schema.Schema().Required) > 0 {
		if !hasAllRequiredKeys(working, schema.Schema().Required) {
			clone := schema.Schema().CloneSchemas()
			clone.Required = nil
			resolved, err := clone.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
			if err != nil {
				return nil, fmt.Errorf("failed to prepare parameter schema: %w", err)
			}
			validateSchema = resolved
		}
	}

	if err := validateSchema.Validate(working); err != nil {
		return nil, fmt.Errorf("parameter validation failed: %w", err)
	}

	return working, nil
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

func schemaPairsToMap(pairs []paramPair, properties map[string]*jsonschema.Schema, allowSchemaFallbackJSON bool) (map[string]any, error) {
	result := make(map[string]any, len(pairs))
	for _, pair := range pairs {
		if pair.Name == "" {
			continue
		}
		value, err := coerceSchemaPairValue(pair.Name, pair.Value, properties[pair.Name], allowSchemaFallbackJSON)
		if err != nil {
			return nil, err
		}
		result[pair.Name] = value
	}
	return result, nil
}

func coerceSchemaPairValue(name, raw string, schema *jsonschema.Schema, allowSchemaFallbackJSON bool) (any, error) {
	paramType, ok := schemaScalarType(schema)
	if ok {
		value, err := coerceStringToType(raw, paramType)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		return value, nil
	}
	if allowSchemaFallbackJSON {
		return parseJSONLikeValue(raw), nil
	}
	return raw, nil
}

func schemaScalarType(schema *jsonschema.Schema) (string, bool) {
	if schema == nil {
		return "", false
	}
	switch schema.Type {
	case core.ParamDefTypeString, core.ParamDefTypeInteger, core.ParamDefTypeNumber, core.ParamDefTypeBoolean:
		return schema.Type, true
	}
	if len(schema.Types) == 1 {
		switch schema.Types[0] {
		case core.ParamDefTypeString, core.ParamDefTypeInteger, core.ParamDefTypeNumber, core.ParamDefTypeBoolean:
			return schema.Types[0], true
		}
	}
	if len(schema.Enum) == 0 {
		return "", false
	}

	firstType, ok := inferScalarType(schema.Enum[0])
	if !ok {
		return "", false
	}
	for _, item := range schema.Enum[1:] {
		itemType, ok := inferScalarType(item)
		if !ok || itemType != firstType {
			return "", false
		}
	}
	return firstType, true
}

func inferScalarType(value any) (string, bool) {
	switch value.(type) {
	case string:
		return core.ParamDefTypeString, true
	case bool:
		return core.ParamDefTypeBoolean, true
	case float32, float64:
		return core.ParamDefTypeNumber, true
	case int, int8, int16, int32, int64:
		return core.ParamDefTypeInteger, true
	case uint, uint8, uint16, uint32, uint64:
		return core.ParamDefTypeInteger, true
	default:
		return "", false
	}
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

func mergeTypedMapIntoEntries(entries []dagParamEntry, typedValues map[string]any, schemaOrder []string) []dagParamEntry {
	result := cloneParamEntries(entries)
	remaining := maps.Clone(typedValues)
	seen := map[string]struct{}{}

	for i := range result {
		if result[i].Name == "" {
			continue
		}
		value, ok := remaining[result[i].Name]
		if !ok {
			result[i].HasValue = false
			result[i].Value = ""
			continue
		}
		result[i].Value = stringifyTypedValue(value)
		result[i].HasValue = true
		delete(remaining, result[i].Name)
		seen[result[i].Name] = struct{}{}
	}

	for _, name := range schemaOrder {
		if _, ok := seen[name]; ok {
			continue
		}
		value, ok := remaining[name]
		if !ok {
			continue
		}
		result = append(result, dagParamEntry{
			Name:     name,
			Value:    stringifyTypedValue(value),
			HasValue: true,
		})
		delete(remaining, name)
	}

	if len(remaining) == 0 {
		return result
	}

	extraNames := make([]string, 0, len(remaining))
	for name := range remaining {
		extraNames = append(extraNames, name)
	}
	sort.Strings(extraNames)
	for _, name := range extraNames {
		result = append(result, dagParamEntry{
			Name:     name,
			Value:    stringifyTypedValue(remaining[name]),
			HasValue: true,
		})
	}

	return result
}

func entriesFromTypedMap(values map[string]any, schemaOrder []string) []dagParamEntry {
	entries := make([]dagParamEntry, 0, len(values))
	seen := map[string]struct{}{}

	for _, name := range schemaOrder {
		value, ok := values[name]
		if !ok {
			continue
		}
		entries = append(entries, dagParamEntry{
			Name:     name,
			Value:    stringifyTypedValue(value),
			HasValue: true,
		})
		seen[name] = struct{}{}
	}

	extraNames := make([]string, 0)
	for name := range values {
		if _, ok := seen[name]; ok {
			continue
		}
		extraNames = append(extraNames, name)
	}
	sort.Strings(extraNames)
	for _, name := range extraNames {
		entries = append(entries, dagParamEntry{
			Name:     name,
			Value:    stringifyTypedValue(values[name]),
			HasValue: true,
		})
	}

	return entries
}

func topLevelSchemaOrder(root *jsonschema.Schema) []string {
	if root == nil || len(root.Properties) == 0 {
		return nil
	}

	if len(root.PropertyOrder) > 0 {
		order := make([]string, 0, len(root.Properties))
		seen := map[string]struct{}{}
		for _, name := range root.PropertyOrder {
			if _, ok := root.Properties[name]; !ok {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			order = append(order, name)
			seen[name] = struct{}{}
		}
		if len(order) == len(root.Properties) {
			return order
		}
		extraNames := make([]string, 0)
		for name := range root.Properties {
			if _, ok := seen[name]; ok {
				continue
			}
			extraNames = append(extraNames, name)
		}
		sort.Strings(extraNames)
		return append(order, extraNames...)
	}

	order := make([]string, 0, len(root.Properties))
	for name := range root.Properties {
		order = append(order, name)
	}
	sort.Strings(order)
	return order
}

func extractSchemaValues(input any) any {
	paramsMap, ok := input.(map[string]any)
	if !ok {
		return nil
	}
	return paramsMap["values"]
}

func hasAllRequiredKeys(values map[string]any, required []string) bool {
	for _, name := range required {
		if _, ok := values[name]; !ok {
			return false
		}
	}
	return true
}

func stringifyTypedValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(data)
	}
}

func stringifyUntypedValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func containsTypedValue(values []any, target any) bool {
	for _, item := range values {
		if reflect.DeepEqual(item, target) {
			return true
		}
	}
	return false
}

func parseJSONLikeValue(value string) any {
	var decoded any
	if err := json.Unmarshal([]byte(value), &decoded); err == nil {
		return decoded
	}
	return value
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
		return int64(v), nil
	case uint8:
		return int64(v), nil
	case uint16:
		return int64(v), nil
	case uint32:
		return int64(v), nil
	case uint64:
		return int64(v), nil
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
