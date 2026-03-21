// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package bundledskills

import "embed"

const (
	DaguSkillDir      = "dagu"
	DaguReferencesDir = DaguSkillDir + "/references"
)

var exampleSkillIDs []string

// Assets contains the bundled skill content shipped with the binary.
// Patterns are explicit so repo-local docs and Go files in this directory are not embedded.
//
//go:embed dagu/SKILL.md dagu/references/*.md
var Assets embed.FS

// ExampleIDs returns the bundled example skill IDs seeded for first-time users.
func ExampleIDs() []string {
	return append([]string(nil), exampleSkillIDs...)
}
