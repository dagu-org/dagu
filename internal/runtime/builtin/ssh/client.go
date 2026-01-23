package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/fileutil"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
	hostPort   string
	cfg        *ssh.ClientConfig
	Shell      string   // Shell for remote command execution
	ShellArgs  []string // Shell arguments for remote command execution
	bastionCfg *bastionClientConfig
}

// bastionClientConfig holds bastion connection configuration
type bastionClientConfig struct {
	hostPort string
	cfg      *ssh.ClientConfig
}

func NewClient(cfg *Config) (*Client, error) {
	authMethod, err := selectSSHAuthMethod(cfg)
	if err != nil {
		return nil, err
	}

	hostKeyCallback, err := getHostKeyCallback(cfg.StrictHostKey, cfg.KnownHostFile)
	if err != nil {
		return nil, fmt.Errorf("failed to setup host key verification: %w", err)
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultSSHTimeout
	}

	port := cfg.Port
	if port == "" || port == "0" {
		port = "22"
	}

	// Setup bastion configuration if provided
	var bastionCfg *bastionClientConfig
	if cfg.Bastion != nil {
		bastionCfg, err = newBastionClientConfig(cfg.Bastion, timeout)
		if err != nil {
			return nil, fmt.Errorf("failed to setup bastion client: %w", err)
		}
	}

	return &Client{
		hostPort: net.JoinHostPort(cfg.Host, port),
		cfg: &ssh.ClientConfig{
			User:            cfg.User,
			Auth:            []ssh.AuthMethod{authMethod},
			HostKeyCallback: hostKeyCallback,
			Timeout:         timeout,
		},
		Shell:      cfg.Shell,
		ShellArgs:  slices.Clone(cfg.ShellArgs),
		bastionCfg: bastionCfg,
	}, nil
}

func (c *Client) NewSession() (*ssh.Client, *ssh.Session, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, nil, err
	}

	session, err := conn.NewSession()
	if err != nil {
		conn.Close()
		return nil, nil, err
	}

	return conn, session, nil
}

// dial establishes an SSH connection either directly or via bastion host.
func (c *Client) dial() (*ssh.Client, error) {
	if c.bastionCfg != nil {
		return c.dialViaBastion()
	}
	return ssh.Dial("tcp", c.hostPort, c.cfg)
}

// dialViaBastion establishes an SSH connection through a bastion/jump host.
func (c *Client) dialViaBastion() (*ssh.Client, error) {
	// Connect to bastion host first
	bastionConn, err := ssh.Dial("tcp", c.bastionCfg.hostPort, c.bastionCfg.cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to bastion host: %w", err)
	}

	// Create a tunnel through bastion to the target host
	targetConn, err := bastionConn.Dial("tcp", c.hostPort)
	if err != nil {
		bastionConn.Close()
		return nil, fmt.Errorf("failed to dial target through bastion: %w", err)
	}

	// Perform SSH handshake over the tunnel
	ncc, chans, reqs, err := ssh.NewClientConn(targetConn, c.hostPort, c.cfg)
	if err != nil {
		targetConn.Close()
		bastionConn.Close()
		return nil, fmt.Errorf("failed to establish SSH connection through bastion: %w", err)
	}

	// Return the target SSH client
	// Note: The bastion connection is kept alive as long as the target connection is open.
	// When the target connection is closed, the bastion connection will also be closed.
	return ssh.NewClient(ncc, chans, reqs), nil
}

// newBastionClientConfig creates the bastion client configuration
func newBastionClientConfig(bastion *BastionConfig, timeout time.Duration) (*bastionClientConfig, error) {
	authMethod, err := selectBastionAuthMethod(bastion)
	if err != nil {
		return nil, err
	}

	port := bastion.Port
	if port == "" || port == "0" {
		port = "22"
	}

	return &bastionClientConfig{
		hostPort: net.JoinHostPort(bastion.Host, port),
		cfg: &ssh.ClientConfig{
			User: bastion.User,
			Auth: []ssh.AuthMethod{authMethod},
			// Bastion host key checking is disabled by default for simplicity.
			// In production, users should configure proper host key verification.
			HostKeyCallback: ssh.InsecureIgnoreHostKey(), // nolint: gosec
			Timeout:         timeout,
		},
	}, nil
}

// selectBastionAuthMethod determines the authentication method for bastion host.
func selectBastionAuthMethod(bastion *BastionConfig) (ssh.AuthMethod, error) {
	if bastion.Key != "" {
		keyPath, err := fileutil.ResolvePath(bastion.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve bastion key path: %w", err)
		}
		signer, err := getPublicKeySigner(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load bastion SSH key: %w", err)
		}
		return ssh.PublicKeys(signer), nil
	}

	if bastion.Password != "" {
		return ssh.Password(bastion.Password), nil
	}

	// Try default keys for bastion
	for _, defaultKey := range getDefaultSSHKeys() {
		if _, err := os.Stat(defaultKey); err == nil {
			signer, err := getPublicKeySigner(defaultKey)
			if err == nil {
				return ssh.PublicKeys(signer), nil
			}
		}
	}

	return nil, fmt.Errorf("no authentication method available for bastion: provide either SSH key or password")
}

// getHostKeyCallback returns the appropriate host key callback based on configuration
func getHostKeyCallback(strictHostKey bool, knownHostFile string) (ssh.HostKeyCallback, error) {
	if !strictHostKey {
		// User explicitly opted out of host key checking
		return ssh.InsecureIgnoreHostKey(), nil // nolint: gosec
	}

	// Default to ~/.ssh/known_hosts if not specified
	if knownHostFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		knownHostFile = filepath.Join(home, ".ssh", "known_hosts")
	}

	// Expand path if it starts with ~
	knownHostFile, err := fileutil.ResolvePath(knownHostFile)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve known_hosts path: %w", err)
	}

	return knownhosts.New(knownHostFile)
}

// getDefaultSSHKeys returns a list of default SSH key paths to try
func getDefaultSSHKeys() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{}
	}

	sshDir := filepath.Join(home, ".ssh")
	return []string{
		filepath.Join(sshDir, "id_rsa"),
		filepath.Join(sshDir, "id_ecdsa"),
		filepath.Join(sshDir, "id_ed25519"),
		filepath.Join(sshDir, "id_dsa"),
	}
}

// selectSSHAuthMethod selects the authentication method based on the configuration.
// Priority: explicit key > default keys > password.
func selectSSHAuthMethod(cfg *Config) (ssh.AuthMethod, error) {
	keyPath, err := resolveKeyPath(cfg)
	if err != nil {
		return nil, err
	}

	if keyPath != "" {
		signer, err := getPublicKeySigner(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
		}
		return ssh.PublicKeys(signer), nil
	}

	if cfg.Password != "" {
		return ssh.Password(cfg.Password), nil
	}

	return nil, fmt.Errorf("no authentication method available: provide either SSH key or password")
}

// resolveKeyPath determines the SSH key path to use.
// Returns empty string if password authentication should be used instead.
func resolveKeyPath(cfg *Config) (string, error) {
	if cfg.Key != "" {
		return fileutil.ResolvePath(cfg.Key)
	}

	if cfg.Password != "" {
		return "", nil
	}

	// Try default SSH keys
	for _, defaultKey := range getDefaultSSHKeys() {
		if _, err := os.Stat(defaultKey); err == nil {
			return defaultKey, nil
		}
	}

	return "", fmt.Errorf("no SSH key specified and no default keys found (~/.ssh/id_rsa, id_ecdsa, id_ed25519, or id_dsa)")
}

func getPublicKeySigner(path string) (ssh.Signer, error) {
	key, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}
