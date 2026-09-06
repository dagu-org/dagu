// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec030_git_worktree_action_test

import (
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// TestDeleteBranchOtherGitRefused proves "delete_branch must not delete
// a branch that is checked out in another worktree" for the linked-worktree
// case, complementing the existing primary-tree case in
// TestGitWorktreeRemoveRuntimeErrors ("checked-out primary branch cannot be
// deleted").
//
// Two linked worktrees are forced to check out the same branch (git worktree
// add --force permits this even though it's normally refused). Removing one
// of them with delete_branch: true must be refused as a preflight failure --
// "branch checkout conflicts" is one of the checks the Remove Operation
// performs before changing the repository -- so the target worktree, its
// registration, and the branch itself must all remain exactly as they were.
func TestDeleteBranchOtherGitRefused(t *testing.T) {
	t.Parallel()

	dagu := harness.NewRunner(t)
	repo := initRepository(t, dagu)

	targetPath := dagu.ProjectPath("wt/shared-target")
	createLinkedWorktree(t, repo.path, targetPath, "shared", repo.baseCommit)

	otherPath := dagu.ProjectPath("wt/shared-other")
	createForcedLinkedWorktree(t, repo.path, otherPath, "shared")
	requireLinkedWorktree(t, repo.path, otherPath, "shared", repo.baseCommit)

	result := startWithParams(dagu, "runtime_remove_delete.yaml", "working_dir=./repo", "branch=shared")
	result.ExpectNonZeroExitCode()
	result.ExpectStderrNotEmpty()

	// A preflight failure must leave the worktree, its registration, and the
	// branch unchanged, so no outputs are published for this attempt.
	requireNoPublishedOutputs(t, dagu)

	require.DirExists(t, targetPath)
	requireLinkedWorktree(t, repo.path, targetPath, "shared", repo.baseCommit)

	require.True(t, refExists(t, repo.path, "refs/heads/shared"))
	requireLinkedWorktree(t, repo.path, otherPath, "shared", repo.baseCommit)
}

// createForcedLinkedWorktree registers a linked worktree checking out branch,
// overriding Git's default refusal to check out a branch that is already
// checked out in another worktree.
func createForcedLinkedWorktree(t *testing.T, repoPath, path, branch string) {
	t.Helper()
	gitRun(t, repoPath, "worktree", "add", "--force", path, branch)
}
