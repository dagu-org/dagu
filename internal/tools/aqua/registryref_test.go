// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package aqua

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testLatestSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

type registryRequestCounts struct {
	releases atomic.Int32
	commits  atomic.Int32
}

type registryTestServer struct {
	url     string
	calls   *registryRequestCounts
	fail    *atomic.Bool
	started <-chan struct{}
	finish  func()
}

// Hold the cold response until callers overlap, without making the test depend
// on the resolver's synchronization implementation.
func blockedRegistryServer(t *testing.T, callers int) *registryTestServer {
	t.Helper()
	var calls registryRequestCounts
	var fail atomic.Bool
	started := make(chan struct{})
	allStarted := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/aquaproj/aqua-registry/releases/latest":
			n := calls.releases.Add(1)
			if n == 1 {
				close(started)
			}
			if n == int32(callers) {
				close(allStarted)
			}
			select {
			case <-release:
			case <-r.Context().Done():
				return
			}
			if fail.Load() {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			_, _ = w.Write([]byte(`{"tag_name":"v4.999.0"}`))
		case "/repos/aquaproj/aqua-registry/commits/v4.999.0":
			calls.commits.Add(1)
			_, _ = w.Write([]byte(testLatestSHA))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(func() {
		unblock()
		server.Close()
	})
	finish := func() {
		select {
		case <-allStarted:
		case <-time.After(250 * time.Millisecond):
		}
		unblock()
	}
	return &registryTestServer{url: server.URL, calls: &calls, fail: &fail, started: started, finish: finish}
}

func TestRegistryRefConcurrent(t *testing.T) {
	t.Parallel()
	const callers = 8
	server := blockedRegistryServer(t, callers)
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	results := make(chan resolvedRegistryRef, callers)
	for range callers {
		go func() {
			installer := New()
			installer.githubAPIBase = server.url
			results <- installer.resolveStandardRegistryRef(ctx, opts, false)
		}()
	}
	select {
	case <-server.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	server.finish()
	for range callers {
		assert.Equal(t, testLatestSHA, (<-results).SHA)
	}
	assert.EqualValues(t, 1, server.calls.releases.Load(), "cold callers must share the release lookup")
	assert.EqualValues(t, 1, server.calls.commits.Load(), "cold callers must share the commit lookup")
}

func TestRegistryRefFailedConcurrent(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		stale        bool
		forceRefresh bool
	}{
		{},
		{stale: true},
		{forceRefresh: true},
		{stale: true, forceRefresh: true},
	} {
		t.Run(fmt.Sprintf("stale=%t/force=%t", tc.stale, tc.forceRefresh), func(t *testing.T) {
			const callers = 8
			server := blockedRegistryServer(t, callers)
			server.fail.Store(true)
			opts := tools.InstallOptions{ToolsDir: t.TempDir()}
			installer := New()
			installer.githubAPIBase = server.url
			wantSHA := ir.DefaultAquaStandardRegistryRef
			wantSource := registryRefSourceBootstrap
			if tc.stale {
				installer.writeLatestRefCache(installer.latestRefCachePath(opts), latestRegistryRef{
					Tag: "v4.999.0", SHA: testLatestSHA, FetchedAt: time.Now().Add(-48 * time.Hour),
				})
				wantSHA, wantSource = testLatestSHA, registryRefSourceStaleCache
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			results := make(chan resolvedRegistryRef, callers)
			started := make(chan struct{}, callers)
			start := make(chan struct{})
			for range callers {
				go func() {
					caller := New()
					caller.githubAPIBase = server.url
					var first sync.Once
					caller.now = func() time.Time {
						now := time.Now()
						first.Do(func() {
							started <- struct{}{}
							<-start
						})
						return now
					}
					results <- caller.resolveStandardRegistryRef(ctx, opts, tc.forceRefresh)
				}()
			}
			for range callers {
				select {
				case <-started:
				case <-ctx.Done():
					close(start)
					t.Fatal(ctx.Err())
				}
			}
			close(start)
			select {
			case <-server.started:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			server.finish()
			for range callers {
				resolved := <-results
				assert.Equal(t, wantSHA, resolved.SHA)
				assert.Equal(t, wantSource, resolved.Source)
			}
			assert.EqualValues(t, 1, server.calls.releases.Load(), "waiters must share a failed lookup too")
			assert.EqualValues(t, 0, server.calls.commits.Load())
			_, fresh := readLatestRefCache(installer.latestRefCachePath(opts), time.Now())
			assert.False(t, fresh, "a failed lookup must not make the registry cache fresh")

			server.fail.Store(false)
			resolved := installer.resolveStandardRegistryRef(ctx, opts, tc.forceRefresh)
			assert.Equal(t, registryRefSourceLive, resolved.Source, "a later independent caller must be able to retry")
			assert.Equal(t, testLatestSHA, resolved.SHA)
			assert.EqualValues(t, 2, server.calls.releases.Load())
			assert.EqualValues(t, 1, server.calls.commits.Load())
		})
	}
}

func TestRegistryRefProcesses(t *testing.T) {
	t.Parallel()
	server := blockedRegistryServer(t, 4)
	resolveRegistryProcesses(t, server, testLatestSHA)
	assert.EqualValues(t, 1, server.calls.releases.Load(), "processes sharing a tools directory must resolve once")
	assert.EqualValues(t, 1, server.calls.commits.Load())
}

func TestRegistryRefFailedProcesses(t *testing.T) {
	t.Parallel()
	server := blockedRegistryServer(t, 4)
	server.fail.Store(true)
	resolveRegistryProcesses(t, server, ir.DefaultAquaStandardRegistryRef)
	assert.EqualValues(t, 1, server.calls.releases.Load(), "waiting processes must share a failed lookup")
	assert.EqualValues(t, 0, server.calls.commits.Load())
}

func resolveRegistryProcesses(t *testing.T, server *registryTestServer, expectedSHA string) {
	t.Helper()
	const callers = 4
	dir := t.TempDir()
	start := filepath.Join(dir, "start")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	results := make(chan error, callers)
	for n := range callers {
		ready := filepath.Join(dir, fmt.Sprintf("ready-%d", n))
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRegistryRefProcessHelper$", "--", server.url, dir, ready, start)
		cmd.Env = append(os.Environ(), "DAGU_TEST_REGISTRY_HELPER=1", "DAGU_TEST_REGISTRY_EXPECTED_SHA="+expectedSHA, "GITHUB_TOKEN=", "GH_TOKEN=")
		go func() {
			output, err := cmd.CombinedOutput()
			if err != nil {
				err = fmt.Errorf("registry helper: %w: %s", err, output)
			}
			results <- err
		}()
	}
	require.Eventually(t, func() bool {
		for n := range callers {
			if _, err := os.Stat(filepath.Join(dir, fmt.Sprintf("ready-%d", n))); err != nil {
				return false
			}
		}
		return true
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, os.WriteFile(start, nil, 0o600))
	select {
	case <-server.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	server.finish()
	for range callers {
		require.NoError(t, <-results)
	}
}

func TestRegistryRefWaitCanceled(t *testing.T) {
	t.Parallel()
	server := blockedRegistryServer(t, 2)
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	installer := New()
	installer.githubAPIBase = server.url
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	first := make(chan resolvedRegistryRef, 1)
	go func() { first <- installer.resolveStandardRegistryRef(ctx, opts, false) }()
	select {
	case <-server.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	waiter := New()
	waiter.githubAPIBase = server.url
	waitCtx, stopWaiting := context.WithTimeout(ctx, 50*time.Millisecond)
	defer stopWaiting()
	resolved := waiter.resolveStandardRegistryRef(waitCtx, opts, false)
	require.ErrorIs(t, waitCtx.Err(), context.DeadlineExceeded)
	assert.Equal(t, registryRefSourceBootstrap, resolved.Source)
	assert.EqualValues(t, 1, server.calls.releases.Load(), "a canceled waiter must not start another fetch")
	server.finish()
	assert.Equal(t, testLatestSHA, (<-first).SHA, "canceling a waiter must not cancel the owner")
	assert.Equal(t, registryRefSourceCache, waiter.resolveStandardRegistryRef(ctx, opts, false).Source)
}

func TestRegistryRefCacheUnavailable(t *testing.T) {
	t.Parallel()
	calls := 0
	server := newLatestRefServer(t, &calls)
	installer := New()
	installer.githubAPIBase = server.URL
	blocked := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(blocked, nil, 0o600))
	resolved := installer.resolveStandardRegistryRef(context.Background(), tools.InstallOptions{ToolsDir: blocked}, false)
	assert.Equal(t, registryRefSourceLive, resolved.Source)
	assert.Equal(t, testLatestSHA, resolved.SHA)
	assert.Equal(t, 2, calls, "cache failure must not disable live resolution")
}

func TestRegistryRefCacheWriteUnavailable(t *testing.T) {
	t.Parallel()
	const (
		callers        = 4
		resolveTimeout = 30 * time.Second
	)
	server := blockedRegistryServer(t, callers)
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	require.NoError(t, os.MkdirAll(New().latestRefCachePath(opts), 0o750))
	// Windows retries each failed replacement while holding the cache lock.
	// Allow every caller to exhaust those retries before the test deadline.
	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()
	results := make(chan resolvedRegistryRef, callers)
	for range callers {
		go func() {
			installer := New()
			installer.githubAPIBase = server.url
			results <- installer.resolveStandardRegistryRef(ctx, opts, false)
		}()
	}
	select {
	case <-server.started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	server.finish()
	for range callers {
		resolved := <-results
		assert.Equal(t, registryRefSourceLive, resolved.Source)
		assert.Equal(t, testLatestSHA, resolved.SHA)
	}
	assert.EqualValues(t, callers, server.calls.releases.Load(), "cache write failure must not become a registry failure")
	assert.EqualValues(t, callers, server.calls.commits.Load())
}

func TestRegistryRefFutureFailure(t *testing.T) {
	t.Parallel()
	calls := 0
	server := newLatestRefServer(t, &calls)
	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	installer.writeLatestRefCacheEntry(installer.latestRefCachePath(opts), latestRefCacheEntry{
		FailedAt: time.Now().Add(time.Hour),
	})
	resolved := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceLive, resolved.Source)
	assert.Equal(t, testLatestSHA, resolved.SHA)
	assert.Equal(t, 2, calls, "a clock change must not suppress independent retries")
}

func TestRegistryRefLockUnavailable(t *testing.T) {
	t.Parallel()
	calls := 0
	server := newLatestRefServer(t, &calls)
	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	cachePath := installer.latestRefCachePath(opts)
	require.NoError(t, os.MkdirAll(cachePath+".lock", 0o750))
	resolved := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceLive, resolved.Source)
	assert.Equal(t, testLatestSHA, resolved.SHA)
	assert.Equal(t, 2, calls)
	cached, ok := readLatestRefCache(cachePath, time.Now())
	assert.True(t, ok, "a failed lock must not disable a writable cache")
	assert.Equal(t, testLatestSHA, cached.SHA)
}

func TestRegistryRefFailureWithoutLock(t *testing.T) {
	t.Parallel()
	server := newFailingRefServer(t)
	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}
	cachePath := installer.latestRefCachePath(opts)
	installer.writeLatestRefCache(cachePath, latestRegistryRef{
		Tag: "v4.999.0", SHA: testLatestSHA, FetchedAt: time.Now().Add(-48 * time.Hour),
	})
	before, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	// A directory can itself be flocked on Unix. A symlink loop makes opening
	// the lock fail without making the registry cache unwritable.
	if err := os.Symlink(cachePath+".lock", cachePath+".lock"); err != nil {
		t.Skipf("cannot create a symlink loop on this platform: %v", err)
	}
	unlock, lockErr := installer.lockRegistryRef(context.Background(), cachePath)
	if unlock != nil {
		unlock()
	}
	require.Error(t, lockErr)
	resolved := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceStaleCache, resolved.Source)
	after, err := os.ReadFile(cachePath)
	require.NoError(t, err)
	assert.Equal(t, before, after, "an unlocked failure must not overwrite another caller's cache")
}

func TestRegistryRefOwnerExit(t *testing.T) {
	t.Parallel()
	calls := 0
	server := newLatestRefServer(t, &calls)
	dir := t.TempDir()
	ready := filepath.Join(dir, "ready")
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRegistryRefProcessHelper$", "--", "hold-lock", dir, ready, "unused")
	cmd.Env = append(os.Environ(), "DAGU_TEST_REGISTRY_HELPER=1", "GITHUB_TOKEN=", "GH_TOKEN=")
	require.NoError(t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	require.Eventually(t, func() bool {
		_, err := os.Stat(ready)
		return err == nil
	}, 10*time.Second, 10*time.Millisecond)
	require.NoError(t, cmd.Process.Kill())
	require.Error(t, <-done)
	installer := New()
	installer.githubAPIBase = server.URL
	resolveCtx, stop := context.WithTimeout(ctx, 2*time.Second)
	defer stop()
	resolved := installer.resolveStandardRegistryRef(resolveCtx, tools.InstallOptions{ToolsDir: dir}, false)
	assert.Equal(t, registryRefSourceLive, resolved.Source)
	assert.Equal(t, testLatestSHA, resolved.SHA)
	assert.Equal(t, 2, calls, "the next process must recover without a stale-lock delay")
}

func TestRegistryRefProcessHelper(t *testing.T) {
	if os.Getenv("DAGU_TEST_REGISTRY_HELPER") != "1" {
		return
	}
	args := os.Args[len(os.Args)-4:]
	installer := New()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	opts := tools.InstallOptions{ToolsDir: args[1]}
	if args[0] == "hold-lock" {
		unlock, err := installer.lockRegistryRef(ctx, installer.latestRefCachePath(opts))
		require.NoError(t, err)
		defer unlock()
		require.NoError(t, os.WriteFile(args[2], nil, 0o600))
		<-ctx.Done()
		return
	}
	var first sync.Once
	installer.now = func() time.Time {
		now := time.Now()
		first.Do(func() {
			require.NoError(t, os.WriteFile(args[2], nil, 0o600))
			require.Eventually(t, func() bool {
				_, err := os.Stat(args[3])
				return err == nil
			}, 10*time.Second, 10*time.Millisecond)
		})
		return now
	}
	installer.githubAPIBase = args[0]
	resolved := installer.resolveStandardRegistryRef(ctx, opts, false)
	expectedSHA := os.Getenv("DAGU_TEST_REGISTRY_EXPECTED_SHA")
	if expectedSHA == "" {
		expectedSHA = testLatestSHA
	}
	require.Equal(t, expectedSHA, resolved.SHA)
}

func newLatestRefServer(t *testing.T, calls *int) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*calls++
		switch r.URL.Path {
		case "/repos/aquaproj/aqua-registry/releases/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v4.999.0"}`))
		case "/repos/aquaproj/aqua-registry/commits/v4.999.0":
			assert.Equal(t, "application/vnd.github.sha", r.Header.Get("Accept"))
			_, _ = w.Write([]byte(testLatestSHA))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newFailingRefServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestResolveStandardRegistryRefResolvesAndCaches(t *testing.T) {
	t.Parallel()

	calls := 0
	server := newLatestRefServer(t, &calls)
	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)

	installer := New()
	installer.githubAPIBase = server.URL
	installer.now = func() time.Time { return base }
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}

	resolved := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceLive, resolved.Source)
	assert.Equal(t, "v4.999.0", resolved.Tag)
	assert.Equal(t, testLatestSHA, resolved.SHA)
	callsAfterFirst := calls
	require.Positive(t, callsAfterFirst)

	cached := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceCache, cached.Source)
	assert.Equal(t, resolved.SHA, cached.SHA)
	assert.Equal(t, callsAfterFirst, calls)

	installer.now = func() time.Time { return base.Add(latestRefCacheTTL + time.Hour) }
	expired := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceLive, expired.Source)
	assert.Greater(t, calls, callsAfterFirst)
}

func TestResolveStandardRegistryRefForceRefreshSkipsFreshCache(t *testing.T) {
	t.Parallel()

	calls := 0
	server := newLatestRefServer(t, &calls)

	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}

	first := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	require.Equal(t, registryRefSourceLive, first.Source)
	callsAfterFirst := calls

	refreshed := installer.resolveStandardRegistryRef(context.Background(), opts, true)
	assert.Equal(t, registryRefSourceLive, refreshed.Source)
	assert.Greater(t, calls, callsAfterFirst)
}

func TestResolveStandardRegistryRefFallsBackToStaleCache(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	opts := tools.InstallOptions{ToolsDir: t.TempDir()}

	stale := New()
	stale.now = func() time.Time { return base }
	stale.writeLatestRefCache(stale.latestRefCachePath(opts), latestRegistryRef{
		Tag:       "v4.900.0",
		SHA:       testLatestSHA,
		FetchedAt: base.Add(-48 * time.Hour),
	})

	server := newFailingRefServer(t)
	installer := New()
	installer.githubAPIBase = server.URL
	installer.now = func() time.Time { return base }

	resolved := installer.resolveStandardRegistryRef(context.Background(), opts, false)
	assert.Equal(t, registryRefSourceStaleCache, resolved.Source)
	assert.Equal(t, "v4.900.0", resolved.Tag)
	assert.Equal(t, testLatestSHA, resolved.SHA)
}

func TestResolveStandardRegistryRefFallsBackToBootstrap(t *testing.T) {
	t.Parallel()

	server := newFailingRefServer(t)
	installer := New()
	installer.githubAPIBase = server.URL

	resolved := installer.resolveStandardRegistryRef(context.Background(), tools.InstallOptions{ToolsDir: t.TempDir()}, false)
	assert.Equal(t, registryRefSourceBootstrap, resolved.Source)
	assert.Equal(t, ir.DefaultAquaStandardRegistryRef, resolved.SHA)
	assert.Empty(t, resolved.Tag)
}

func seedFreshRefCache(t *testing.T, installer *Installer, opts tools.InstallOptions) {
	t.Helper()
	installer.writeLatestRefCache(installer.latestRefCachePath(opts), latestRegistryRef{
		Tag:       "v4.999.0",
		SHA:       testLatestSHA,
		FetchedAt: installer.now(),
	})
}

func TestInstallDoesNotRefreshRegistryOnLocalFailure(t *testing.T) {
	t.Parallel()

	calls := 0
	server := newLatestRefServer(t, &calls)

	// A file where the env tree lives makes the install fail locally before
	// any registry or package work starts, while the ref cache stays writable.
	toolsDir := filepath.Join(t.TempDir(), "tools")
	require.NoError(t, os.MkdirAll(filepath.Join(toolsDir, "aqua"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(toolsDir, "aqua", "envs"), []byte("not a dir"), 0o600))

	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: toolsDir}
	seedFreshRefCache(t, installer, opts)
	callsAfterSeed := calls

	_, err := installer.Install(context.Background(), &ir.ToolConfig{
		Packages: []ir.ToolPackage{{Package: "jqlang/jq", Version: "jq-1.7.1"}},
	}, opts)

	require.Error(t, err)
	assert.False(t, isRegistryResolutionError(err))
	assert.Equal(t, callsAfterSeed, calls, "a local failure must not trigger a registry refresh")
}

func TestInstallDoesNotRefreshRegistryOnCanceledContext(t *testing.T) {
	t.Parallel()

	calls := 0
	server := newLatestRefServer(t, &calls)

	installer := New()
	installer.githubAPIBase = server.URL
	opts := tools.InstallOptions{ToolsDir: filepath.Join(t.TempDir(), "tools")}
	seedFreshRefCache(t, installer, opts)
	callsAfterSeed := calls

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := installer.Install(ctx, &ir.ToolConfig{
		Packages: []ir.ToolPackage{{Package: "jqlang/jq", Version: "jq-1.7.1"}},
	}, opts)

	require.Error(t, err)
	assert.Equal(t, callsAfterSeed, calls, "a canceled context must not trigger a registry refresh")
}

func TestReadLatestRefCacheRejectsStaleAndInvalid(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
	fresh := latestRegistryRef{Tag: "v4.999.0", SHA: testLatestSHA, FetchedAt: now.Add(-time.Hour)}
	stale := latestRegistryRef{Tag: "v4.999.0", SHA: testLatestSHA, FetchedAt: now.Add(-latestRefCacheTTL - time.Hour)}
	future := latestRegistryRef{Tag: "v4.999.0", SHA: testLatestSHA, FetchedAt: now.Add(time.Hour)}
	badSHA := latestRegistryRef{Tag: "v4.999.0", SHA: "not-a-sha", FetchedAt: now.Add(-time.Hour)}

	installer := New()
	writeCase := func(t *testing.T, ref latestRegistryRef) string {
		t.Helper()
		path := t.TempDir() + "/cache.json"
		installer.writeLatestRefCache(path, ref)
		return path
	}

	if cached, ok := readLatestRefCache(writeCase(t, fresh), now); assert.True(t, ok) {
		assert.Equal(t, fresh.SHA, cached.SHA)
	}
	_, ok := readLatestRefCache(writeCase(t, stale), now)
	assert.False(t, ok)
	if cached, ok := readLatestRefCacheAnyAge(writeCase(t, stale)); assert.True(t, ok) {
		assert.Equal(t, stale.SHA, cached.SHA)
	}
	_, ok = readLatestRefCache(writeCase(t, future), now)
	assert.False(t, ok)
	_, ok = readLatestRefCache(writeCase(t, badSHA), now)
	assert.False(t, ok)
	_, ok = readLatestRefCacheAnyAge(writeCase(t, badSHA))
	assert.False(t, ok)
	_, ok = readLatestRefCache("", now)
	assert.False(t, ok)
}

func TestIsCommitSHA(t *testing.T) {
	t.Parallel()

	assert.True(t, isCommitSHA(testLatestSHA))
	assert.True(t, isCommitSHA("080d723b75cd0ea7c2b2059bf6266d3ab39aa792"))
	assert.False(t, isCommitSHA("080D723B75CD0EA7C2B2059BF6266D3AB39AA792"))
	assert.False(t, isCommitSHA("v4.999.0"))
	assert.False(t, isCommitSHA(""))
	assert.False(t, isCommitSHA(testLatestSHA+"aa"))
}
