// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cli_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

func TestSyncDisabled(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.RunWithEnv(sharedEnv(t), "sync", "status")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("git sync is not enabled")
}

// Status reports configuration without contacting the remote repository.
func TestSyncConfiguredStatus(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	const repository = "https://example.invalid/dagu/workflows.git"
	env := append(sharedEnv(t),
		"DAGU_GITSYNC_ENABLED=true",
		"DAGU_GITSYNC_REPOSITORY="+repository,
		"DAGU_GITSYNC_BRANCH=main",
		"DAGU_GITSYNC_AUTH_TOKEN=conformance-token",
	)

	result := dagu.RunWithEnv(env, "sync", "status")
	result.ExpectExitCode(0)
	requireLineContains(t, result.Stdout(), "Repository:", repository)
	requireLineContains(t, result.Stdout(), "Branch:", "main")
}
