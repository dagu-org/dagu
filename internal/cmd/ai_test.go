// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/persis/fileagentskill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIToolDetection(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

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
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

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

func TestDetectAITargetsIncludesBothCodexDirectories(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".codex"), 0o750))

	detected := detectAITargets(homeDir, aiTools())

	var codexTargets []string
	for _, d := range detected {
		if d.tool.Name == "Codex" {
			codexTargets = append(codexTargets, d.targetPath)
		}
	}

	assert.ElementsMatch(t, []string{
		filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md"),
		filepath.Join(homeDir, ".codex", "skills", "dagu", "SKILL.md"),
	}, codexTargets)
}

func TestRunAIInstallInstallsBothCodexDirectories(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".codex"), 0o750))

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	for _, target := range []string{
		filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md"),
		filepath.Join(homeDir, ".codex", "skills", "dagu", "SKILL.md"),
	} {
		_, err := os.Stat(target)
		assert.NoError(t, err, "expected skill to be installed at %s", target)
	}
}

func TestRunAIInstallInstallsIntoCustomSkillsDir(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	customSkillsDir := filepath.Join(t.TempDir(), "skills")

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, customSkillsDir))
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	_, err := os.Stat(filepath.Join(customSkillsDir, "dagu", "SKILL.md"))
	assert.NoError(t, err)
}

func TestRunAIInstallCustomSkillsDirReplacesDetection(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))
	customSkillsDir := filepath.Join(t.TempDir(), "skills")

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, customSkillsDir))
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	_, customErr := os.Stat(filepath.Join(customSkillsDir, "dagu", "SKILL.md"))
	assert.NoError(t, customErr)

	_, autoErr := os.Stat(filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md"))
	assert.ErrorIs(t, autoErr, os.ErrNotExist)
}

func TestRunAIInstallDeduplicatesCustomSkillsDirs(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	customSkillsDir := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(customSkillsDir, "dagu", "SKILL.md")

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, customSkillsDir))
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, customSkillsDir+string(os.PathSeparator)))
	require.NoError(t, cmd.Flags().Set("yes", "true"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	_, err := os.Stat(target)
	assert.NoError(t, err)
}

func TestRunAIInstallRequiresInputWithoutYes(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))

	cmd := aiInstallCmd()
	cmd.SetIn(strings.NewReader(""))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := runAIInstall(cmd, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, "rerun with --yes")

	_, statErr := os.Stat(filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md"))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

func TestRunAIInstallRejectsInvalidCustomSkillsDir(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, filepath.Join(t.TempDir(), "copilot-instructions.md")))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	err := runAIInstall(cmd, nil)
	require.Error(t, err)
	assert.ErrorContains(t, err, flagSkillsDir)
	assert.ErrorContains(t, err, "skills directory")
}

func TestRunAIInstallPreservesExistingSkillWhenOverwriteDeclined(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	target := filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o750))
	require.NoError(t, os.WriteFile(target, []byte("custom skill"), 0o600))

	cmd := aiInstallCmd()
	cmd.SetIn(strings.NewReader("y\nn\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "custom skill", string(data))
}

func TestRunAIInstallPreservesExistingCustomSkillWhenOverwriteDeclined(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	customSkillsDir := filepath.Join(t.TempDir(), "skills")
	target := filepath.Join(customSkillsDir, "dagu", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o750))
	require.NoError(t, os.WriteFile(target, []byte("custom skill"), 0o600))

	cmd := aiInstallCmd()
	require.NoError(t, cmd.Flags().Set(flagSkillsDir, customSkillsDir))
	cmd.SetIn(strings.NewReader("y\nn\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Equal(t, "custom skill", string(data))
}

func TestRunAIInstallOverwritesExistingSkillWhenConfirmed(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	target := filepath.Join(homeDir, ".agents", "skills", "dagu", "SKILL.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(target), 0o750))
	require.NoError(t, os.WriteFile(target, []byte("custom skill"), 0o600))

	cmd := aiInstallCmd()
	cmd.SetIn(strings.NewReader("y\ny\n"))
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	require.NoError(t, runAIInstall(cmd, nil))

	data, err := os.ReadFile(target)
	require.NoError(t, err)
	assert.Contains(t, string(data), "name: dagu")
}

func TestAIToolDetectionUsesConfiguredCodexHomes(t *testing.T) {
	t.Setenv("AGENTS_HOME", filepath.Join(t.TempDir(), "agents-home"))
	t.Setenv("CODEX_HOME", filepath.Join(t.TempDir(), "codex-home"))
	t.Setenv("XDG_CONFIG_HOME", "")

	tools := aiTools()
	result := tools[1].detect(t.TempDir())

	assert.ElementsMatch(t, []string{
		filepath.Join(os.Getenv("AGENTS_HOME"), "skills"),
		filepath.Join(os.Getenv("CODEX_HOME"), "skills"),
	}, result)
}

func TestAIToolDetectionPrefersConfiguredCodexHomeOverDefault(t *testing.T) {
	t.Setenv("AGENTS_HOME", filepath.Join(t.TempDir(), "agents-home"))
	t.Setenv("CODEX_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	homeDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(homeDir, ".agents"), 0o750))

	tools := aiTools()
	result := tools[1].detect(homeDir)

	assert.Equal(t, []string{
		filepath.Join(os.Getenv("AGENTS_HOME"), "skills"),
	}, result)
}

func TestAIToolDetectionPrefersXDGCopilotHome(t *testing.T) {
	t.Setenv("AGENTS_HOME", "")
	t.Setenv("CODEX_HOME", "")

	xdgConfigHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgConfigHome)

	homeDir := t.TempDir()

	xdgCopilotDir := filepath.Join(xdgConfigHome, ".copilot")
	require.NoError(t, os.MkdirAll(xdgCopilotDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(xdgCopilotDir, "config.json"), []byte("{}"), 0o600))

	homeCopilotDir := filepath.Join(homeDir, ".copilot")
	require.NoError(t, os.MkdirAll(homeCopilotDir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(homeCopilotDir, "config.json"), []byte("{}"), 0o600))

	tools := aiTools()
	result := tools[4].detect(homeDir)

	assert.Equal(t, []string{xdgCopilotDir}, result)
}

func TestInstallSkill(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	targetSKILLMD := filepath.Join(targetDir, "dagu", "SKILL.md")

	skillFS := fileagentskill.SkillFS()
	err := installSkill(targetSKILLMD, skillFS)
	require.NoError(t, err)

	_, err = os.ReadFile(targetSKILLMD)
	require.NoError(t, err)
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

	_, err := os.ReadFile(targetSKILLMD)
	require.NoError(t, err)
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
}

func TestInstallCopilotRejectsMalformedMarkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		existing string
	}{
		{
			name:     "missing end marker",
			existing: copilotBeginMark + "\npartial content\n",
		},
		{
			name: "duplicate blocks",
			existing: copilotBeginMark + "\nold content\n" + copilotEndMark + "\n" +
				"between\n" +
				copilotBeginMark + "\nstale content\n" + copilotEndMark + "\n",
		},
		{
			name: "duplicate begin marker",
			existing: copilotBeginMark + "\nfirst\n" +
				copilotBeginMark + "\nsecond\n" + copilotEndMark + "\n",
		},
		{
			name: "duplicate end marker",
			existing: copilotBeginMark + "\ncontent\n" +
				copilotEndMark + "\n" + copilotEndMark + "\n",
		},
		{
			name:     "end marker before begin marker",
			existing: copilotEndMark + "\ncontent\n" + copilotBeginMark + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			targetDir := t.TempDir()
			targetPath := filepath.Join(targetDir, copilotFileName)
			require.NoError(t, os.WriteFile(targetPath, []byte(tt.existing), 0o600))

			skillFS := fileagentskill.SkillFS()
			err := installCopilot(targetPath, skillFS)
			require.Error(t, err)
			assert.ErrorContains(t, err, "malformed DAGU markers")
		})
	}
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

	_, err := skillFS.ReadFile("dagu/SKILL.md")
	require.NoError(t, err)
}
