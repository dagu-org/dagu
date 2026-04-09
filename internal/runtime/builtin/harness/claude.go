// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import "strconv"

func init() { registerProvider(&claudeProvider{}) }

type claudeProvider struct{}

func (p *claudeProvider) Name() string       { return "claude" }
func (p *claudeProvider) BinaryName() string { return "claude" }

func (p *claudeProvider) BuildArgs(cfg *harnessConfig, prompt string) []string {
	args := []string{"-p", prompt}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.Effort != "" {
		args = append(args, "--effort", cfg.Effort)
	}
	switch cfg.OutputFormat {
	case "json":
		args = append(args, "--output-format", "json")
	case "stream-json":
		args = append(args, "--output-format", "stream-json")
	}
	if cfg.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(cfg.MaxTurns))
	}
	if cfg.MaxBudgetUSD > 0 {
		args = append(args, "--max-budget-usd", strconv.FormatFloat(cfg.MaxBudgetUSD, 'f', -1, 64))
	}
	if cfg.PermissionMode != "" {
		args = append(args, "--permission-mode", cfg.PermissionMode)
	}
	if cfg.AllowedTools != "" {
		args = append(args, "--allowedTools", cfg.AllowedTools)
	}
	if cfg.DisallowedTools != "" {
		args = append(args, "--disallowedTools", cfg.DisallowedTools)
	}
	if cfg.SystemPrompt != "" {
		args = append(args, "--system-prompt", cfg.SystemPrompt)
	}
	if cfg.AppendSystemPrompt != "" {
		args = append(args, "--append-system-prompt", cfg.AppendSystemPrompt)
	}
	if cfg.Bare {
		args = append(args, "--bare")
	}
	if cfg.AddDir != "" {
		args = append(args, "--add-dir", cfg.AddDir)
	}
	if cfg.Worktree {
		args = append(args, "--worktree")
	}
	args = append(args, cfg.ExtraFlags...)

	return args
}
