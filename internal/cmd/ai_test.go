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

	// Create tool directories with their indicator files
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".claude", ".claude.json"), []byte("{}"), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".config", "opencode"), 0o750))

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".gemini"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".gemini", "GEMINI.md"), []byte(""), 0o600))

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".copilot"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(homeDir, ".copilot", "config.json"), []byte("{}"), 0o600))

	tests := []struct {
		name     string
		useHome  string
		toolIdx  int
		expected bool
	}{
		{name: "claude code detected", useHome: homeDir, toolIdx: 0, expected: true},
		{name: "claude code not detected", useHome: "", toolIdx: 0, expected: false},
		{name: "codex detected via .agents", useHome: homeDir, toolIdx: 1, expected: true},
		{name: "opencode detected", useHome: homeDir, toolIdx: 2, expected: true},
		{name: "gemini detected", useHome: homeDir, toolIdx: 3, expected: true},
		{name: "copilot detected", useHome: homeDir, toolIdx: 4, expected: true},
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

func TestAIToolDetectionRequiresIndicatorFile(t *testing.T) {
	t.Parallel()

	homeDir := t.TempDir()

	// Create .claude directory without .claude.json — should NOT be detected
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".claude"), 0o750))

	// Create .gemini directory without GEMINI.md — should NOT be detected
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".gemini"), 0o750))

	// Create .copilot directory without config.json — should NOT be detected
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".copilot"), 0o750))

	tools := aiTools()
	assert.Empty(t, tools[0].detect(homeDir), "Claude Code should not be detected without .claude.json")
	assert.Empty(t, tools[3].detect(homeDir), "Gemini CLI should not be detected without GEMINI.md")
	assert.Empty(t, tools[4].detect(homeDir), "Copilot CLI should not be detected without config.json")
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

func TestInstallSkillOverwrites(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetSKILLMD := filepath.Join(targetDir, "dagu", "SKILL.md")

	// Install once
	skillFS := fileagentskill.SkillFS()
	require.NoError(t, installSkill(targetSKILLMD, skillFS))

	// Install again — should overwrite without error
	require.NoError(t, installSkill(targetSKILLMD, skillFS))

	data, err := os.ReadFile(targetSKILLMD)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: dagu")
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
	require.NoError(t, os.WriteFile(targetPath, []byte(existing), 0o600))

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
	require.NoError(t, os.WriteFile(targetPath, []byte(existing), 0o600))

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
