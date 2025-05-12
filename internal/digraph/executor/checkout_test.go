package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

const (
	testTagRef    = "v1.0.0"
	testBranchRef = "main"
	testCommitRef = "e2f9e8b0f3574ad7e12aa82c5b7c4c203fd0c4b0"
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

func getFileRemotePath() (string, error) {
	var (
		currentDir string
		err        error
	)

	if currentDir, err = os.Getwd(); err != nil {
		return "", err
	}

	// Get the current working directory,currentDir : "workspace/internal/digraph/executor"
	// return the path of "workspace/internal/testdata/testrepo.git"
	currentDir = currentDir[:len(currentDir)-len("/internal/digraph/executor")]

	return currentDir + "/internal/testdata/test-repo.git", nil
}

// TestGitCheckout_Run is a test function for the GitCheckout executor. By git file protocol
func TestGitCheckout_Run(t *testing.T) {
	var (
		testCases = getTestGitCheckoutTestCases()
		repo      string
		err       error
	)

	if repo, err = getFileRemotePath(); err != nil {
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

			if err = testGitCheckout.Run(ctx); err != nil {
				t.Fatalf("failed to run git checkout: %v", err)
			}

			if err = testCase.judge(testGitCheckout.config.path); err != nil {
				t.Fatalf("failed to judge git checkout: %v", err)
			}

			if _, err = os.Stat(testGitCheckout.config.repoCachePath); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					cacheIsExists = false
				} else {
					t.Fatalf("failed to check cache path: %v", err)
				}
			}

			if testCase.cache != cacheIsExists {
				t.Fatalf("cache path exists: %v, expected: %v", cacheIsExists, testCase.cache)
			}

			if err = testCase.cleanUp(testGitCheckout); err != nil {
				t.Fatalf("failed to clean up: %v", err)
			}
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
		firstRun       = testcaseList[0]
		secondRun      = testcaseList[1]
		firstExecutor  *gitCheckout
		secondExecutor *gitCheckout
		repo           string
		err            error
	)

	if repo, err = getFileRemotePath(); err != nil {
		t.Fatalf("failed to get file remote path: %v", err)
	}

	repo = fmt.Sprintf("file:///%s", filepath.ToSlash(repo))

	firstExecutor = getTestGitCheckout(repo, firstRun.ref, firstRun.cache)

	// first run
	if err = firstExecutor.Run(context.Background()); err != nil {
		t.Fatalf("failed to run git checkout: %v", err)
	}

	// check first run
	if err = firstRun.judge(firstExecutor.config.path); err != nil {
		t.Fatalf("failed to judge git checkout: %v", err)
	}

	// check if the cache path exists
	if _, err = os.Stat(firstExecutor.config.repoCachePath); err != nil {
		t.Fatalf("failed to check cache path: %v", err)
	}

	// check first run cache path
	if err = firstRun.judge(firstExecutor.config.repoCachePath); err != nil {
		t.Fatalf("failed to judge git checkout cache: %v", err)
	}

	// second run
	secondExecutor = getTestGitCheckout(repo, secondRun.ref, secondRun.cache)
	if err = secondExecutor.Run(context.Background()); err != nil {
		t.Fatalf("failed to run git checkout: %v", err)
	}

	if err = secondRun.judge(secondExecutor.config.path); err != nil {
		t.Fatalf("failed to judge git checkout: %v", err)
	}

	// check if the cache path exists
	if _, err = os.Stat(secondExecutor.config.repoCachePath); err != nil {
		t.Fatalf("failed to check cache path: %v", err)
	}

	// check second run cache path
	if err = firstRun.judge(secondExecutor.config.repoCachePath); err != nil {
		t.Fatalf("failed to judge git checkout cache: %v", err)
	}

	if err = firstRun.cleanUp(firstExecutor); err != nil {
		t.Fatalf("failed to clean up: %v", err)
	}

	if err = secondRun.cleanUp(secondExecutor); err != nil {
		t.Fatalf("failed to clean up: %v", err)
	}
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
