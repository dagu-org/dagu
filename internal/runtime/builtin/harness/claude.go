// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&claudeProvider{}) }

type claudeProvider struct{}

func (p *claudeProvider) Name() string       { return "claude" }
func (p *claudeProvider) BinaryName() string { return "claude" }

func (p *claudeProvider) BaseArgs(prompt string) []string {
	return []string{"-p", prompt}
}
