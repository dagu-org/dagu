package ssh

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type Client struct {
	hostPort string
	cfg      *ssh.ClientConfig
	Shell    string // Shell for remote command execution
}

func NewClient(cfg *Config) (*Client, error) {
	authMethod, err := selectSSHAuthMethod(cfg)
	if err != nil {
		return nil, err
	}

	// Get host key callback for SSH verification
	hostKeyCallback, err := getHostKeyCallback(cfg.StrictHostKey, cfg.KnownHostFile)
	if err != nil {
		return nil, fmt.Errorf("failed to setup host key verification: %w", err)
	}

	clientConfig := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{authMethod},
		HostKeyCallback: hostKeyCallback,
	}

	port := cfg.Port
	if port == "" || port == "0" {
		port = "22"
	}

	return &Client{
		hostPort: net.JoinHostPort(cfg.Host, port),
		cfg:      clientConfig,
		Shell:    cfg.Shell,
	}, nil
}

func (c *Client) NewSession() (*ssh.Session, error) {
	conn, err := ssh.Dial("tcp", c.hostPort, c.cfg)
	if err != nil {
		return nil, err
	}

	return conn.NewSession()
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
// If the key is provided, it will use the public key authentication method.
// If no key is provided, it will try default SSH keys.
// Otherwise, it will use the password authentication method.
func selectSSHAuthMethod(cfg *Config) (ssh.AuthMethod, error) {
	var signer ssh.Signer
	var keyPath string

	// If key is specified, use it
	if len(cfg.Key) != 0 {
		resolvedPath, err := fileutil.ResolvePath(cfg.Key)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve key path: %w", err)
		}
		keyPath = resolvedPath
	} else if cfg.Password == "" {
		// No key specified and no password, try default keys
		for _, defaultKey := range getDefaultSSHKeys() {
			if _, err := os.Stat(defaultKey); err == nil {
				keyPath = defaultKey
				break
			}
		}
		if keyPath == "" {
			return nil, fmt.Errorf("no SSH key specified and no default keys found (~/.ssh/id_rsa, id_ecdsa, id_ed25519, or id_dsa)")
		}
	}

	// If we have a key path, use public key authentication
	if keyPath != "" {
		var err error
		if signer, err = getPublicKeySigner(keyPath); err != nil {
			return nil, fmt.Errorf("failed to load SSH key from %s: %w", keyPath, err)
		}
		return ssh.PublicKeys(signer), nil
	}

	// Fall back to password authentication if provided
	if cfg.Password != "" {
		return ssh.Password(cfg.Password), nil
	}

	// No authentication method available
	return nil, fmt.Errorf("no authentication method available: provide either SSH key or password")
}

// ref:
//
//	https://go.googlesource.com/crypto/+/master/ssh/example_test.go
//	https://gist.github.com/boyzhujian/73b5ecd37efd6f8dd38f56e7588f1b58
func getPublicKeySigner(path string) (ssh.Signer, error) {
	// A public key may be used to authenticate against the remote
	// frontend by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	key, err := os.ReadFile(path) //nolint:gosec
	if err != nil {
		return nil, err
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	return signer, nil
}
