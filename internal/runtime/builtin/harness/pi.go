// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

func init() { registerProvider(&piProvider{}) }

type piProvider struct{}

func (p *piProvider) Name() string       { return "pi" }
func (p *piProvider) BinaryName() string { return "pi" }

func (p *piProvider) BuildArgs(cfg *harnessConfig, prompt string) []string {
	args := []string{"-p", prompt}

	if cfg.PiProvider != "" {
		args = append(args, "--provider", cfg.PiProvider)
	}
	if cfg.Model != "" {
		args = append(args, "--model", cfg.Model)
	}
	if cfg.OutputFormat == "json" {
		args = append(args, "--mode", "json")
	}

	// Use explicit thinking if set, otherwise map from effort
	if cfg.Thinking != "" {
		args = append(args, "--thinking", cfg.Thinking)
	} else if cfg.Effort != "" {
		if t := mapEffortToThinking(cfg.Effort); t != "" {
			args = append(args, "--thinking", t)
		}
	}

	if cfg.NoTools {
		args = append(args, "--no-tools")
	} else if cfg.Tools != "" {
		args = append(args, "--tools", cfg.Tools)
	}
	if cfg.NoExtensions {
		args = append(args, "--no-extensions")
	}
	if cfg.Session != "" {
		args = append(args, "--session", cfg.Session)
	}
	args = append(args, cfg.ExtraFlags...)

	return args
}

// mapEffortToThinking converts the generic effort level to Pi's thinking level.
func mapEffortToThinking(effort string) string {
	switch effort {
	case "low":
		return "low"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "max":
		return "xhigh"
	default:
		return ""
	}
}
