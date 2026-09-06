// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// Nested navigation returns the selected property from the root schema.
func TestSchemaDrillsIntoNestedPath(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	root := dagu.Run("schema", "dag")
	root.ExpectExitCode(0)
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal([]byte(root.Stdout()), &schema))
	require.Contains(t, schema.Properties, "steps")

	nested := dagu.Run("schema", "dag", "steps")
	nested.ExpectExitCode(0)
	require.JSONEq(t, string(schema.Properties["steps"]), nested.Stdout())
}

// TestSchemaShowsConfigRootFields asserts on "coordinator", a property that
// exists only on the config root schema and not on the DAG schema, so the
// test can't pass against the wrong schema.
func TestSchemaShowsConfigRootFields(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("schema", "config")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), `"coordinator"`)
}

func TestSchemaRejectsUnknownName(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("schema", "bogus")
	result.ExpectNonZeroExitCode()
}
