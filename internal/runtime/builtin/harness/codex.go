// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&codexProvider{}) }

type codexProvider struct{}

func (p *codexProvider) Name() string       { return "codex" }
func (p *codexProvider) BinaryName() string { return "codex" }

func (p *codexProvider) BaseArgs(prompt string) []string {
	return []string{"exec", prompt}
}

func (p *codexProvider) DefaultConfig() map[string]any {
	return map[string]any{
		"skip_git_repo_check": true,
	}
}
