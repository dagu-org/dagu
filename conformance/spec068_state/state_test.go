// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec068_state tests persistent state through the Dagu binary.
package spec068_state_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestStateHappyPath(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "happy_path.yaml")
	result.ExpectExitCode(0)

	var setVal struct {
		Operation string `json:"operation"`
		Scope     string `json:"scope"`
		Key       string `json:"key"`
		Version   int64  `json:"version"`
		Created   bool   `json:"created"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &setVal))
	require.Equal(t, "set", setVal.Operation)
	require.Equal(t, "dag", setVal.Scope)
	require.Equal(t, "mykey", setVal.Key)
	require.EqualValues(t, 1, setVal.Version)
	require.True(t, setVal.Created)

	var getVal struct {
		Found bool           `json:"found"`
		Value map[string]any `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &getVal))
	require.True(t, getVal.Found)
	require.EqualValues(t, map[string]any{"hello": "world", "n": float64(1)}, getVal.Value)

	var getMissingDefault struct {
		Found bool           `json:"found"`
		Value map[string]any `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &getMissingDefault))
	require.False(t, getMissingDefault.Found, "state.get on a missing key without required: true reports found: false, not an error")
	require.EqualValues(t, map[string]any{"fallback": true}, getMissingDefault.Value)

	var diffCreate struct {
		Changed       bool `json:"changed"`
		FoundPrevious bool `json:"foundPrevious"`
		Version       int64
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &diffCreate))
	require.True(t, diffCreate.Changed)
	require.False(t, diffCreate.FoundPrevious, "the first state.diff against a new key has no previous value")

	var diffSame struct {
		Changed         bool            `json:"changed"`
		FoundPrevious   bool            `json:"foundPrevious"`
		PreviousVersion int64           `json:"previousVersion"`
		Previous        json.RawMessage `json:"previous"`
		Version         int64           `json:"version"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 4)), &diffSame))
	require.False(t, diffSame.Changed, "state.diff against an identical value reports changed: false")
	require.True(t, diffSame.FoundPrevious)
	require.JSONEq(t, `{"v":1}`, string(diffSame.Previous))
	require.Equal(t, diffSame.PreviousVersion, diffSame.Version, "an unchanged diff does not bump the version")

	var diffNoUpdate struct {
		Changed bool  `json:"changed"`
		Version int64 `json:"version"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 5)), &diffNoUpdate))
	require.True(t, diffNoUpdate.Changed)
	require.EqualValues(t, 1, diffNoUpdate.Version, "update: false reports the change but does not write it, so the version stays at its previous value")

	var diffUpdate struct {
		Changed bool  `json:"changed"`
		Version int64 `json:"version"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 6)), &diffUpdate))
	require.True(t, diffUpdate.Changed)
	require.EqualValues(t, 2, diffUpdate.Version, "without update: false, a changed diff writes the new value and bumps the version")

	var listAll struct {
		Entries []struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 7)), &listAll))
	require.Len(t, listAll.Entries, 2, "state.list without a prefix returns every entry in scope, and it defaults to omitting values")
	for _, entry := range listAll.Entries {
		require.Nil(t, entry.Value)
	}

	var listPrefixValues struct {
		Entries []struct {
			Key   string         `json:"key"`
			Value map[string]any `json:"value"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 8)), &listPrefixValues))
	require.Len(t, listPrefixValues.Entries, 1)
	require.Equal(t, "diff_key", listPrefixValues.Entries[0].Key)
	require.EqualValues(t, map[string]any{"v": float64(2)}, listPrefixValues.Entries[0].Value)

	var deleted struct {
		Deleted bool `json:"deleted"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 9)), &deleted))
	require.True(t, deleted.Deleted)

	var afterDelete struct {
		Found bool `json:"found"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 10)), &afterDelete))
	require.False(t, afterDelete.Found)
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 11)), &listAll))
	require.Len(t, listAll.Entries, 1)
	require.Equal(t, "diff_key", listAll.Entries[0].Key)

}

// TestStateScopeIsolation proves the difference between scope: dag (which
// namespaces state under the current DAG's own name -- isolated per
// sub-dag) and scope: root_dag (which namespaces state under the root
// dag-run's name, shared by every DAG in that run's own nested-call tree).
func TestStateScopeIsolation(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "scope_isolation_parent.yaml")
	result.ExpectExitCode(0)

	var dagScope struct {
		Found bool   `json:"found"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 1)), &dagScope))
	require.False(t, dagScope.Found,
		"scope: dag namespaces by the current DAG's own name, so the parent cannot see a key the child wrote under its own dag scope")
	require.Equal(t, "MISSING", dagScope.Value)

	var rootScope struct {
		Found bool   `json:"found"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 2)), &rootScope))
	require.True(t, rootScope.Found,
		"scope: root_dag namespaces by the root dag-run's name, so the parent sees what the child wrote under root_dag scope")
	require.Equal(t, "from-child-root-scope", rootScope.Value)

	for i, want := range []string{"first", "second"} {
		var custom struct {
			Found bool   `json:"found"`
			Value string `json:"value"`
		}
		require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 5+i)), &custom))
		require.True(t, custom.Found)
		require.Equal(t, want, custom.Value)
	}

}

func TestStateErrorScenarios(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "error_scenarios.yaml")
	result.ExpectNonZeroExitCode()

	// Both writes must fail independently after the initial value is stored.
	for _, step := range []string{"set_create_only_again", "set_bad_version"} {
		require.Regexp(t, step+`[^\n]*\[failed\]`, result.Stdout())
	}
	require.Equal(t, 2, strings.Count(result.Stdout(), "dag state: conflict"))
	require.Contains(t, result.Stdout(), "dag state: not found",
		"state.get required: true against a missing key fails instead of reporting found: false")
	require.Contains(t, result.Stdout(), "namespace is required for custom")
	require.Contains(t, result.Stdout(), `invalid key "../escape"`)
}

func TestStateDownstreamReference(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("start", "downstream_reference.yaml")
	result.ExpectExitCode(0)

	var output struct {
		Found bool `json:"found"`
		Value int  `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 3)), &output))
	require.True(t, output.Found)
	require.Equal(t, 42, output.Value)
}

func TestStatePersistence(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	fixedHome := t.TempDir()
	env := []string{
		"HOME=" + fixedHome,
		"DAGU_HOME=" + filepath.Join(fixedHome, "dagu"),
	}

	writeResult := dagu.RunWithEnv(env, "start", "global_persistence_writer.yaml")
	writeResult.ExpectExitCode(0)

	// A separate dagu start invocation, pinned to the same HOME/DAGU_HOME,
	// stands in for a genuinely separate process reading state a prior run
	// wrote -- proving state.set's writes are durable, not scoped to the
	// dag-run that created them.
	readResult := dagu.RunWithEnv(env, "start", "global_persistence_reader.yaml")
	readResult.ExpectExitCode(0)

	var getVal struct {
		Found bool   `json:"found"`
		Value string `json:"value"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, readResult.Stdout(), 0)), &getVal))
	require.True(t, getVal.Found)
	require.Equal(t, "persisted-across-runs", getVal.Value)
}

// TestStateValidation covers dagu validate for the state executor. Like
// the harness executor (spec 067) and unlike the remote actions in specs
// 060-066, state is a built-in executor: dagu validate resolves and checks
// some of its configuration without running anything. It checks that
// with.key is present (for get/set/delete/diff) and with.value is present
// (for set/diff), but not scope-specific rules like a custom scope needing
// with.namespace, or with.key's character restrictions -- those surface
// only at run time (see TestStateErrorScenarios).
func TestStateValidation(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		file       string
		wantStderr string
	}{
		{"invalid_missing_key.yaml", "key is required"},
		{"invalid_missing_value.yaml", "value is required for set"},
		{"invalid_diff_value.yaml", "value is required for diff"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.file)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.wantStderr)
		})
	}

	t.Run("a well-formed state step passes validate", func(t *testing.T) {
		t.Parallel()

		dagu := harness.NewRunner(t)
		result := dagu.Run("validate", "happy_path.yaml")
		result.ExpectExitCode(0)
	})
}
