// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package core

import (
	"maps"
	"sort"
)

type HarnessPromptMode string

const (
	HarnessPromptModeArg   HarnessPromptMode = "arg"
	HarnessPromptModeFlag  HarnessPromptMode = "flag"
	HarnessPromptModeStdin HarnessPromptMode = "stdin"
)

type HarnessPromptPosition string

const (
	HarnessPromptPositionBeforeFlags HarnessPromptPosition = "before_flags"
	HarnessPromptPositionAfterFlags  HarnessPromptPosition = "after_flags"
)

type HarnessFlagStyle string

const (
	HarnessFlagStyleGNULong    HarnessFlagStyle = "gnu_long"
	HarnessFlagStyleSingleDash HarnessFlagStyle = "single_dash"
)

// HarnessDefinition describes how to invoke a named harness CLI.
type HarnessDefinition struct {
	Binary         string                `json:"binary,omitempty"`
	PrefixArgs     []string              `json:"prefixArgs,omitempty"`
	PromptMode     HarnessPromptMode     `json:"promptMode,omitempty"`
	PromptFlag     string                `json:"promptFlag,omitempty"`
	PromptPosition HarnessPromptPosition `json:"promptPosition,omitempty"`
	FlagStyle      HarnessFlagStyle      `json:"flagStyle,omitempty"`
	OptionFlags    map[string]string     `json:"optionFlags,omitempty"`
}

// HarnessDefinitions contains named reusable harness definitions.
// Nil values are used internally during base-config merge to delete inherited entries.
type HarnessDefinitions map[string]*HarnessDefinition

var builtinHarnessProviders = map[string]struct{}{
	"claude":   {},
	"codex":    {},
	"copilot":  {},
	"opencode": {},
	"pi":       {},
}

// IsBuiltinHarnessProvider reports whether name is a built-in harness provider.
func IsBuiltinHarnessProvider(name string) bool {
	_, ok := builtinHarnessProviders[name]
	return ok
}

// BuiltinHarnessProviderNames returns the built-in harness provider names.
func BuiltinHarnessProviderNames() []string {
	names := make([]string, 0, len(builtinHarnessProviders))
	for name := range builtinHarnessProviders {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneHarnessDefinition(def *HarnessDefinition) *HarnessDefinition {
	if def == nil {
		return nil
	}
	return &HarnessDefinition{
		Binary:         def.Binary,
		PrefixArgs:     append([]string(nil), def.PrefixArgs...),
		PromptMode:     def.PromptMode,
		PromptFlag:     def.PromptFlag,
		PromptPosition: def.PromptPosition,
		FlagStyle:      def.FlagStyle,
		OptionFlags:    maps.Clone(def.OptionFlags),
	}
}

func cloneHarnessDefinitions(defs HarnessDefinitions) HarnessDefinitions {
	if defs == nil {
		return nil
	}
	cloned := make(HarnessDefinitions, len(defs))
	for name, def := range defs {
		cloned[name] = cloneHarnessDefinition(def)
	}
	return cloned
}
