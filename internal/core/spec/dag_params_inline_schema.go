// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec

import (
	"encoding/json"
	"fmt"
	"maps"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/google/jsonschema-go/jsonschema"
)

// isInlineJSONSchema returns true when params is a map[string]any with a
// "properties" key whose value is also a map[string]any.  It returns false for
// the external-schema format (which uses a "schema" key with a file path).
func isInlineJSONSchema(input any) bool {
	m, ok := input.(map[string]any)
	if !ok {
		return false
	}
	// External schema format takes precedence.
	if _, ok := extractParamsSchemaDeclaration(input); ok {
		return false
	}
	props, ok := m["properties"]
	if !ok {
		return false
	}
	_, ok = props.(map[string]any)
	return ok
}

// buildInlineSchemaParamPlan compiles the inline JSON Schema stored directly in
// the params field and returns a dagParamPlan backed by the compiled schema.
func buildInlineSchemaParamPlan(input any) (*dagParamPlan, error) {
	m, ok := input.(map[string]any)
	if !ok {
		return nil, core.NewValidationError("params", input, fmt.Errorf("%w: expected an object for inline JSON Schema", ErrInvalidParamValue))
	}

	// Strip fields that are not understood by the JSON Schema library at the
	// top level to avoid resolution errors.  Nested readonly fields are passed
	// through; the library absorbs unknown keywords inside property definitions.
	stripped := make(map[string]any, len(m))
	maps.Copy(stripped, m)
	delete(stripped, "readOnly")
	delete(stripped, "readonly")

	raw, err := json.Marshal(stripped)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal inline param schema: %w", err)
	}

	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse inline param schema: %w", err)
	}

	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve inline param schema: %w", err)
	}

	root := resolved.Schema()
	schemaOrder := topLevelSchemaOrder(root)
	schemaProperties := map[string]*jsonschema.Schema{}
	if root != nil {
		maps.Copy(schemaProperties, root.Properties)
	}

	// Start with an empty typed map and apply schema defaults.
	typedDefaults := map[string]any{}
	typedDefaults, err = validateSchemaMap(typedDefaults, resolved, true)
	if err != nil {
		return nil, err
	}

	plan := &dagParamPlan{
		kind:             dagParamKindInlineSchema,
		schema:           resolved,
		schemaOrder:      schemaOrder,
		schemaProperties: schemaProperties,
		entries:          entriesFromTypedMap(typedDefaults, schemaOrder),
	}

	if paramDefs, ok := deriveExternalSchemaParamDefs(root, typedDefaults); ok {
		plan.paramDefs = paramDefs
	}

	return plan, nil
}
