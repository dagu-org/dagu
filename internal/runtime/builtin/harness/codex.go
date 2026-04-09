// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&codexProvider{}) }

type codexProvider struct{}

func (p *codexProvider) Name() string       { return "codex" }
func (p *codexProvider) BinaryName() string { return "codex" }

func (p *codexProvider) BuildArgs(cfg *harnessConfig, prompt string) []string {
	args := []string{"exec", prompt}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.OutputFormat == "json" {
		args = append(args, "--json")
	}
	if cfg.FullAuto || cfg.Effort == "high" || cfg.Effort == "max" {
		args = append(args, "--full-auto")
	}
	if cfg.Sandbox != "" {
		args = append(args, "--sandbox", cfg.Sandbox)
	}
	if cfg.OutputSchema != "" {
		args = append(args, "--output-schema", cfg.OutputSchema)
	}
	if cfg.Ephemeral {
		args = append(args, "--ephemeral")
	}
	if cfg.SkipGitCheck {
		args = append(args, "--skip-git-repo-check")
	}
	args = append(args, cfg.ExtraFlags...)

	return args
}
