// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec022_mcp_change_tool_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/mcptest"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestChangeToolIdentityAndInputSchema(t *testing.T) {
	server := mcptest.NewServer(t)
	session := server.Connect(t, "")
	ctx := mcptest.Context(t)

	result, err := session.ListTools(ctx, nil)
	require.NoError(t, err)

	var tool *mcpsdk.Tool
	for _, candidate := range result.Tools {
		if candidate.Name == "dagu_change" {
			tool = candidate
			break
		}
	}
	require.NotNil(t, tool)
	require.NotNil(t, tool.Annotations)
	require.NotNil(t, tool.Annotations.DestructiveHint)
	require.True(t, *tool.Annotations.DestructiveHint)
	require.NotNil(t, tool.Annotations.OpenWorldHint)
	require.False(t, *tool.Annotations.OpenWorldHint)

	schema := toolInputSchema(t, tool)
	require.Equal(t, "object", schema["type"])
	require.Equal(t, false, schema["additionalProperties"])

	properties, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	for _, field := range []string{"mode", "type", "name", "spec", "newName", "profile", "workspace", "path", "content", "newPath"} {
		property, ok := properties[field].(map[string]any)
		require.True(t, ok)
		require.Equal(t, "string", property["type"])
	}

	type branchContract struct {
		types    []any
		required []any
	}
	expectedBranches := map[string]branchContract{
		"upsert_dag": {
			types:    []any{"upsert_dag"},
			required: []any{"name", "spec"},
		},
		"rename_dag": {
			types:    []any{"rename_dag"},
			required: []any{"type", "name", "newName"},
		},
		"delete_dag": {
			types:    []any{"delete_dag"},
			required: []any{"type", "name"},
		},
		"set_dag_profile": {
			types:    []any{"set_dag_profile"},
			required: []any{"type", "name", "profile"},
		},
		"clear_dag_profile": {
			types:    []any{"clear_dag_profile"},
			required: []any{"type", "name"},
		},
		"upsert_wiki_page": {
			types:    []any{"upsert_wiki_page"},
			required: []any{"type", "workspace", "path", "content"},
		},
		"rename_wiki_page": {
			types:    []any{"rename_wiki_page"},
			required: []any{"type", "workspace", "path", "newPath"},
		},
		"delete_wiki_page": {
			types:    []any{"delete_wiki_page"},
			required: []any{"type", "workspace", "path"},
		},
	}
	// The schema is flat (no root-level combinators) so clients that cannot handle a root oneOf
	// still send every argument; each per-type contract is documented in the descriptions and
	// enforced server-side (see change_errors_test.go).
	for _, combinator := range []string{"oneOf", "anyOf", "allOf"} {
		require.NotContains(t, schema, combinator)
	}
	typeSchema, ok := properties["type"].(map[string]any)
	require.True(t, ok)
	typeDescription, ok := typeSchema["description"].(string)
	require.True(t, ok)
	for changeType, contract := range expectedBranches {
		require.Contains(t, requireArray(t, typeSchema, "enum"), changeType)
		fields := make([]string, 0, len(contract.required))
		for _, field := range contract.required {
			name, ok := field.(string)
			require.True(t, ok)
			require.Contains(t, properties, name)
			fields = append(fields, name)
		}
		require.Contains(t, typeDescription, changeType+": "+strings.Join(fields, ", ")+".")
	}
}

func toolInputSchema(t *testing.T, tool *mcpsdk.Tool) map[string]any {
	t.Helper()

	data, err := json.Marshal(tool.InputSchema)
	require.NoError(t, err)
	var schema map[string]any
	require.NoError(t, json.Unmarshal(data, &schema))
	return schema
}
