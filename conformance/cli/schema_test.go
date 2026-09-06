// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cli_test

import (
	"encoding/json"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestSchemaShowsDAGRootFields(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("schema", "dag")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "steps")
}

// The nested result must describe the selected steps field.
func TestSchemaDrillsIntoNestedPath(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("schema", "dag", "steps")
	result.ExpectExitCode(0)
	var schema struct {
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(result.Stdout()), &schema))
	require.Contains(t, schema.Description, "List of steps that define the DAG")
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
