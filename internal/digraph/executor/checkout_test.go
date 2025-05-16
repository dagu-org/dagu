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
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testTagRef             = "v1.0.0"
	testBranchRef          = "master"
	testCommitRef          = "e25aed53c630415498734a05f6d76011e32acdea"
	testBranchStartWithRef = "refs/heads/master"
	testTagStartWithRef    = "refs/tags/v1.0.0"
)

var (
	testPath = "." + "/test-git-checkout"
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
		defRefType    refType
		err           error
	)

	if defAuthMethod, err = def.authMethod(); err != nil {
		panic(err)
	}

	if defRefType, err = def.getRefType(); err != nil {
		panic(err)
	}

	return convertFromDef(def, defAuthMethod, defRefType)
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

// startWithRefJudgeFunc is a judge function for branch reference start with refs
func startWithRefJudgeFunc(path string, ref string) error {
	var (
		repo *git.Repository
		err  error
	)

	if repo, err = git.PlainOpen(path); err != nil {
		return err
	}

	// Check if the branch reference exists
	if _, err = repo.Reference(plumbing.ReferenceName(ref), false); err != nil {
		return err
	}

	return nil
}

// branchStartWithRefJudgeFunc is a judge function for branch reference start with refs
func branchStartWithRefJudgeFunc(path string) error {
	return startWithRefJudgeFunc(path, testBranchStartWithRef)
}

// tagStartWithRefJudgeFunc is a judge function for tag reference start with refs
func tagStartWithRefJudgeFunc(path string) error {
	return startWithRefJudgeFunc(path, testTagStartWithRef)
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
		{
			msg:   "branch reference start with refs",
			ref:   testBranchStartWithRef,
			cache: false,
			judge: branchStartWithRefJudgeFunc,
		},
		{
			msg:   "tag reference start with refs",
			ref:   testTagStartWithRef,
			cache: false,
			judge: tagStartWithRefJudgeFunc,
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

type authMethodTestcase struct {
	msg              string
	def              *gitCheckoutExecConfigDefinition
	expectAuthName   string
	expectAuthMethod bool
	expectErr        bool
}

const testRSAPrivateKey = `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAovxthlB+eWsz4Vboz5wN1LzdXMroFsKZIJ70hNqKO0VK0eGP
9Ndigg+eAyHIFHdsJIIPQUUKLoSNkcR+PTDit7eXkIOf65kjMG5Pycj8z+eCE2Cz
xVmzLM6ZkKLYO/K8ROwJihKRGmdmPa6bGM1kWhXvrnPLK82KsJYUM3Ku7MLIedkw
5uKdWLaTniUgRaHoulOa7+1QTnZuK/KxW6U3Dhci43EOvEQ3ZJHpvio+jn1vC86R
PlkgaKUd+eOmSEDdGihs1CQvfQZMLVuiSUkFEkIacbA0QsQfZzZIJ0N6Q7oqjroO
gkw1sDE8pIaAJz7aNBi+kRaFMiFbgpsMrZltHQIDAQABAoIBACs131evuYg5Srzg
TMLV7bjMBagXR2bZWr2SRuN+CQ3jtg1kzsSr4br3pv3Pk/sRGkOnk6HLSwLAM8RE
ou9YKZNpgi5XJyvQIssxQ8gMmDIKf6rhhWe5+03SzFXTRp7GIPHo3jKT75JffXS2
+PmfYo6bqDrJCkFnsfBVKa/mJMgyA45NUoZc3iAW7KMJhJWDLuJ+ZHJj85r+iAxG
Aszmu9soXdMBaA5l8YYICoTCM2hTcbVAGrtCT69o/VOEgZSG5ykSrRxBFmwMhjJY
R8XcaeOgI5pfKLMb/CJ/qUSsAovNpc7BLzcVYBIY3AOMTtbGTnobxWjKCAYc+QB2
C0K5PSECgYEA1FKRWVE5T9lwP6hD2SMX74mPxEZbuxgzcedrs53raITmoMS1naDi
0ZqaTYOi4VYYgyojsP07ALow14QUMtBRLythl9z/CM5qtI61mkfJEkCNAxlRrxeQ
NmqND9jP591ymkPjFnhn3cp/Hx3SJdfodrxUkDPgHhkSaBLaPMGdkYUCgYEAxIOt
nHKeWSq/AH51zN+P/mGtWwi+RURf6EMVII7z66bJU+E+nbWGjrSBqHInLeq/zI/a
d2r9k9yXLvtUpK3PiOv3BgPqmov4KUPwr46DrgqJCKIhVGGTP79MNXU6DEA75nMj
BnlM5GgbE8+Dpt2XHEhdLyMB6K384rUPc14PdLkCgYEAnye9eHxgP7C4aZ9SLKQX
vyEYuYIcJOUBOzLEEwIfgluNHZoWobAGFiST4eL453zIJxohYvyPi/4FuqdxFJ3/
HSKhp1qreghxCCOpkZqZ6KqmiVojVuKM4Z2BXA2j2ySuUWDuCtv6z9CI9eQ+sMtl
oAuQQAAC0cztdUIcgUqJOJkCgYAOzcChXX0SSIcU+XHUWi8VwbP2fKUgwLLc41jP
GBXF9c2K1RgLd2ZIj86IqvjKm7mRJnEVt+icX+y/rE1HDpTowqXcPSVKOSsbqLOT
9g9zZ/XEwbnzClq2XanXCRqzW49nn9rOnQqu1izcBDDtvBmrFsR2TZPSPHElfvBI
B5jweQKBgDth1kPUitS9Lvcrc8YHklxizQ0J3zUTBJUAjwPU25RhtG7qia6lNCUr
Clcy8Xl3GK8Gv72Sv1VMJsg0mjYSFa2eMLdnoKlN3oIOZ4238V4p+ZgG22/R1Kmr
lyln/UHGhbpjOyuBlhqm26eURqB1jlOC39vwPsvrwAa273Mw/kQN
-----END RSA PRIVATE KEY-----
`

func getAuthMethodTestcaseList() ([]*authMethodTestcase, error) {
	var (
		err                   error
		tempRSAPrivateKeyFile *os.File
	)

	if tempRSAPrivateKeyFile, err = os.CreateTemp(".", "test_rsa"); err != nil {
		return nil, err
	}

	if _, err = tempRSAPrivateKeyFile.WriteString(testRSAPrivateKey); err != nil {
		return nil, err
	}

	return []*authMethodTestcase{
		{
			msg: "ssh protocol with ssh agent",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "git@github.com:dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					SSHAgent: true,
					SSHUser:  "dagu",
				},
			},
			expectAuthName:   ssh.PublicKeysCallbackName,
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "ssh protocol with ssh key file not exist",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "git@github.com:dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					SSHKey: "not_exist",
				},
			},
			expectAuthMethod: false,
			expectErr:        true,
		},
		{
			msg: "ssh protocol with ssh key file exist",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "git@github.com:dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					SSHKey: tempRSAPrivateKeyFile.Name(),
				},
			},
			expectAuthName:   ssh.PublicKeysName,
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "https protocol with username and password",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "https://github.com/dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					UserName: "dagu",
					Password: "dagu",
				},
			},
			expectAuthName:   "http-basic-auth",
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "https protocol with username and token",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "https://github.com/dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					UserName: "dagu",
					TokenEnv: "DAGU_TOKEN",
				},
			},
			expectAuthName:   "http-basic-auth",
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "http protocol with username and password",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "http://github.com/dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					UserName: "dagu",
					Password: "dagu",
				},
			},
			expectAuthName:   "http-basic-auth",
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "http protocol with username and token",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "http://github.com/dagu/dagu.git",
				Auth: gitCheckoutExecAuthConfigDefinition{
					UserName: "dagu",
					TokenEnv: "DAGU_TOKEN",
				},
			},
			expectAuthName:   "http-basic-auth",
			expectAuthMethod: true,
			expectErr:        false,
		},
		{
			msg: "file protocol",
			def: &gitCheckoutExecConfigDefinition{
				Repo: "file:///tmp/dagu",
			},
			expectAuthMethod: false,
			expectErr:        false,
		},
	}, nil
}

func TestAuthMethod(t *testing.T) {
	assert.NoError(t, os.Setenv("DAGU_TOKEN", "dagu"))
	defer func() {
		assert.NoError(t, os.Unsetenv("DAGU_TOKEN"))
	}()

	testCaseList, err := getAuthMethodTestcaseList()
	require.NoError(t, err)

	for _, testCase := range testCaseList {
		t.Run(testCase.msg, func(t *testing.T) {
			var (
				authMethod transport.AuthMethod
			)

			authMethod, err = testCase.def.authMethod()

			assert.Equal(t, testCase.expectAuthMethod, authMethod != nil)
			assert.Equal(t, testCase.expectErr, err != nil)
			if testCase.expectAuthMethod {
				assert.Equal(t, testCase.expectAuthName, authMethod.Name())
			}
		})
	}
}
