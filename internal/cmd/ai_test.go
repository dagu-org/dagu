// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/persis/fileagentskill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIToolDetection(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()

	// Create tool directories for detection tests
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "opencode"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".gemini"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".copilot"), 0o750))

	tests := []struct {
		name     string
		useHome  string // which home dir to use for detection
		toolIdx  int
		expected bool
	}{
		{
			name:     "claude code detected",
			useHome:  homeDir,
			toolIdx:  0,
			expected: true,
		},
		{
			name:     "claude code not detected",
			useHome:  "", // will use empty temp dir
			toolIdx:  0,
			expected: false,
		},
		{
			name:     "codex detected via .agents",
			useHome:  homeDir,
			toolIdx:  1,
			expected: true,
		},
		{
			name:     "opencode detected",
			useHome:  homeDir,
			toolIdx:  2,
			expected: true,
		},
		{
			name:     "gemini detected",
			useHome:  homeDir,
			toolIdx:  3,
			expected: true,
		},
		{
			name:     "copilot detected",
			useHome:  homeDir,
			toolIdx:  4,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools := aiTools()
			detectHome := tt.useHome
			if detectHome == "" {
				detectHome = t.TempDir()
			}
			result := tools[tt.toolIdx].detect(detectHome)
			if tt.expected {
				assert.NotEmpty(t, result)
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestInstallSkill(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetSKILLMD := filepath.Join(targetDir, "dagu", "SKILL.md")

	skillFS := fileagentskill.SkillFS()
	err := installSkill(targetSKILLMD, skillFS)
	require.NoError(t, err)

	// Check SKILL.md exists
	data, err := os.ReadFile(targetSKILLMD)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: dagu")
	assert.Contains(t, string(data), "Dagu DAG Authoring Reference")

	// Check references exist
	refDir := filepath.Join(targetDir, "dagu", "references")
	for _, name := range []string{"cli.md", "schema.md", "executors.md", "env.md", "pitfalls.md"} {
		_, err := os.Stat(filepath.Join(refDir, name))
		assert.NoError(t, err, "reference file %s should exist", name)
	}
}

func TestInstallCopilotFresh(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, copilotFileName)

	skillFS := fileagentskill.SkillFS()
	err := installCopilot(targetPath, skillFS)
	require.NoError(t, err)

	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, copilotBeginMark)
	assert.Contains(t, content, copilotEndMark)
	assert.Contains(t, content, "Dagu DAG Authoring Reference")
	// Should not contain frontmatter
	assert.NotContains(t, content, "name: dagu")
}

func TestInstallCopilotReplace(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, copilotFileName)

	// Write existing file with markers
	existing := "# My Instructions\n\nSome content.\n\n" +
		copilotBeginMark + "\nold content\n" + copilotEndMark + "\n\n# More stuff\n"
	require.NoError(t, os.WriteFile(targetPath, []byte(existing), 0o644))

	skillFS := fileagentskill.SkillFS()
	err := installCopilot(targetPath, skillFS)
	require.NoError(t, err)

	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "# My Instructions")
	assert.Contains(t, content, "# More stuff")
	assert.NotContains(t, content, "old content")
	assert.Contains(t, content, "Dagu DAG Authoring Reference")
	// Markers should appear exactly once
	assert.Equal(t, 1, strings.Count(content, copilotBeginMark))
	assert.Equal(t, 1, strings.Count(content, copilotEndMark))
}

func TestInstallCopilotAppend(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, copilotFileName)

	// Write existing file without markers
	existing := "# My Instructions\n\nSome existing content."
	require.NoError(t, os.WriteFile(targetPath, []byte(existing), 0o644))

	skillFS := fileagentskill.SkillFS()
	err := installCopilot(targetPath, skillFS)
	require.NoError(t, err)

	data, err := os.ReadFile(targetPath)
	require.NoError(t, err)

	content := string(data)
	assert.Contains(t, content, "# My Instructions")
	assert.Contains(t, content, "Some existing content.")
	assert.Contains(t, content, copilotBeginMark)
	assert.Contains(t, content, copilotEndMark)
	assert.Contains(t, content, "Dagu DAG Authoring Reference")
}

func TestSkillExistsSkillFormat(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, "dagu", "SKILL.md")

	d := detectedTool{
		tool:       aiTool{Format: "skill"},
		targetPath: targetPath,
	}

	// Not exists
	assert.False(t, skillExists(d))

	// Create the file
	require.NoError(t, os.MkdirAll(filepath.Dir(targetPath), 0o750))
	require.NoError(t, os.WriteFile(targetPath, []byte("test"), 0o644))

	// Now exists
	assert.True(t, skillExists(d))
}

func TestSkillExistsCopilotFormat(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetPath := filepath.Join(targetDir, copilotFileName)

	d := detectedTool{
		tool:       aiTool{Format: "copilot"},
		targetPath: targetPath,
	}

	// File doesn't exist
	assert.False(t, skillExists(d))

	// File exists but no markers
	require.NoError(t, os.WriteFile(targetPath, []byte("some content"), 0o644))
	assert.False(t, skillExists(d))

	// File with markers
	require.NoError(t, os.WriteFile(targetPath, []byte("before\n"+copilotBeginMark+"\ncontent\n"+copilotEndMark), 0o644))
	assert.True(t, skillExists(d))
}

func TestStripFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "with frontmatter",
			input:    "---\nname: test\n---\n# Content",
			expected: "# Content",
		},
		{
			name:     "without frontmatter",
			input:    "# Content",
			expected: "# Content",
		},
		{
			name:     "unclosed frontmatter",
			input:    "---\nname: test\n# Content",
			expected: "---\nname: test\n# Content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := stripFrontmatter(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestTildefy(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "~/.claude/skills/dagu/SKILL.md", tildefy("/home/user/.claude/skills/dagu/SKILL.md", "/home/user"))
	assert.Equal(t, "/other/path", tildefy("/other/path", "/home/user"))
}

func TestEmbeddedSkillFS(t *testing.T) {
	t.Parallel()

	skillFS := fileagentskill.SkillFS()

	// Verify SKILL.md exists and has correct frontmatter
	data, err := skillFS.ReadFile("examples/dagu/SKILL.md")
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: dagu")

	// Verify all reference files exist
	for _, name := range []string{"cli.md", "schema.md", "executors.md", "env.md", "pitfalls.md"} {
		_, err := skillFS.ReadFile("examples/dagu/references/" + name)
		assert.NoError(t, err, "reference file %s should be embedded", name)
	}
}
