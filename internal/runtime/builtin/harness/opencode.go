// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&opencodeProvider{}) }

type opencodeProvider struct{}

func (p *opencodeProvider) Name() string       { return "opencode" }
func (p *opencodeProvider) BinaryName() string { return "opencode" }

func (p *opencodeProvider) BaseArgs(prompt string) []string {
	return []string{"run", prompt}
}
