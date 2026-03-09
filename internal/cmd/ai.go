// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bufio"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/persis/fileagentskill"
	"github.com/spf13/cobra"
)

const (
	skillDirName     = "dagu"
	skillEmbedPrefix = "examples/dagu"
	copilotBeginMark = "<!-- BEGIN DAGU -->"
	copilotEndMark   = "<!-- END DAGU -->"
	copilotFileName  = "copilot-instructions.md"
)

// aiTool describes an external AI coding tool and how to detect/install skills.
type aiTool struct {
	Name   string
	Format string // "skill" or "copilot"
	detect func(homeDir string) string
}

// AI returns the "ai" parent command.
func AI() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ai",
		Short: "AI coding tool integrations",
	}
	cmd.AddCommand(aiInstallCmd())
	return cmd
}

func aiInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install Dagu skill into AI coding tools",
		Long: `Install the Dagu DAG authoring skill into detected AI coding tools.

This interactive wizard detects installed AI coding tools and installs the
Dagu skill (SKILL.md) into each tool's skill directory, enabling the tool
to write correct Dagu DAG YAML files.

Supported tools: Claude Code, Codex, OpenCode, Gemini CLI, Copilot CLI`,
		Args: cobra.NoArgs,
		RunE: runAIInstall,
	}
}

func aiTools() []aiTool {
	return []aiTool{
		{
			Name:   "Claude Code",
			Format: "skill",
			detect: func(homeDir string) string {
				dir := filepath.Join(homeDir, ".claude")
				if dirExists(dir) {
					return filepath.Join(dir, "skills")
				}
				return ""
			},
		},
		{
			Name:   "Codex",
			Format: "skill",
			detect: func(homeDir string) string {
				// Check $AGENTS_HOME first (new), then ~/.agents/, then $CODEX_HOME (legacy), then ~/.codex/
				if v := os.Getenv("AGENTS_HOME"); v != "" {
					if dirExists(v) {
						return filepath.Join(v, "skills")
					}
				}
				if dir := filepath.Join(homeDir, ".agents"); dirExists(dir) {
					return filepath.Join(dir, "skills")
				}
				if v := os.Getenv("CODEX_HOME"); v != "" {
					if dirExists(v) {
						return filepath.Join(v, "skills")
					}
				}
				if dir := filepath.Join(homeDir, ".codex"); dirExists(dir) {
					return filepath.Join(dir, "skills")
				}
				return ""
			},
		},
		{
			Name:   "OpenCode",
			Format: "skill",
			detect: func(homeDir string) string {
				dir := filepath.Join(homeDir, ".config", "opencode")
				if dirExists(dir) {
					return filepath.Join(dir, "skills")
				}
				return ""
			},
		},
		{
			Name:   "Gemini CLI",
			Format: "skill",
			detect: func(homeDir string) string {
				dir := filepath.Join(homeDir, ".gemini")
				if dirExists(dir) {
					return filepath.Join(dir, "skills")
				}
				return ""
			},
		},
		{
			Name:   "Copilot CLI",
			Format: "copilot",
			detect: func(homeDir string) string {
				if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
					dir := filepath.Join(v, ".copilot")
					if dirExists(dir) {
						return dir
					}
				}
				dir := filepath.Join(homeDir, ".copilot")
				if dirExists(dir) {
					return dir
				}
				return ""
			},
		},
	}
}

type detectedTool struct {
	tool       aiTool
	targetPath string // full path to the skill file or directory
}

func runAIInstall(_ *cobra.Command, _ []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)
	tools := aiTools()

	var detected []detectedTool
	var notDetected []string

	for _, t := range tools {
		baseDir := t.detect(homeDir)
		if baseDir == "" {
			notDetected = append(notDetected, t.Name)
			continue
		}
		var target string
		if t.Format == "copilot" {
			target = filepath.Join(baseDir, copilotFileName)
		} else {
			target = filepath.Join(baseDir, skillDirName, "SKILL.md")
		}
		detected = append(detected, detectedTool{tool: t, targetPath: target})
	}

	fmt.Println("Dagu AI skill installer")
	fmt.Println()

	if len(detected) == 0 {
		fmt.Println("No AI coding tools detected.")
		fmt.Println()
		fmt.Println("Supported tools: Claude Code, Codex, OpenCode, Gemini CLI, Copilot CLI")
		return nil
	}

	fmt.Println("Detected AI coding tools:")
	for i, d := range detected {
		fmt.Printf("  [%d] %-14s %s\n", i+1, d.tool.Name, tildefy(d.targetPath, homeDir))
	}
	fmt.Println()

	skillFS := fileagentskill.SkillFS()

	for _, d := range detected {
		exists := skillExists(d)
		if !promptDefault(reader, fmt.Sprintf("Install to %s?", d.tool.Name), true) {
			fmt.Println("  — Skipped")
			fmt.Println()
			continue
		}

		if exists {
			if !promptDefault(reader, "  Skill already exists. Overwrite?", false) {
				fmt.Println("  — Skipped")
				fmt.Println()
				continue
			}
		}

		var installErr error
		if d.tool.Format == "copilot" {
			installErr = installCopilot(d.targetPath, skillFS)
		} else {
			installErr = installSkill(d.targetPath, skillFS)
		}

		if installErr != nil {
			fmt.Printf("  ✗ Error: %v\n", installErr)
			fmt.Println()
			continue
		}

		action := "Installed"
		if exists {
			action = "Updated"
		}
		fmt.Printf("  ✓ %s %s\n", action, tildefy(d.targetPath, homeDir))
		fmt.Println()
	}

	if len(notDetected) > 0 {
		fmt.Printf("Not detected: %s\n", strings.Join(notDetected, ", "))
	}

	return nil
}

// installSkill copies the embedded dagu skill directory to the target path.
func installSkill(targetSKILLMD string, skillFS embed.FS) error {
	targetDir := filepath.Dir(targetSKILLMD) // .../skills/dagu

	err := fs.WalkDir(skillFS, skillEmbedPrefix, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		relPath := strings.TrimPrefix(path, skillEmbedPrefix+"/")
		destPath := filepath.Join(targetDir, relPath)

		data, readErr := skillFS.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("read embedded file %s: %w", path, readErr)
		}

		if mkErr := os.MkdirAll(filepath.Dir(destPath), 0o750); mkErr != nil {
			return fmt.Errorf("create directory %s: %w", filepath.Dir(destPath), mkErr)
		}

		if wErr := os.WriteFile(destPath, data, 0o600); wErr != nil {
			return fmt.Errorf("write %s: %w", destPath, wErr)
		}

		return nil
	})

	return err
}

// installCopilot appends or replaces skill content in copilot-instructions.md.
func installCopilot(targetPath string, skillFS embed.FS) error {
	// Build content from SKILL.md body + reference files
	content, err := buildCopilotContent(skillFS)
	if err != nil {
		return err
	}

	wrappedContent := copilotBeginMark + "\n" + content + "\n" + copilotEndMark

	existing, readErr := os.ReadFile(targetPath) //nolint:gosec // path is constructed internally, not from user input
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", targetPath, readErr)
	}

	var result string
	if readErr == nil {
		existingStr := string(existing)
		beginIdx := strings.Index(existingStr, copilotBeginMark)
		endIdx := strings.Index(existingStr, copilotEndMark)

		if beginIdx >= 0 && endIdx >= 0 && endIdx > beginIdx {
			// Replace between markers
			result = existingStr[:beginIdx] + wrappedContent + existingStr[endIdx+len(copilotEndMark):]
		} else {
			// Append
			result = existingStr
			if !strings.HasSuffix(result, "\n") && len(result) > 0 {
				result += "\n"
			}
			result += "\n" + wrappedContent + "\n"
		}
	} else {
		// New file
		result = wrappedContent + "\n"
	}

	if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o750); mkErr != nil {
		return fmt.Errorf("create directory: %w", mkErr)
	}

	return os.WriteFile(targetPath, []byte(result), 0o600)
}

// buildCopilotContent extracts SKILL.md body (without frontmatter) and appends reference files.
func buildCopilotContent(skillFS embed.FS) (string, error) {
	// Read SKILL.md and strip frontmatter
	skillData, err := skillFS.ReadFile(skillEmbedPrefix + "/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("read SKILL.md: %w", err)
	}

	body := stripFrontmatter(string(skillData))

	// Append reference files
	refPrefix := skillEmbedPrefix + "/references"
	entries, err := fs.ReadDir(skillFS, refPrefix)
	if err != nil {
		return "", fmt.Errorf("read references directory: %w", err)
	}

	var parts []string
	parts = append(parts, strings.TrimSpace(body))

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, readErr := skillFS.ReadFile(refPrefix + "/" + entry.Name())
		if readErr != nil {
			return "", fmt.Errorf("read reference %s: %w", entry.Name(), readErr)
		}
		parts = append(parts, strings.TrimSpace(string(data)))
	}

	return strings.Join(parts, "\n\n"), nil
}

// stripFrontmatter removes YAML frontmatter delimited by --- from the start of content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find closing ---
	rest := content[3:]
	_, after, found := strings.Cut(rest, "\n---")
	if !found {
		return content
	}
	return strings.TrimLeft(after, "\n")
}

// skillExists checks if the skill is already installed at the target.
func skillExists(d detectedTool) bool {
	if d.tool.Format == "copilot" {
		data, err := os.ReadFile(d.targetPath)
		if err != nil {
			return false
		}
		return strings.Contains(string(data), copilotBeginMark)
	}
	_, err := os.Stat(d.targetPath)
	return err == nil
}

// promptDefault asks the user a yes/no question with a default answer.
func promptDefault(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Printf("%s %s ", prompt, hint)

	response, err := reader.ReadString('\n')
	if err != nil {
		return defaultYes
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" {
		return defaultYes
	}
	return response == "y" || response == "yes"
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// tildefy replaces the home directory prefix with ~ for display.
func tildefy(path, homeDir string) string {
	if strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}
