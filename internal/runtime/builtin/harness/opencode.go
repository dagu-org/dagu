// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&opencodeProvider{}) }

type opencodeProvider struct{}

func (p *opencodeProvider) Name() string       { return "opencode" }
func (p *opencodeProvider) BinaryName() string { return "opencode" }

func (p *opencodeProvider) BuildArgs(cfg *harnessConfig, prompt string) []string {
	args := []string{"run", prompt}

	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.OutputFormat == "json" {
		args = append(args, "--format", "json")
	}
	if cfg.File != "" {
		args = append(args, "--file", cfg.File)
	}
	if cfg.Agent != "" {
		args = append(args, "--agent", cfg.Agent)
	}
	if cfg.Title != "" {
		args = append(args, "--title", cfg.Title)
	}
	args = append(args, cfg.ExtraFlags...)

	return args
}
