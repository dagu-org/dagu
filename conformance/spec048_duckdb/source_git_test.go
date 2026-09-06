// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

// This file covers the "source:<target>@version" action reference form
// (resolveSourceBundle in internal/runtime/builtin/action/resolver.go)
// against a genuine, custom, non-official git remote -- not the local
// bundle testdata/echo_action/testdata/bad_output_action TestLocalAction/
// TestActionBoundary already exercise, and not the official
// dagucloud/duckdb short-name convention this package's other live tests
// use. A target this specific ("git://host:port/name") is never mistaken
// for a local directory by resolveSourceBundle (unlike a plain path or a
// file:// URL, both of which short-circuit to reading a directory tree
// directly), so it is forced through the same real
// clone-over-network/checkout/cache-by-resolved-SHA code path
// (cloneGitSource) any non-official git-hosted action would use.
//
// startGitActionServer (source_git_helper_test.go) builds a one-commit,
// tag "v1" echo-action bundle and serves it with a real "git daemon"
// process on a free loopback port, so this is a genuine git clone over a
// real (if local) network transport, not a shortcut -- with no dependency
// on any actual third-party git host.
package spec048_duckdb_test

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// gitActionServerEnv returns the host environment entry
// source_git_*.yaml's own env: block resolves with.SERVER_BASE from,
// pointed at the given git daemon's own loopback port, plus the git config
// overrides (see gitEnv) dagu's own git subprocess calls need on this
// machine.
func gitActionServerEnv(server gitActionServer) []string {
	return append([]string{"HOST_SERVER_BASE=git://127.0.0.1:" + strconv.Itoa(server.Port)}, gitEnv...)
}

func TestSourceGitURLLiveClone(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	server := startGitActionServer(t, "echo-action")

	result := dagu.RunWithEnv(gitActionServerEnv(server), "start", "source_git_success.yaml")
	result.ExpectExitCode(0)

	var output struct {
		Echoed string `json:"echoed"`
	}
	require.NoError(t, json.Unmarshal([]byte(stepStdout(t, result.Stdout(), 0)), &output))
	require.Equal(t, "hello-from-real-git-clone", output.Echoed)
}

func TestSourceGitURLBadVersion(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	server := startGitActionServer(t, "echo-action")

	result := dagu.RunWithEnv(gitActionServerEnv(server), "start", "source_git_bad_version.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("did not match any file(s) known to git")
}

func TestSourceGitURLBadRepo(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	server := startGitActionServer(t, "echo-action")

	result := dagu.RunWithEnv(gitActionServerEnv(server), "start", "source_git_bad_repo.yaml")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrContains("repository not exported")
}

// dagu validate never resolves any action reference -- including a
// source:git://... one -- so it never reaches the network at all. This
// fixture points at an address nothing listens on (port 1, a reserved
// port no process here can bind); if validate ever tried to resolve it,
// it would hang or fail slowly instead of returning immediately.
func TestSourceGitURLValidateNeverResolves(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	result := dagu.Run("validate", "source_git_unreachable.yaml")
	result.ExpectExitCode(0)
}
