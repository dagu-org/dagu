// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd

import (
	"bufio"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"

	"github.com/dagu-org/dagu/internal/persis/fileagentskill"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const (
	skillDirName     = "dagu"
	skillEmbedPrefix = "examples/dagu"
	copilotBeginMark = "<!-- BEGIN DAGU -->"
	copilotEndMark   = "<!-- END DAGU -->"
	copilotFileName  = "copilot-instructions.md"
	flagSkillsDir    = "skills-dir"
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
	detect func(homeDir string) []string
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
	cmd.Flags().StringArray(flagSkillsDir, nil, "Install into the specified skills directory; repeatable. If set, auto-detection is skipped")
	return cmd
}

func aiTools() []aiTool {
	return []aiTool{
		{
			Name:   "Claude Code",
			Format: "skill",
			detect: func(homeDir string) []string {
				if fileExists(filepath.Join(homeDir, ".claude", ".claude.json")) {
					return []string{filepath.Join(homeDir, ".claude", "skills")}
				}
				return nil
			},
		},
		{
			Name:   "Codex",
			Format: "skill",
			detect: func(homeDir string) []string {
				var paths []string

				if dir := resolveEnvOrExistingDir("AGENTS_HOME", filepath.Join(homeDir, ".agents")); dir != "" {
					paths = append(paths, filepath.Join(dir, "skills"))
				}
				if dir := resolveEnvOrExistingDir("CODEX_HOME", filepath.Join(homeDir, ".codex")); dir != "" {
					paths = append(paths, filepath.Join(dir, "skills"))
				}
				return uniquePaths(paths)
			},
		},
		{
			Name:   "OpenCode",
			Format: "skill",
			detect: func(homeDir string) []string {
				dir := filepath.Join(homeDir, ".config", "opencode")
				if dirExists(dir) {
					return []string{filepath.Join(dir, "skills")}
				}
				return nil
			},
		},
		{
			Name:   "Gemini CLI",
			Format: "skill",
			detect: func(homeDir string) []string {
				if fileExists(filepath.Join(homeDir, ".gemini", "GEMINI.md")) {
					return []string{filepath.Join(homeDir, ".gemini", "skills")}
				}
				return nil
			},
		},
		{
			Name:   "Copilot CLI",
			Format: "copilot",
			detect: func(homeDir string) []string {
				if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
					dir := filepath.Join(v, ".copilot")
					if fileExists(filepath.Join(dir, "config.json")) {
						return []string{dir}
					}
				}
				dir := filepath.Join(homeDir, ".copilot")
				if fileExists(filepath.Join(dir, "config.json")) {
					return []string{dir}
				}
				return nil
			},
		},
	}
}

type detectedTool struct {
	tool       aiTool
	targetPath string
}

type installState int

type copilotContentInspection struct {
	state    installState
	beginIdx int
	endIdx   int
}

const (
	installStateFresh installState = iota
	installStateOverwrite
	installStateUpdate
)

func runAIInstall(cmd *cobra.Command, _ []string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to determine home directory: %w", err)
	}

	skipPrompt, err := cmd.Flags().GetBool("yes")
	if err != nil {
		return fmt.Errorf("failed to get yes flag: %w", err)
	}

	customSkillDirs, err := cmd.Flags().GetStringArray(flagSkillsDir)
	if err != nil {
		return fmt.Errorf("failed to get skills-dir flag: %w", err)
	}

	in := cmd.InOrStdin()
	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	reader := bufio.NewReader(in)

	var detected []detectedTool
	if len(customSkillDirs) > 0 {
		detected, err = customSkillTargets(customSkillDirs)
		if err != nil {
			return err
		}
	} else {
		detected = detectAITargets(homeDir, aiTools())
	}

	if len(detected) == 0 {
		_, _ = fmt.Fprintln(out, "No AI coding tools detected.")
		return nil
	}

	_, _ = fmt.Fprintf(out, "Found %s installation target(s)\n\n", bold(len(detected)))

	skillFS := fileagentskill.SkillFS()

	var hadFailure bool

	for _, d := range detected {
		displayPath := tildefy(d.targetPath, homeDir)
		state, stateErr := detectInstallState(d)
		if stateErr != nil {
			hadFailure = true
			_, _ = fmt.Fprintf(errOut, "  %-14s %s %s: %v\n", d.tool.Name, red("✗"), dim(displayPath), stateErr)
			continue
		}

		if !skipPrompt {
			prompt := fmt.Sprintf("  %s %s?", bold(d.tool.Name), dim(displayPath))
			confirmed, promptErr := promptDefault(reader, out, prompt, true)
			if promptErr != nil {
				return fmt.Errorf("interactive confirmation required for %s: %w; rerun with --yes to install non-interactively", displayPath, promptErr)
			}
			if !confirmed {
				clearLine(out)
				_, _ = fmt.Fprintf(out, "  %-14s %s %s\n", d.tool.Name, dim("skipped"), dim(displayPath))
				continue
			}
			clearLine(out)
		}

		if state != installStateFresh && !skipPrompt {
			action := "Overwrite"
			if state == installStateUpdate {
				action = "Update"
			}
			confirmed, promptErr := promptDefault(reader, out, fmt.Sprintf("  %s existing install at %s?", action, dim(displayPath)), false)
			if promptErr != nil {
				return fmt.Errorf("interactive confirmation required for %s: %w; rerun with --yes to install non-interactively", displayPath, promptErr)
			}
			if !confirmed {
				clearLine(out)
				_, _ = fmt.Fprintf(out, "  %-14s %s %s\n", d.tool.Name, dim("kept existing"), dim(displayPath))
				continue
			}
			clearLine(out)
		}

		var installErr error
		if d.tool.Format == "copilot" {
			installErr = installCopilot(d.targetPath, skillFS)
		} else {
			installErr = installSkill(d.targetPath, skillFS)
		}

		if installErr != nil {
			hadFailure = true
			_, _ = fmt.Fprintf(errOut, "  %-14s %s %s: %v\n", d.tool.Name, red("✗"), dim(displayPath), installErr)
			continue
		}

		status := "installed"
		if state == installStateOverwrite || state == installStateUpdate {
			status = "updated"
		}
		_, _ = fmt.Fprintf(out, "  %-14s %s %s %s\n", d.tool.Name, green("✓"), status, dim(displayPath))
	}

	if hadFailure {
		return fmt.Errorf("failed to install into one or more AI tools")
	}

	return nil
}

func detectAITargets(homeDir string, tools []aiTool) []detectedTool {
	seen := make(map[string]struct{})
	var detected []detectedTool

	for _, t := range tools {
		for _, baseDir := range t.detect(homeDir) {
			var target string
			if t.Format == "copilot" {
				target = filepath.Join(baseDir, copilotFileName)
			} else {
				target = filepath.Join(baseDir, skillDirName, "SKILL.md")
			}
			target = filepath.Clean(target)

			if _, ok := seen[target]; ok {
				continue
			}
			seen[target] = struct{}{}
			detected = append(detected, detectedTool{tool: t, targetPath: target})
		}
	}

	return detected
}

func customSkillTargets(skillDirs []string) ([]detectedTool, error) {
	seen := make(map[string]struct{})
	customTool := aiTool{Name: "Custom", Format: "skill"}
	var detected []detectedTool

	for _, skillDir := range skillDirs {
		rootDir, err := validateSkillRootDir(skillDir)
		if err != nil {
			return nil, err
		}

		target := filepath.Join(rootDir, skillDirName, "SKILL.md")
		target = filepath.Clean(target)
		if _, ok := seen[target]; ok {
			continue
		}

		seen[target] = struct{}{}
		detected = append(detected, detectedTool{tool: customTool, targetPath: target})
	}

	return detected, nil
}

func detectInstallState(target detectedTool) (installState, error) {
	if target.tool.Format == "copilot" {
		content, err := os.ReadFile(target.targetPath) //nolint:gosec // path is constructed internally
		if os.IsNotExist(err) {
			return installStateFresh, nil
		}
		if err != nil {
			return installStateFresh, fmt.Errorf("read %s: %w", target.targetPath, err)
		}

		inspection, inspectErr := inspectCopilotContent(string(content))
		if inspectErr != nil {
			return installStateFresh, fmt.Errorf("inspect %s: %w", target.targetPath, inspectErr)
		}
		if inspection.state == installStateUpdate {
			return installStateUpdate, nil
		}
		return installStateFresh, nil
	}

	if fileExists(target.targetPath) {
		return installStateOverwrite, nil
	}

	return installStateFresh, nil
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
		inspection, inspectErr := inspectCopilotContent(existingStr)
		if inspectErr != nil {
			return fmt.Errorf("inspect %s: %w", targetPath, inspectErr)
		}

		if inspection.state == installStateUpdate {
			result = existingStr[:inspection.beginIdx] + wrappedContent + existingStr[inspection.endIdx+len(copilotEndMark):]
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

func inspectCopilotContent(content string) (copilotContentInspection, error) {
	beginIdx := strings.Index(content, copilotBeginMark)
	endIdx := strings.Index(content, copilotEndMark)
	hasBegin := beginIdx >= 0
	hasEnd := endIdx >= 0

	switch {
	case !hasBegin && !hasEnd:
		return copilotContentInspection{
			state:    installStateFresh,
			beginIdx: -1,
			endIdx:   -1,
		}, nil
	case hasBegin && hasEnd && endIdx > beginIdx:
		return copilotContentInspection{
			state:    installStateUpdate,
			beginIdx: beginIdx,
			endIdx:   endIdx,
		}, nil
	default:
		return copilotContentInspection{}, errors.New("found malformed DAGU markers")
	}
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
func promptDefault(reader *bufio.Reader, w io.Writer, prompt string, defaultYes bool) (bool, error) {
	hint := "[Y/n]"
	if !defaultYes {
		hint = "[y/N]"
	}
	if _, err := fmt.Fprintf(w, "%s %s ", prompt, dim(hint)); err != nil {
		return false, err
	}

	response, err := reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) || strings.TrimSpace(response) == "" {
			return false, err
		}
	}

	response = strings.ToLower(strings.TrimSpace(response))
	if response == "" {
		if err != nil {
			return false, err
		}
		return defaultYes, nil
	}
	return response == "y" || response == "yes", nil
}

// clearLine moves cursor to start of previous line and clears it.
func clearLine(w io.Writer) {
	if !isTerminalWriter(w) {
		return
	}
	_, _ = fmt.Fprint(w, "\033[A\033[2K\r")
}

func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
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

func validateSkillRootDir(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", fmt.Errorf("%s cannot be empty", flagSkillsDir)
	}

	clean := filepath.Clean(trimmed)
	base := filepath.Base(clean)
	switch base {
	case copilotFileName, "SKILL.md":
		return "", fmt.Errorf("%s %q must be a skills directory, not a file path", flagSkillsDir, path)
	case skillDirName:
		return "", fmt.Errorf("%s %q must be a skills root directory, not the %s skill directory itself", flagSkillsDir, path, skillDirName)
	}

	info, err := os.Stat(clean)
	switch {
	case err == nil && !info.IsDir():
		return "", fmt.Errorf("%s %q must be a directory", flagSkillsDir, path)
	case err != nil && !os.IsNotExist(err):
		return "", fmt.Errorf("check %s %q: %w", flagSkillsDir, path, err)
	}

	return clean, nil
}

func resolveEnvOrExistingDir(envVar, fallbackDir string) string {
	if v := os.Getenv(envVar); v != "" {
		return filepath.Clean(v)
	}
	if dirExists(fallbackDir) {
		return fallbackDir
	}
	return ""
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	var unique []string

	for _, path := range paths {
		if path == "" {
			continue
		}

		clean := filepath.Clean(path)
		if _, ok := seen[clean]; ok {
			continue
		}

		seen[clean] = struct{}{}
		unique = append(unique, clean)
	}

	return unique
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
