// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec067_harness_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// Containerized harness execution (a harness.run step with a step-level
// container: block) is excluded from live coverage: it requires a working
// container daemon and an image, neither guaranteed in every environment
// this suite runs in. These tests instead cover the deterministic
// configuration and dispatch failures that are reachable without ever
// reaching a daemon (or, for the last case, reachable identically whether or
// not one is even installed), matching the same restraint spec037/spec038
// already apply to their own container-dependent behavior.
func TestHarnessContainerConfigErrors(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		fixture string
		errText string
	}{
		// A custom provider whose prompt_mode is stdin has no argument form:
		// the SDK container client has no stdin, so this can only be caught
		// once the provider is resolved, at run time.
		{"container_stdin_prompt_mode.yaml", "harness: containerized harness does not support stdin input"},
		// container.name targets an existing container by name, which has no
		// image ENTRYPOINT to override with the agent binary; only exec mode
		// (container.exec) may target one.
		{"container_name_image_mode.yaml", "harness: container.name is not supported for an image-mode container step"},
		// managed: true OpenCode runs against a Dagu-hosted session, which a
		// containerized step has no access to.
		{"container_managed_opencode.yaml", "harness: managed OpenCode is not supported inside containers"},
	} {
		t.Run(tc.fixture, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			env := writeFakeHarnessScripts(dagu)
			result := dagu.RunWithEnv(env, "start", tc.fixture)
			result.ExpectNonZeroExitCode()
			result.ExpectStderrContains(tc.errText)
		})
	}

	// None of the above is caught at validate time: provider resolution (and
	// so the container-specific checks above) happens only once the step
	// actually runs.
	for _, fixture := range []string{
		"container_stdin_prompt_mode.yaml",
		"container_name_image_mode.yaml",
		"container_managed_opencode.yaml",
	} {
		t.Run("validate accepts "+fixture, func(t *testing.T) {
			t.Parallel()

			dagu := harness.NewRunner(t)
			env := writeFakeHarnessScripts(dagu)
			dagu.RunWithEnv(env, "validate", fixture).ExpectExitCode(0)
		})
	}
}

// A containerized harness step's daemon dispatch is real: pointing the
// engine's container-runtime selection at a socket that does not exist
// fails the step with a connection error, proving the container path
// genuinely attempts to reach a daemon rather than silently no-op'ing.
// DAGU_CONTAINER_RUNTIME/DAGU_PODMAN_HOST select the daemon socket from the
// engine's own process environment (never overridable by DAG/step env:), so
// this reproduces the same way whether or not a real container daemon is
// installed on the machine running this test.
func TestHarnessContainerDaemonUnreachable(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	env := writeFakeHarnessScripts(dagu)
	env = append(env,
		"DAGU_CONTAINER_RUNTIME=podman",
		"DAGU_PODMAN_HOST=unix:///nonexistent-dagu-conformance-podman.sock",
	)
	result := dagu.RunWithEnv(env, "start", "container_daemon_unreachable.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("harness: failed to initialize container client")
}
