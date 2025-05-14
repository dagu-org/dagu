package executor

import (
	"context"
	"errors"
	"fmt"

	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTagRef    = "v1.0.0"
	testBranchRef = "master"
	testCommitRef = "e25aed53c630415498734a05f6d76011e32acdea"
)

var (
	testPath = os.TempDir() + "/test-git-checkout"
)

func getTestGitCheckout(repo string, ref string, cache bool) *gitCheckout {
	var (
		def = &gitCheckoutExecConfigDefinition{
			Ref:      ref,
			Repo:     repo,
			Path:     testPath,
			Depth:    1,
			Progress: true,
			Cache:    cache,
		}
		defAuthMethod transport.AuthMethod
		err           error
	)

	if defAuthMethod, err = def.authMethod(); err != nil {
		panic(err)
	}

	return convertFromDef(def, defAuthMethod)
}

type gitCheckoutCheckoutJudgeFunc func(path string) error

type TestGitCheckoutTestCase struct {
	msg   string
	ref   string
	cache bool
	judge gitCheckoutCheckoutJudgeFunc
}

func (t *TestGitCheckoutTestCase) cleanUp(executor *gitCheckout) error {
	var (
		err error
	)

	if err = os.RemoveAll(executor.config.path); err != nil {
		return err
	}

	if executor.config.cache {
		if err = os.RemoveAll(executor.config.repoCachePath); err != nil {
			return err
		}
	}

	return nil
}

// tagRefJudgeFunc is a judge function for tag reference
func tagRefJudgeFunc(path string) error {
	var (
		repo *git.Repository
		err  error
	)

	if repo, err = git.PlainOpen(path); err != nil {
		return err
	}

	// Check if the tag reference exists
	if _, err = repo.Tag(testTagRef); err != nil {
		return err
	}

	return nil
}

// branchRefJudgeFunc is a judge function for branch reference
func branchRefJudgeFunc(path string) error {
	var (
		repo *git.Repository
		err  error
	)

	if repo, err = git.PlainOpen(path); err != nil {
		return err
	}

	if _, err = repo.Reference(plumbing.NewBranchReferenceName(testBranchRef), false); err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			// Return a friendly error for this one, versus just ReferenceNotFound.
			return fmt.Errorf("branch %s not found", testBranchRef)
		}

		return err

	}

	return nil
}

// commitRefJudgeFunc is a judge function for commit reference
func commitRefJudgeFunc(path string) error {
	var (
		repo       *git.Repository
		commitHash = plumbing.NewHash(testCommitRef)
		err        error
	)

	if repo, err = git.PlainOpen(path); err != nil {
		return err
	}

	// Check if the commit reference exists
	if _, err = repo.CommitObject(commitHash); err != nil {
		return err
	}

	return nil
}

func getTestGitCheckoutTestCases() []TestGitCheckoutTestCase {
	return []TestGitCheckoutTestCase{
		{
			msg:   "tag reference",
			ref:   testTagRef,
			cache: false,
			judge: tagRefJudgeFunc,
		},
		{
			msg:   "branch reference",
			ref:   testBranchRef,
			cache: false,
			judge: branchRefJudgeFunc,
		},
		{
			msg:   "commit reference",
			ref:   testCommitRef,
			cache: false,
			judge: commitRefJudgeFunc,
		},
	}
}

func getFileRemotePath(rootPath string) (string, error) {
	var (
		err error
	)

	testRepoPath := filepath.Join(rootPath, "testdata", "test-repo.git")

	if _, err = os.Stat(testRepoPath); os.IsNotExist(err) {
		return "", err
	}

	return testRepoPath, nil
}

// getProjectRoot returns the root directory of the project.
func getProjectRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(1)
	require.True(t, ok, "failed to get caller information")
	rootDir := filepath.Join(filepath.Dir(filename), "..", "..")

	return filepath.Clean(rootDir)
}

// TestGitCheckout_Run is a test function for the GitCheckout executor. By git file protocol
func TestGitCheckout_Run(t *testing.T) {
	var (
		testCases = getTestGitCheckoutTestCases()
		rootPath  = getProjectRoot(t)
		repo      string
		err       error
	)

	if repo, err = getFileRemotePath(rootPath); err != nil {
		t.Fatalf("failed to get file remote path: %v", err)
	}

	repo = fmt.Sprintf("file:///%s", filepath.ToSlash(repo))

	for _, testCase := range testCases {
		t.Run(testCase.msg, func(t *testing.T) {
			var (
				testGitCheckout = getTestGitCheckout(repo, testCase.ref, testCase.cache)
				ctx             = context.Background()
				cacheIsExists   = true
			)

			assert.NoError(t, testGitCheckout.Run(ctx))
			assert.NoError(t, testCase.judge(testGitCheckout.config.path))

			if _, err = os.Stat(testGitCheckout.config.repoCachePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					cacheIsExists = false
				} else {
					t.Fatalf("failed to check cache path: %v", err)
				}
			}

			assert.Equal(t, testCase.cache, cacheIsExists)
			assert.NoError(t, testCase.cleanUp(testGitCheckout))
		})
	}
}

func getCacheTestCache() []*TestGitCheckoutTestCase {
	return []*TestGitCheckoutTestCase{
		{
			msg:   "first run with cache",
			ref:   testBranchRef,
			cache: true,
			judge: branchRefJudgeFunc,
		},
		{
			msg:   "second run with cache",
			ref:   testTagRef,
			cache: true,
			judge: tagRefJudgeFunc,
		},
	}
}

func TestGitCheckoutWithCache(t *testing.T) {
	var (
		testcaseList   = getCacheTestCache()
		rootPath       = getProjectRoot(t)
		firstRun       = testcaseList[0]
		secondRun      = testcaseList[1]
		firstExecutor  *gitCheckout
		secondExecutor *gitCheckout
		repo           string
		err            error
	)
	repo, err = getFileRemotePath(rootPath)
	assert.NoError(t, err)

	repo = fmt.Sprintf("file:///%s", filepath.ToSlash(repo))

	firstExecutor = getTestGitCheckout(repo, firstRun.ref, firstRun.cache)

	// first run
	assert.NoError(t, firstExecutor.Run(context.Background()))

	// check first run
	assert.NoError(t, firstRun.judge(firstExecutor.config.path))

	// check if the cache path exists
	_, err = os.Stat(firstExecutor.config.repoCachePath)
	assert.NoError(t, err)

	// check first run cache path
	assert.NoError(t, firstRun.judge(firstExecutor.config.repoCachePath))

	// second run
	secondExecutor = getTestGitCheckout(repo, secondRun.ref, secondRun.cache)
	assert.NoError(t, secondExecutor.Run(context.Background()))

	assert.NoError(t, secondRun.judge(secondExecutor.config.path))

	// check if the cache path exists
	_, err = os.Stat(secondExecutor.config.repoCachePath)
	assert.NoError(t, err)

	// check second run cache path
	assert.NoError(t, firstRun.judge(secondExecutor.config.repoCachePath))

	// clean up
	assert.NoError(t, firstRun.cleanUp(firstExecutor))
	assert.NoError(t, secondRun.cleanUp(secondExecutor))
}

type repoCachePathTestcase struct {
	msg        string
	repo       string
	expectRepo string
}

func getRepoCachePathTestcaseList() []*repoCachePathTestcase {
	var (
		err        error
		homeDir    string
		expectRepo string
	)

	if homeDir, err = os.UserHomeDir(); err != nil {
		if os.PathSeparator == '\\' {
			homeDir = "C:\\Users\\Default"
		} else {
			homeDir = "/home/default"
		}
	}

	expectRepo = filepath.Join(homeDir, ".cache", "dagu", "git", "github.com", "dagu", "dagu.git")

	return []*repoCachePathTestcase{
		{
			msg:        "https protocol",
			repo:       "https://github.com/dagu/dagu.git",
			expectRepo: expectRepo,
		},
		{
			msg:        "http protocol",
			repo:       "http://github.com/dagu/dagu.git",
			expectRepo: expectRepo,
		},
		{
			msg:        "ssh protocol",
			repo:       "git@github.com:dagu/dagu.git",
			expectRepo: expectRepo,
		},
		{
			msg:        "file protocol",
			repo:       "file:////github.com/dagu/dagu.git",
			expectRepo: expectRepo,
		},
	}
}

func TestGetRepoCachePath(t *testing.T) {
	var (
		testCases = getRepoCachePathTestcaseList()
	)

	for _, testCase := range testCases {
		t.Run(testCase.msg, func(t *testing.T) {
			var (
				def = &gitCheckoutExecConfigDefinition{
					Repo: testCase.repo,
				}
				result = def.getRepoCachePath()
			)

			if result != testCase.expectRepo {
				t.Fatalf("expected %s, got %s", testCase.expectRepo, result)
			}
		})
	}
}
