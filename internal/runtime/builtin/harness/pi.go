// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&piProvider{}) }

type piProvider struct{}

func (p *piProvider) Name() string       { return "pi" }
func (p *piProvider) BinaryName() string { return "pi" }

func (p *piProvider) BaseArgs(prompt string) []string {
	return []string{"-p", prompt}
}
