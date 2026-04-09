// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import "fmt"

// Provider defines how a coding agent CLI's arguments are built.
type Provider interface {
	// Name returns the provider identifier (claude, codex, opencode, pi).
	Name() string

	// BinaryName returns the CLI binary name to look up in PATH.
	BinaryName() string

	// BuildArgs translates the common harnessConfig and prompt into
	// CLI arguments for this provider.
	BuildArgs(cfg *harnessConfig, prompt string) []string
}

var providers = map[string]Provider{}

func registerProvider(p Provider) {
	providers[p.Name()] = p
}

func getProvider(name string) (Provider, error) {
	p, ok := providers[name]
	if !ok {
		return nil, fmt.Errorf("harness: unknown provider %q; supported: claude, codex, opencode, pi", name)
	}
	return p, nil
}
