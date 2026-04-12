// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package harness

import (
	"fmt"
	"sort"
)

// Provider defines how a coding agent CLI is invoked.
type Provider interface {
	// Name returns the provider identifier used in config.provider.
	Name() string

	// BinaryName returns the CLI binary name to look up in PATH.
	BinaryName() string

	// BaseArgs returns the base CLI arguments for non-interactive execution.
	// This is how the prompt is passed to the CLI (e.g., ["-p", prompt] or ["exec", prompt]).
	// Additional flags from the config map are appended after these.
	BaseArgs(prompt string) []string
}

var providers = map[string]Provider{}

func registerProvider(p Provider) {
	name := p.Name()
	if _, exists := providers[name]; exists {
		panic(fmt.Sprintf("harness: duplicate provider registration %q", name))
	}
	providers[name] = p
}

func getProvider(name string) (Provider, error) {
	p, ok := providers[name]
	if !ok {
		names := make([]string, 0, len(providers))
		for k := range providers {
			names = append(names, k)
		}
		sort.Strings(names)
		return nil, fmt.Errorf("harness: unknown provider %q; registered: %v", name, names)
	}
	return p, nil
}
