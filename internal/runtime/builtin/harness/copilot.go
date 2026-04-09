// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&copilotProvider{}) }

type copilotProvider struct{}

func (p *copilotProvider) Name() string       { return "copilot" }
func (p *copilotProvider) BinaryName() string { return "copilot" }

func (p *copilotProvider) BaseArgs(prompt string) []string {
	return []string{"-p", prompt}
}
