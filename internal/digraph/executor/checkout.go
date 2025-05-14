package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/dagu-org/dagu/internal/digraph"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/filesystem"
	"github.com/go-viper/mapstructure/v2"
)

const (
	gitCheckOutExecutorType = "git-checkout"
	defaultSSHUser          = "git"
	fileProtocol            = "file"
	httpProtocol            = "http"
	httpsProtocol           = "https"
)

var _ Executor = (*gitCheckout)(nil)

func init() {
	Register(gitCheckOutExecutorType, newCheckout)
}

type refType string

const (
	refTypeEmpty  refType = ""
	refTypeBranch refType = "branch"
	refTypeTag    refType = "tag"
	refTypeCommit refType = "commit"
	refTypeRefs   refType = "refs"
)

type gitCheckoutExecConfigDefinition struct {
	Repo     string
	Ref      string
	Path     string
	Depth    int
	Progress bool
	Cache    bool
	Auth     gitCheckoutExecAuthConfigDefinition
}

func (g *gitCheckoutExecConfigDefinition) getRepoCachePath() string {
	var (
		homeDir string
		err     error
	)

	if homeDir, err = os.UserHomeDir(); err != nil {
		if os.PathSeparator == '\\' {
			homeDir = "C:\\Users\\Default"
		} else {
			homeDir = "/home/default"
		}
	}

	// https://github.com/dagu-org/dagu.git -> github.com/dagu-org/dagu.git
	// http://github.com/dagu-org/dagu.git -> github.com/dagu-org/dagu.git
	// git@github.com:dagu-org/dagu.git -> github.com/dagu-org/dagu.git
	// file://github.com/dagu-org/dagu.git -> github.com/dagu-org/dagu.git
	re := regexp.MustCompile(`^(https?://|git@|file:////)`)
	cleaned := re.ReplaceAllString(g.Repo, "")
	cleaned = strings.ReplaceAll(cleaned, ":", "/")
	cacheDir := filepath.Join(homeDir, ".cache", "dagu", "git")

	return filepath.Join(cacheDir, cleaned)
}

func (g *gitCheckoutExecConfigDefinition) getRefType() (refType, error) {
	var (
		tagRegex  = regexp.MustCompile(`^v(\d+\.)+\d+$`)
		hashRegex = regexp.MustCompile(`^[0-9a-f]{40}$`)
	)

	if g.Ref == "" {
		return refTypeEmpty, errors.New("ref is required")
	}

	if strings.HasPrefix(g.Ref, "refs") {
		return refTypeRefs, nil
	}

	if tagRegex.MatchString(g.Ref) {
		return refTypeTag, nil
	}

	if hashRegex.MatchString(g.Ref) {
		return refTypeCommit, nil
	}

	return refTypeBranch, nil
}

type gitCheckoutExecAuthConfigDefinition struct {
	TokenEnv       string
	UserName       string
	Password       string
	SSHUser        string
	SSHKey         string
	SSHKeyPassword string
	SSHAgent       bool
}

// httpAuthMethod returns http auth method, if auth config is not set, return nil
func (g *gitCheckoutExecAuthConfigDefinition) httpAuthMethod() (transport.AuthMethod, error) {
	if g.TokenEnv != "" {
		g.UserName = g.SSHUser
		g.Password = os.Getenv(g.TokenEnv)
	}

	return &githttp.BasicAuth{
		Username: g.UserName,
		Password: g.Password,
	}, nil
}

// sshAuthMethod returns ssh auth method, if auth config is not set, return nil
func (g *gitCheckoutExecAuthConfigDefinition) sshAuthMethod() (transport.AuthMethod, error) {
	var (
		authMethod transport.AuthMethod
		publicKey  *ssh.PublicKeys
		err        error
	)

	if g.SSHAgent {
		if authMethod, err = ssh.NewSSHAgentAuth(g.SSHUser); err != nil {
			return nil, fmt.Errorf("failed to create ssh agent auth: %w", err)
		}

		return authMethod, nil
	}

	if _, err = os.Stat(g.SSHKey); err != nil {
		return nil, fmt.Errorf("failed to find ssh key file: %w", err)
	}

	if publicKey, err = ssh.NewPublicKeysFromFile(g.SSHUser, g.SSHKey, g.SSHKeyPassword); err != nil {
		return nil, fmt.Errorf("failed to create ssh public keys: %w", err)
	}

	return publicKey, nil
}

func (g *gitCheckoutExecConfigDefinition) authMethod() (transport.AuthMethod, error) {
	var (
		endpoint *transport.Endpoint
		err      error
	)

	if endpoint, err = transport.NewEndpoint(g.Repo); err != nil {
		return nil, fmt.Errorf("failed to create endpoint: %w", err)
	}

	if len(g.Auth.SSHUser) == 0 {
		g.Auth.SSHUser = defaultSSHUser
	}

	if endpoint.Protocol == fileProtocol {
		return nil, nil
	}

	if endpoint.Protocol == httpProtocol || endpoint.Protocol == httpsProtocol {
		return g.Auth.httpAuthMethod()
	}

	return g.Auth.sshAuthMethod()
}

type gitCheckoutExecConfig struct {
	repo          string
	ref           string
	refType       refType
	path          string
	depth         int
	progress      bool
	cache         bool
	repoCachePath string
}

type gitCheckout struct {
	stdout     io.Writer
	stderr     io.Writer
	authMethod transport.AuthMethod
	config     *gitCheckoutExecConfig
}

func convertFromDef(def *gitCheckoutExecConfigDefinition, authMethod transport.AuthMethod, executorRefType refType) *gitCheckout {
	return &gitCheckout{
		stdout: os.Stdout,
		stderr: os.Stderr,
		config: &gitCheckoutExecConfig{
			repo:          def.Repo,
			ref:           def.Ref,
			path:          def.Path,
			depth:         def.Depth,
			progress:      def.Progress,
			cache:         def.Cache,
			repoCachePath: def.getRepoCachePath(),
			refType:       executorRefType,
		},
		authMethod: authMethod,
	}
}

func newCheckout(_ context.Context, step digraph.Step) (Executor, error) {
	var (
		def             = &gitCheckoutExecConfigDefinition{}
		authMethod      transport.AuthMethod
		executorRefType refType
		err             error
	)

	if err = decodeGitCheckoutConfig(step.ExecutorConfig.Config, def); err != nil {
		return nil, fmt.Errorf("failed to decode git checkout config: %w", err)
	}

	if authMethod, err = def.authMethod(); err != nil {
		return nil, err
	}

	if executorRefType, err = def.getRefType(); err != nil {
		return nil, fmt.Errorf("failed to parse ref type: %w", err)
	}

	return convertFromDef(def, authMethod, executorRefType), nil
}

func decodeGitCheckoutConfig(data map[string]any, config *gitCheckoutExecConfigDefinition) error {
	var (
		mapDecoder   *mapstructure.Decoder
		decodeConfig = &mapstructure.DecoderConfig{
			Result:           config,
			WeaklyTypedInput: true,
		}
		err error
	)

	if mapDecoder, err = mapstructure.NewDecoder(decodeConfig); err != nil {
		return fmt.Errorf("failed to create map decoder: %w", err)
	}

	if err = mapDecoder.Decode(data); err != nil {
		return fmt.Errorf("failed to decode git checkout config: %w", err)
	}

	return nil
}

func (g *gitCheckout) SetStdout(out io.Writer) {
	g.stdout = out
}

func (g *gitCheckout) SetStderr(out io.Writer) {
	g.stderr = out
}

func (g *gitCheckout) Kill(_ os.Signal) error {
	return nil
}

func (g *gitCheckout) rmWorkPathIfExists() error {
	var (
		err error
	)

	if err = os.RemoveAll(g.config.path); err != nil {
		return fmt.Errorf("failed to remove %s: %w", g.config.path, err)
	}

	return nil
}

// getFetchOptions returns the fetch options
// ref may be in the form of
// branch : "refs/heads/main" or "main"
// tag : "refs/tags/v1.0" or "v1.0"
// commit : "refs/commits/abc123" or "abc123"
func (g *gitCheckout) getFetchOptions() (*git.FetchOptions, error) {
	var (
		fetchOptions = &git.FetchOptions{
			RemoteName: git.DefaultRemoteName,
			Auth:       g.authMethod,
			Depth:      g.config.depth,
			RefSpecs: []config.RefSpec{
				config.RefSpec(fmt.Sprintf("+refs/heads/%s:refs/remotes/origin/%s", g.config.ref, g.config.ref)),
			},
			Force: true,
		}
	)

	if g.config.progress {
		fetchOptions.Progress = g.stdout
	}

	if g.config.refType == refTypeRefs {
		fetchOptions.RefSpecs = []config.RefSpec{config.RefSpec(fmt.Sprintf("+%s:%s", g.config.ref, g.config.ref))}
	}

	if g.config.refType == refTypeTag {
		fetchOptions.RefSpecs = []config.RefSpec{config.RefSpec(fmt.Sprintf("+refs/tags/%s:refs/tags/%s", g.config.ref, g.config.ref))}
	}

	if g.config.refType == refTypeCommit {
		hash := plumbing.NewHash(g.config.ref)
		fetchOptions.RefSpecs = []config.RefSpec{config.RefSpec(fmt.Sprintf("+%[1]s:%[1]s", hash, hash))}
		fetchOptions.Tags = git.NoTags
	}

	return fetchOptions, nil
}

// initRepo initializes the git repository
func (g *gitCheckout) initRepo() (*git.Repository, error) {
	var (
		repo *git.Repository
		err  error
	)

	if repo, err = git.PlainInit(g.config.path, false); err != nil {
		return nil, fmt.Errorf("failed to init git repository: %w", err)
	}

	if _, err = repo.CreateRemote(&config.RemoteConfig{
		Name: git.DefaultRemoteName,
		URLs: []string{g.config.repo},
	}); err != nil {
		return nil, fmt.Errorf("failed to create remote repository: %w", err)
	}

	return repo, nil
}

func (g *gitCheckout) setRepoAlternate(repo *git.Repository) error {
	var (
		err error
	)

	storage, ok := repo.Storer.(*filesystem.Storage)
	if !ok {
		return fmt.Errorf("unexpected storage type")
	}

	if err = storage.AddAlternate(g.config.repoCachePath); err != nil {
		return fmt.Errorf("failed to add alternate: %w", err)
	}

	return nil
}

func (g *gitCheckout) getRepo() (*git.Repository, error) {
	var (
		repo *git.Repository
		err  error
	)

	if repo, err = g.initRepo(); err != nil {
		return nil, err
	}

	if !g.config.cache {
		return repo, nil
	}

	if err = g.setRepoAlternate(repo); err != nil {
		return nil, err
	}

	return repo, nil
}

func (g *gitCheckout) fetch(ctx context.Context, repo *git.Repository) error {
	var (
		fetchOptions *git.FetchOptions
		err          error
	)

	if fetchOptions, err = g.getFetchOptions(); err != nil {
		return fmt.Errorf("failed to get fetch options: %w", err)
	}

	if err = repo.FetchContext(ctx, fetchOptions); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return fmt.Errorf("failed to fetch repository: %w", err)
	}

	return nil
}

func (g *gitCheckout) getCheckoutOptions(repo *git.Repository) (*git.CheckoutOptions, error) {
	var (
		isHash          = g.config.refType == refTypeCommit
		isTag           = g.config.refType == refTypeTag
		isRefs          = g.config.refType == refTypeRefs
		checkoutOptions = &git.CheckoutOptions{}
		revHash         *plumbing.Hash
		err             error
	)

	if !isHash && !isTag && !isRefs {
		checkoutOptions.Force = true

		localBranchReferenceName := plumbing.NewBranchReferenceName(g.config.ref)
		remoteReferenceName := plumbing.NewRemoteReferenceName(git.DefaultRemoteName, g.config.ref)
		newReference := plumbing.NewSymbolicReference(localBranchReferenceName, remoteReferenceName)

		if err = repo.Storer.SetReference(newReference); err != nil {
			return nil, fmt.Errorf("failed to set reference: %w", err)
		}
		checkoutOptions.Branch = localBranchReferenceName

		return checkoutOptions, nil
	}

	if revHash, err = repo.ResolveRevision(plumbing.Revision(g.config.ref)); err != nil {
		return nil, fmt.Errorf("failed to resolve revision: %w", err)
	}

	checkoutOptions.Hash = plumbing.NewHash(revHash.String())

	return checkoutOptions, nil
}

func (g *gitCheckout) checkout(repo *git.Repository) error {
	var (
		worktree       *git.Worktree
		checkoutOption *git.CheckoutOptions
		err            error
	)

	if worktree, err = repo.Worktree(); err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	if checkoutOption, err = g.getCheckoutOptions(repo); err != nil {
		return err
	}

	if err = worktree.Checkout(checkoutOption); err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}

	return nil
}

func (g *gitCheckout) saveCache() error {
	var (
		workDir   = os.DirFS(g.config.path)
		isExisted bool
		err       error
	)

	if _, err = os.Stat(g.config.repoCachePath); err == nil {
		isExisted = true
	}

	if g.config.cache && !isExisted {
		if err = os.CopyFS(g.config.repoCachePath, workDir); err != nil {
			return fmt.Errorf("failed to copy git cache: %w", err)
		}
	}

	return nil
}

func (g *gitCheckout) Run(ctx context.Context) error {
	var (
		repo *git.Repository
		err  error
	)

	if err = g.rmWorkPathIfExists(); err != nil {
		return err
	}

	if repo, err = g.getRepo(); err != nil {
		return err
	}

	if err = g.fetch(ctx, repo); err != nil {
		return err
	}

	if err = g.checkout(repo); err != nil {
		return err
	}

	return g.saveCache()
}
