// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package spec030_git_worktree_action_test

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/dagucloud/dagu/v2/conformance/harness"
	"github.com/stretchr/testify/require"
)

// TestGitMutationSerialization exercises "Mutation Serialization": add and
// remove operations resolving to the same common Git directory must execute
// their inspect-and-mutate sequences serially, including across different
// DAG runs.
//
// This is inherently a stress test, not a deterministic proof: without
// serialization a race here would surface as flaky corruption under
// concurrent load, not as a guaranteed failure on any single run. The test
// launches several add and several remove operations against the same
// repository as separate, concurrently-running `dagu start` processes (real
// separate DAG runs, not goroutines sharing one run), each targeting its own
// distinct branch and path so no operation's target legitimately conflicts
// with another's. If mutation serialization is missing or broken, this
// concurrent load is expected to eventually surface as an operation failing
// outright or as corrupted, inconsistent, or lost worktree registrations in
// the shared repository afterward -- not necessarily on every run.
func TestGitMutationSerialization(t *testing.T) {
	dagu := harness.NewRunner(t)
	repo := initRepository(t, dagu)

	const addCount = 6
	const removeCount = 6

	// Pre-create the worktrees the concurrent phase will remove, before the
	// race starts, so only the mutation itself -- not fixture setup -- is
	// under contention.
	removePaths := make([]string, removeCount)
	for i := range removeCount {
		branch := fmt.Sprintf("concurrent-remove-%d", i)
		path := dagu.ProjectPath(fmt.Sprintf("wt/remove-%d", i))
		createLinkedWorktree(t, repo.path, path, branch, repo.baseCommit)
		removePaths[i] = path
	}

	type job struct {
		fixture string
		params  []string
	}
	jobs := make([]job, 0, addCount+removeCount)
	for i := range addCount {
		branch := fmt.Sprintf("concurrent-add-%d", i)
		path := fmt.Sprintf("../wt/add-%d", i)
		jobs = append(jobs, job{
			fixture: "runtime_add.yaml",
			params:  []string{"working_dir=./repo", "branch=" + branch, "path=" + path},
		})
	}
	for i := range removeCount {
		branch := fmt.Sprintf("concurrent-remove-%d", i)
		jobs = append(jobs, job{
			fixture: "runtime_remove_branch.yaml",
			params:  []string{"working_dir=./repo", "branch=" + branch},
		})
	}

	// A closed start gate lets every goroutine race to invoke dagu at
	// (as close to) the same instant as possible, maximizing overlap between
	// the concurrent operations' inspect-and-mutate sequences.
	ready := make(chan struct{})
	results := make([]*harness.Result, len(jobs))
	var wg sync.WaitGroup
	for i, j := range jobs {
		wg.Add(1)
		go func(i int, j job) {
			defer wg.Done()
			<-ready
			results[i] = startWithParams(dagu, j.fixture, j.params...)
		}(i, j)
	}
	close(ready)
	wg.Wait()

	for i, result := range results {
		require.Equalf(t, 0, result.ExitCode(), "job %d (%s %v) failed:\nstdout:\n%s\nstderr:\n%s",
			i, jobs[i].fixture, jobs[i].params, result.Stdout(), result.Stderr())
	}

	for i := range addCount {
		branch := fmt.Sprintf("concurrent-add-%d", i)
		path := dagu.ProjectPath(fmt.Sprintf("wt/add-%d", i))
		require.True(t, refExists(t, repo.path, "refs/heads/"+branch), "branch %s was not created", branch)
		requireLinkedWorktree(t, repo.path, path, branch, repo.baseCommit)
	}

	for i := range removeCount {
		branch := fmt.Sprintf("concurrent-remove-%d", i)
		require.NoDirExists(t, removePaths[i])
		require.True(t, refExists(t, repo.path, "refs/heads/"+branch), "branch %s was unexpectedly deleted", branch)
		requireNoLinkedWorktree(t, repo.path, removePaths[i], branch)
	}

	// The repository's own worktree registry must be exactly the primary
	// tree plus the surviving "add" worktrees: no duplicate, missing, or
	// stray entry left by a lost update under concurrent mutation.
	entries := listWorktrees(t, repo.path)
	require.Len(t, entries, 1+addCount)

	// No leftover Git administrative lock file from an interrupted or
	// unsynchronized concurrent mutation.
	lockMatches, err := filepath.Glob(filepath.Join(repo.path, ".git", "worktrees", "*", "*.lock"))
	require.NoError(t, err)
	require.Empty(t, lockMatches, "stale worktree admin lock files: %v", lockMatches)
}
