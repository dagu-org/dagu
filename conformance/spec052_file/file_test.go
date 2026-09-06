// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec052_file_test covers local file action wiring through the CLI.
package spec052_file_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

func TestFileRoundTrip(t *testing.T) {
	t.Parallel()
	dagu := harness.NewRunner(t)
	dagu.Run("start", "roundtrip.yaml").ExpectExitCode(0)
	dagu.ExpectFileContent("files/source.txt", "moved-content")
	dagu.ExpectFileContent("read.txt", "moved-content")
	dagu.ExpectNoFile("files/copy.txt")
	dagu.ExpectNoFile("files/destination.txt")

	info, err := os.Stat(dagu.ProjectPath("files/nested"))
	require.NoError(t, err)
	require.True(t, info.IsDir())

	data, err := os.ReadFile(dagu.ProjectPath("stat.json"))
	require.NoError(t, err)
	var stat struct {
		Operation string `json:"operation"`
		Exists    bool   `json:"exists"`
		Type      string `json:"type"`
	}
	require.NoError(t, json.Unmarshal(data, &stat))
	require.Equal(t, "stat", stat.Operation)
	require.True(t, stat.Exists)
	require.Equal(t, "file", stat.Type)

	data, err = os.ReadFile(dagu.ProjectPath("list.json"))
	require.NoError(t, err)
	var list struct {
		Operation string `json:"operation"`
		Entries   []struct {
			Path string `json:"path"`
		} `json:"entries"`
	}
	require.NoError(t, json.Unmarshal(data, &list))
	require.Equal(t, "list", list.Operation)
	require.Len(t, list.Entries, 2)
	require.Equal(t, "destination.txt", list.Entries[0].Path)
	require.Equal(t, "source.txt", list.Entries[1].Path)
}

func TestFileValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ file, message string }{
		{"write_missing_content.yaml", "content is required for write"},
		{"read_missing_path.yaml", "path is required for read"},
		{"stat_missing_path.yaml", "path is required for stat"},
		{"delete_missing_path.yaml", "path is required for delete"},
		{"mkdir_missing_path.yaml", "path is required for mkdir"},
		{"list_missing_path.yaml", "path is required for list"},
		{"copy_missing_source.yaml", "source is required for copy"},
		{"copy_missing_destination.yaml", "destination is required for copy"},
		{"move_missing_source.yaml", "source is required for move"},
		{"move_missing_destination.yaml", "destination is required for move"},
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
