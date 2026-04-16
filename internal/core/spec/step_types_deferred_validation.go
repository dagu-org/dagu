// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"regexp"

	"github.com/dagucloud/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

var customStepRuntimeExpressionRegexp = regexp.MustCompile("`[^`]+`|\\$\\{[^}]+\\}|\\$[A-Za-z_][A-Za-z0-9_]*")

var customStepWholeRuntimeExpressionRegexp = regexp.MustCompile("^\\s*(?:`[^`]+`|\\$\\{[^}]+\\}|\\$[A-Za-z_][A-Za-z0-9_]*)\\s*$")

func customStepInputSchemaAllowingRuntimeExpressions(schema *jsonschema.Resolved, input map[string]any) (*jsonschema.Resolved, bool) {
	if schema == nil || schema.Schema() == nil {
		return nil, false
	}
	relaxed := schema.Schema().CloneSchemas()
	if !relaxCustomStepRuntimeExpressionSchema(relaxed, input) {
		return nil, false
	}
	resolved, err := relaxed.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, false
	}
	return resolved, true
}

func relaxCustomStepRuntimeExpressionSchema(schema *jsonschema.Schema, value any) bool {
	if schema == nil {
		return false
	}

	switch typed := value.(type) {
	case string:
		if !canDeferCustomStepInputValue(schema, typed) {
			return false
		}
		*schema = jsonschema.Schema{}
		return true
	case map[string]any:
		return relaxCustomStepRuntimeExpressionObjectSchema(schema, typed)
	case []any:
		return relaxCustomStepRuntimeExpressionArraySchema(schema, typed)
	default:
		return false
	}
}

func relaxCustomStepRuntimeExpressionObjectSchema(schema *jsonschema.Schema, value map[string]any) bool {
	relaxed := false
	for key, item := range value {
		propertySchema, ok := schema.Properties[key]
		if !ok || propertySchema == nil {
			continue
		}
		if relaxCustomStepRuntimeExpressionSchema(propertySchema, item) {
			relaxed = true
		}
	}
	return relaxed
}

func relaxCustomStepRuntimeExpressionArraySchema(schema *jsonschema.Schema, value []any) bool {
	relaxed := false
	for idx, item := range value {
		var itemSchema *jsonschema.Schema
		switch {
		case idx < len(schema.PrefixItems):
			itemSchema = schema.PrefixItems[idx]
		case idx < len(schema.ItemsArray):
			itemSchema = schema.ItemsArray[idx]
		case schema.Items != nil:
			itemSchema = schema.Items
		case schema.AdditionalItems != nil:
			itemSchema = schema.AdditionalItems
		}
		if itemSchema == nil {
			continue
		}
		if relaxCustomStepRuntimeExpressionSchema(itemSchema, item) {
			relaxed = true
		}
	}
	return relaxed
}

func canDeferCustomStepInputValue(schema *jsonschema.Schema, value string) bool {
	if !customStepRuntimeExpressionRegexp.MatchString(value) {
		return false
	}

	schemaType, ok := schemaScalarType(schema)
	if !ok {
		return false
	}

	if schemaType == core.ParamDefTypeString {
		return true
	}

	return customStepWholeRuntimeExpressionRegexp.MatchString(value)
}
