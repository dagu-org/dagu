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

	"github.com/fatih/color"

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

var (
	green = color.New(color.FgGreen).SprintFunc()
	red   = color.New(color.FgRed).SprintFunc()
	dim   = color.New(color.Faint).SprintFunc()
	bold  = color.New(color.Bold).SprintFunc()
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
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Dagu skill into AI coding tools",
		Long: `Install the Dagu DAG authoring skill into detected AI coding tools.

Detects installed tools and installs the Dagu skill (SKILL.md) into each
tool's skill directory, enabling it to write correct Dagu DAG YAML files.

Supported tools: Claude Code, Codex, OpenCode, Gemini CLI, Copilot CLI`,
		Args: cobra.NoArgs,
		RunE: runAIInstall,
	}
	cmd.Flags().BoolP("yes", "y", false, "Install to all detected tools without prompting")
	return cmd
}

func aiTools() []aiTool {
	return []aiTool{
		{
			Name:   "Claude Code",
			Format: "skill",
			detect: func(homeDir string) string {
				if fileExists(filepath.Join(homeDir, ".claude", ".claude.json")) {
					return filepath.Join(homeDir, ".claude", "skills")
				}
				return ""
			},
		},
		{
			Name:   "Codex",
			Format: "skill",
			detect: func(homeDir string) string {
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
				if fileExists(filepath.Join(homeDir, ".gemini", "GEMINI.md")) {
					return filepath.Join(homeDir, ".gemini", "skills")
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
					if fileExists(filepath.Join(dir, "config.json")) {
						return dir
					}
				}
				dir := filepath.Join(homeDir, ".copilot")
				if fileExists(filepath.Join(dir, "config.json")) {
					return dir
				}
				return ""
			},
		},
	}
}

type detectedTool struct {
	tool       aiTool
	targetPath string
}

func runAIInstall(cmd *cobra.Command, _ []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	skipPrompt, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)
	tools := aiTools()

	var detected []detectedTool

	for _, t := range tools {
		baseDir := t.detect(homeDir)
		if baseDir == "" {
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

	if len(detected) == 0 {
		fmt.Println("No AI coding tools detected.")
		return nil
	}

	fmt.Printf("Found %s tool(s)\n\n", bold(len(detected)))

	skillFS := fileagentskill.SkillFS()

	var hadFailure bool

	for _, d := range detected {
		displayPath := tildefy(d.targetPath, homeDir)

		if !skipPrompt {
			if !promptDefault(reader, fmt.Sprintf("  %s?", bold(d.tool.Name)), true) {
				clearLine()
				fmt.Printf("  %-14s %s\n", d.tool.Name, dim("skipped"))
				continue
			}
			clearLine()
		}

		var installErr error
		if d.tool.Format == "copilot" {
			installErr = installCopilot(d.targetPath, skillFS)
		} else {
			installErr = installSkill(d.targetPath, skillFS)
		}

		if installErr != nil {
			hadFailure = true
			fmt.Fprintf(os.Stderr, "  %-14s %s %v\n", d.tool.Name, red("✗"), installErr)
			continue
		}

		fmt.Printf("  %-14s %s %s\n", d.tool.Name, green("✓"), dim(displayPath))
	}

	if hadFailure {
		return fmt.Errorf("failed to install into one or more AI tools")
	}

	return nil
}

// installSkill copies the embedded dagu skill directory to the target path.
func installSkill(targetSKILLMD string, skillFS embed.FS) error {
	targetDir := filepath.Dir(targetSKILLMD)

	return fs.WalkDir(skillFS, skillEmbedPrefix, func(path string, d fs.DirEntry, err error) error {
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
}

// installCopilot appends or replaces skill content in copilot-instructions.md.
func installCopilot(targetPath string, skillFS embed.FS) error {
	content, err := buildCopilotContent(skillFS)
	if err != nil {
		return err
	}

	wrappedContent := copilotBeginMark + "\n" + content + "\n" + copilotEndMark

	existing, readErr := os.ReadFile(targetPath) //nolint:gosec // path is constructed internally
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("read %s: %w", targetPath, readErr)
	}

	var result string
	if readErr == nil {
		existingStr := string(existing)
		beginIdx := strings.Index(existingStr, copilotBeginMark)
		endIdx := strings.Index(existingStr, copilotEndMark)

		if beginIdx >= 0 && endIdx >= 0 && endIdx > beginIdx {
			result = existingStr[:beginIdx] + wrappedContent + existingStr[endIdx+len(copilotEndMark):]
		} else {
			result = existingStr
			if !strings.HasSuffix(result, "\n") && len(result) > 0 {
				result += "\n"
			}
			result += "\n" + wrappedContent + "\n"
		}
	} else {
		result = wrappedContent + "\n"
	}

	if mkErr := os.MkdirAll(filepath.Dir(targetPath), 0o750); mkErr != nil {
		return fmt.Errorf("create directory: %w", mkErr)
	}

	return os.WriteFile(targetPath, []byte(result), 0o600)
}

// buildCopilotContent extracts SKILL.md body (without frontmatter) and appends reference files.
func buildCopilotContent(skillFS embed.FS) (string, error) {
	skillData, err := skillFS.ReadFile(skillEmbedPrefix + "/SKILL.md")
	if err != nil {
		return "", fmt.Errorf("read SKILL.md: %w", err)
	}

	body := stripFrontmatter(string(skillData))

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
	rest := content[3:]
	_, after, found := strings.Cut(rest, "\n---")
	if !found {
		return content
	}
	return strings.TrimLeft(after, "\n")
}

// promptDefault asks the user a yes/no question with a default answer.
func promptDefault(reader *bufio.Reader, prompt string, defaultYes bool) bool {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	fmt.Printf("%s %s ", prompt, dim(hint))

	response, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" {
		return defaultYes
	}
	return response == "y" || response == "yes"
}

// clearLine moves cursor to start of previous line and clears it.
func clearLine() {
	fmt.Print("\033[A\033[2K\r")
}

// dirExists checks if a directory exists.
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// fileExists checks if a regular file exists.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// tildefy replaces the home directory prefix with ~ for display.
func tildefy(path, homeDir string) string {
	if path == homeDir {
		return "~"
	}
	if strings.HasPrefix(path, homeDir+string(os.PathSeparator)) {
		return "~" + path[len(homeDir):]
	}
	return path
}
