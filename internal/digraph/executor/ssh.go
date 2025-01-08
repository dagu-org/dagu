package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"golang.org/x/crypto/ssh"

	"github.com/dagu-org/dagu/internal/digraph"
)

var _ Executor = (*sshExec)(nil)

type sshExec struct {
	step      digraph.Step
	config    *sshExecConfig
	sshConfig *ssh.ClientConfig
	stdout    io.Writer
	session   *ssh.Session
}

type sshExecConfigDefinition struct {
	User                  string
	IP                    string
	Port                  string
	Key                   string
	Password              string
	StrictHostKeyChecking bool
}

type sshExecConfig struct {
	User     string
	IP       string
	Port     string
	Key      string
	Password string
}

// selectSSHAuthMethod selects the authentication method based on the configuration.
// If the key is provided, it will use the public key authentication method.
// Otherwise, it will use the password authentication method.
func selectSSHAuthMethod(cfg *sshExecConfig) (ssh.AuthMethod, error) {
	var (
		signer ssh.Signer
		err    error
	)

	if len(cfg.Key) != 0 {
		// Create the Signer for this private key.
		if signer, err = getPublicKeySigner(cfg.Key); err != nil {
			return nil, err
		}

		return ssh.PublicKeys(signer), nil
	}

	return ssh.Password(cfg.Password), nil
}

func newSSHExec(ctx context.Context, step digraph.Step) (Executor, error) {
	def := new(sshExecConfigDefinition)
	md, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{Result: def, WeaklyTypedInput: true},
	)

	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := md.Decode(step.ExecutorConfig.Config); err != nil {
		return nil, fmt.Errorf("failed to decode ssh config: %w", err)
	}

	if def.Port == "0" || def.Port == "" {
		def.Port = "22"
	}

	stepContext := digraph.GetStepContext(ctx)
	cfg, err := digraph.EvalStringFields(stepContext, sshExecConfig{
		User:     def.User,
		IP:       def.IP,
		Key:      def.Key,
		Password: def.Password,
		Port:     def.Port,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields for ssh config: %w", err)
	}

	// StrictHostKeyChecking is not supported yet.
	if def.StrictHostKeyChecking {
		return nil, errStrictHostKey
	}

	cfg, err = digraph.EvalStringFields(stepContext, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to substitute string fields for ssh config: %w", err)
	}

	// Select the authentication method.
	authMethod, err := selectSSHAuthMethod(&cfg)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		// nolint: gosec
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &sshExec{
		step:      step,
		config:    &cfg,
		sshConfig: sshConfig,
		stdout:    os.Stdout,
	}, nil
}

var errStrictHostKey = errors.New("StrictHostKeyChecking is not supported yet")

func (e *sshExec) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *sshExec) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *sshExec) Kill(_ os.Signal) error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}

func (e *sshExec) Run(_ context.Context) error {
	addr := net.JoinHostPort(e.config.IP, e.config.Port)
	conn, err := ssh.Dial("tcp", addr, e.sshConfig)
	if err != nil {
		return err
	}

	session, err := conn.NewSession()
	if err != nil {
		return err
	}
	e.session = session
	defer session.Close()

	// Once a Session is created, you can execute a single command on
	// the remote side using the Run method.
	session.Stdout = e.stdout
	session.Stderr = e.stdout
	command := strings.Join(
		append([]string{e.step.Command}, e.step.Args...), " ",
	)
	return session.Run(command)
}

// referenced code:
//
//	https://go.googlesource.com/crypto/+/master/ssh/example_test.go
//	https://gist.github.com/boyzhujian/73b5ecd37efd6f8dd38f56e7588f1b58
func getPublicKeySigner(path string) (ssh.Signer, error) {
	// A public key may be used to authenticate against the remote
	// frontend by using an unencrypted PEM-encoded private key file.
	//
	// If you have an encrypted private key, the crypto/x509 package
	// can be used to decrypt it.
	key, err := os.ReadFile(path)
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

func init() {
	Register("ssh", newSSHExec)
}
