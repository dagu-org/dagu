// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec051_artifact_test covers artifact action wiring through the CLI.
package spec051_artifact_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestArtifactRoundTrip(t *testing.T) {
	t.Parallel()
	dagu := harness.NewRunner(t)
	dagu.Run("start", "roundtrip.yaml").ExpectExitCode(0)
	dagu.ExpectFileContent("read.txt", "hello")

	for _, tc := range []struct {
		file string
		want map[string]any
	}{
		{"write.json", map[string]any{"operation": "write", "path": "hello.txt", "bytes": float64(5), "created": true}},
		{"read.json", map[string]any{"operation": "read", "path": "hello.txt", "exists": true, "type": "file", "size": float64(5), "bytes": float64(5), "content": "hello"}},
		{"list.json", map[string]any{"operation": "list", "path": ".", "files": float64(1)}},
	} {
		t.Run(tc.file, func(t *testing.T) {
			data, err := os.ReadFile(dagu.ProjectPath(tc.file))
			require.NoError(t, err)
			var out map[string]any
			require.NoError(t, json.Unmarshal(data, &out))
			for key, value := range tc.want {
				require.Equal(t, value, out[key], key)
			}
			if tc.file == "list.json" {
				entries, ok := out["entries"].([]any)
				require.True(t, ok)
				require.Len(t, entries, 1)
				entry, ok := entries[0].(map[string]any)
				require.True(t, ok)
				require.Equal(t, "hello.txt", entry["path"])
				require.Equal(t, "file", entry["type"])
			}
		})
	}
}

func TestArtifactPaths(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ file, message string }{
		{"path_escape_dotdot.yaml", "artifact path must not contain .."},
		{"path_escape_absolute.yaml", "artifact path must be relative"},
		{"path_escape_tilde.yaml", "artifact path must be relative"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			dagu := harness.NewRunner(t)
			result := dagu.Run("start", tc.file)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.message)
		})
	}
}

func TestArtifactValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ file, message string }{
		{"write_missing_content.yaml", "content is required for write"},
		{"read_missing_path.yaml", "path is required for read"},
		{"overwrite_atomic_conflict.yaml", "overwrite requires atomic writes"},
		{"artifacts_disabled_conflict.yaml", "artifact actions require artifacts.enabled to be true"},
	} {
		t.Run(tc.file, func(t *testing.T) {
			t.Parallel()
			dagu := harness.NewRunner(t)
			result := dagu.Run("validate", tc.file)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.message)
		})
	}
}
