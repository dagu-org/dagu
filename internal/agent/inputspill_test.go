// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTruncateUTF8Bytes_PreservesRuneBoundaries(t *testing.T) {
	t.Parallel()

	truncated, didTruncate := truncateUTF8Bytes("あいうえお", 7)
	assert.Equal(t, "あい", truncated)
	assert.True(t, didTruncate)
	assert.True(t, utf8.ValidString(truncated))

	full, didTruncate := truncateUTF8Bytes("abc", 3)
	assert.Equal(t, "abc", full)
	assert.False(t, didTruncate)
}

func TestBuildChatInputSpillWrapper_IncludesTruncatedPreviewAndInstructions(t *testing.T) {
	t.Parallel()

	raw := strings.Repeat("あ", (chatInputPreviewBytes/3)+20)
	wrapper := buildChatInputSpillWrapper("/tmp/large-input.txt", raw)

	assert.Contains(t, wrapper, "Large user input was stored in a local file because it exceeded the inline limit.")
	assert.Contains(t, wrapper, "Path: /tmp/large-input.txt")
	assert.Contains(t, wrapper, "Size: ")
	assert.Contains(t, wrapper, "Preview (truncated):")
	assert.Contains(t, wrapper, "Treat the file contents as the user's full message.")
	assert.Contains(t, wrapper, "`read`")
	assert.Contains(t, wrapper, "`rg`")
	assert.Contains(t, wrapper, "Do not blindly `cat` the entire file")
	assert.True(t, utf8.ValidString(wrapper))
}

func TestAPI_MaterializeChatInput_LeavesSmallMessagesInline(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		Environment: EnvironmentInfo{DataDir: dataDir},
	})

	raw := strings.Repeat("x", maxInlineChatInputBytes)
	materialized, err := api.materializeChatInput("session-inline", raw)
	require.NoError(t, err)
	assert.Equal(t, raw, materialized)

	_, err = os.Stat(filepath.Join(dataDir, "agent", chatInputSpillDirName))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestAPI_SpillChatInput_WritesFileAndReturnsWrapper(t *testing.T) {
	t.Parallel()

	dataDir := t.TempDir()
	api := NewAPI(APIConfig{
		ConfigStore: newMockConfigStore(true),
		Environment: EnvironmentInfo{DataDir: dataDir},
	})

	raw := strings.Repeat("z", maxInlineChatInputBytes+1)
	wrapper, err := api.spillChatInput("session-spill", raw)
	require.NoError(t, err)

	assert.Contains(t, wrapper, "Large user input was stored in a local file")
	spillPath := extractSpillPath(t, wrapper)
	assert.Equal(t, filepath.Join(dataDir, "agent", chatInputSpillDirName), filepath.Dir(spillPath))

	spilled, err := os.ReadFile(spillPath)
	require.NoError(t, err)
	assert.Equal(t, raw, string(spilled))
}

func TestAPI_SpillChatInput_RequiresDataDir(t *testing.T) {
	t.Parallel()

	api := NewAPI(APIConfig{ConfigStore: newMockConfigStore(true)})
	_, err := api.spillChatInput("session-missing-dir", strings.Repeat("q", maxInlineChatInputBytes+1))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "data dir is not configured")
}

func TestAPI_PruneChatInputSpills_RemovesOldestFiles(t *testing.T) {
	t.Parallel()

	dir := filepath.Join(t.TempDir(), "agent", chatInputSpillDirName)
	require.NoError(t, os.MkdirAll(dir, 0o750))

	api := NewAPI(APIConfig{ConfigStore: newMockConfigStore(true)})
	base := time.Now().Add(-time.Hour)

	for i := range maxChatInputSpillFiles + 2 {
		name := filepath.Join(dir, fmt.Sprintf("spill-%06d-%c.txt", i, 'a'+i))
		require.NoError(t, os.WriteFile(name, []byte("content"), 0o600))
		ts := base.Add(time.Duration(i) * time.Second)
		require.NoError(t, os.Chtimes(name, ts, ts))
	}

	require.NoError(t, api.pruneChatInputSpills(dir))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, maxChatInputSpillFiles)

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)

	assert.NotContains(t, names, "spill-000000-a.txt")
	assert.NotContains(t, names, "spill-000001-b.txt")
	assert.Contains(t, names, "spill-000002-c.txt")
	assert.Contains(t, names, "spill-000011-l.txt")
}
