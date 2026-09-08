// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec067_harness_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
)

// Container configuration failures require no daemon or downloaded image.
func TestContainerConfig(t *testing.T) {
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

	// General validation accepts these runtime-only container restrictions.
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

// An unavailable daemon must fail dispatch without pulling an image.
func TestContainerUnavailable(t *testing.T) {
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
