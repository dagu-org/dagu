// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// Package spec070_profiles holds black-box conformance tests for spec 070
// (runtime profiles). CRUD, enable/disable, and basic variable application
// already have CLI-level coverage in conformance/cli/profile_test.go; this
// package covers the layered defaults (global/workspace/selected) precedence,
// webhook-header profile selection, and how a selected profile's entries
// interact with a DAG's own env: and secrets: fields.
package spec070_profiles_test

import (
	"path/filepath"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// sharedEnv gives every command invoked with it the same DAGU_HOME, so a
// profile created by one command is visible to a later `start` in the same
// test (the harness gives each call a fresh isolated home by default).
func sharedEnv(t *testing.T) []string {
	t.Helper()
	return []string{"DAGU_HOME=" + filepath.Join(t.TempDir(), "dagu")}
}

// A selected profile's variable overrides a DAG's own env: entry of the same
// name: profileValues.selectedEnvs is applied after the DAG's own env in the
// final variable composition.
func TestSelectedProfileOverridesDAGEnv(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := sharedEnv(t)

	dagu.RunWithEnv(env, "profile", "create", "prof").ExpectExitCode(0)
	dagu.RunWithEnv(env, "profile", "set-var", "prof", "SHARED", "from-profile").ExpectExitCode(0)

	result := dagu.RunWithEnv(env, "start", "--profile=prof", "profile_overrides_dag_env.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "value is from-profile")
	require.NotContains(t, result.Stdout(), "from-dag-env")
}

// A DAG's own secrets: entry overrides a selected profile's variable of the
// same name: it is the last layer applied, and its resolved value is masked
// like any other secret.
func TestDAGSecretOverridesSelectedProfile(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := sharedEnv(t)

	dagu.RunWithEnv(env, "profile", "create", "prof").ExpectExitCode(0)
	dagu.RunWithEnv(env, "profile", "set-var", "prof", "SHARED", "from-profile").ExpectExitCode(0)

	result := dagu.RunWithEnv(append(env, "SOURCE_VALUE=from-dag-secret"),
		"start", "--profile=prof", "dag_secret_overrides_profile.yaml")
	result.ExpectExitCode(0)
	require.Contains(t, result.Stdout(), "*******")
	require.NotContains(t, result.Stdout(), "from-dag-secret")
	require.NotContains(t, result.Stdout(), "from-profile")
}
