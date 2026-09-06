// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// This file adds live coverage for duckdb@v1: real network resolution of
// the action (cloning https://github.com/dagucloud/duckdb and provisioning
// its pinned duckdb/duckdb@v1.5.2 tool via the project's aqua-based tool
// manager), running the real duckdb CLI it installs, and observing its
// actual output and error behavior. TestLocalAction and TestActionBoundary
// in duckdb_test.go continue to exercise the generic action-bundle
// mechanism (input/output schema validation, source: references) against a
// local bundle with no network access; this file complements that with the
// specific dagucloud/duckdb action's own observed behavior, the same way
// specs 060-066 cover their own remote actions. Unlike those actions'
// Node.js wrappers, duckdb@v1's own workflow.yaml runs a plain shell script
// that execs duckdb directly and captures its stdout via Dagu's own
// declared stdout: {outputs: {field: result}} mechanism (Spec 012) -- there
// is no ok/exitCode/error JSON wrapper at all, just {"result": "<raw duckdb
// stdout>"}.
package spec048_duckdb_test

import (
	"encoding/json"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestDuckDBLiveBasicQuery(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "duckdb_live.yaml")
	result.ExpectExitCode(0)

	var output struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &output))

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(output.Result), &rows),
		"result is duckdb's own -json stdout (the default format), not a Dagu-specific envelope")
	require.Equal(t, []map[string]any{{"answer": float64(42), "engine": "duckdb"}}, rows)
}

func TestDuckDBLivePersistentDatabase(t *testing.T) {
	t.Setenv("DAGU_CONFORMANCE_COMMAND_TIMEOUT", "2m")

	dagu := harness.NewRunner(t)
	dagu.Mkdir("data")
	env := []string{
		"HOST_DB_PATH=" + dagu.ProjectPath("data/test.duckdb"),
		"HOST_DATA_DIR=" + dagu.ProjectPath("data"),
	}
	result := dagu.RunWithEnv(env, "start", "live_persistent_database.yaml")
	result.ExpectExitCode(0)

	var createTable struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &createTable))
	require.Empty(t, createTable.Result, "a DDL/DML statement with no SELECT produces no duckdb stdout")

	var readCSV struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &readCSV))
	require.Equal(t, "id,name\n1,a\n2,b", readCSV.Result,
		"with.readonly: true should see the previous step's own writes to the same with.database file")

	var readTable struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &readTable))
	require.Equal(t, "+---+\n| n |\n+---+\n| 2 |\n+---+", readTable.Result)

	var workdirQuery struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &workdirQuery))
	require.JSONEq(t, `[{"ok":1}]`, workdirQuery.Result, "with.workdir should not interfere with an unrelated query")
}

func TestDuckDBLiveSQLError(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "live_sql_error.yaml")
	result.ExpectNonZeroExitCode()

	// duckdb (run with -bail) exits non-zero and writes its own error to
	// stderr, not stdout; the action's declared result output still
	// captures whatever duckdb wrote to stdout, which is nothing.
	var output struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &output))
	require.Empty(t, output.Result)
	result.ExpectStderrContains("this_table_does_not_exist does not exist")
}

// Unlike node-script@v1's with.script, duckdb@v1's with.query is not
// resolved by Dagu's own value resolution before it reaches the action: a
// literal "$VAR"-shaped token in with.query survives unresolved into the
// query duckdb actually runs.
func TestDuckDBLiveQueryFieldNotResolved(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "live_query_not_resolved.yaml")
	result.ExpectExitCode(0)

	var output struct {
		Result string `json:"result"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &output))
	require.Contains(t, output.Result, "$MY_VAR")
	require.NotContains(t, output.Result, "from-env")
}

// Like node-script@v1, python-script@v1, dbt@v1, ffmpeg@v1, github-cli@v1,
// rclone@v1, and the harness executor's own output: NAME mechanism (specs
// 060-067), a later step reads a result field as
// ${<step id>.outputs.<name>} -- a bare-step-id reference -- and not via
// ${steps.<step id>.outputs.<name>}, confirming duckdb@v1 follows the same
// remote action:-type convention.
func TestDuckDBLiveDownstreamReference(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "live_downstream_reference.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("bad substitution")

	require.Equal(t, "bare result is [{answer:42}]\n", stepStdout(t, result.Stdout(), 1))
}
