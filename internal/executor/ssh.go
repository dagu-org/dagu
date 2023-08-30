package executor

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/dagu-dev/dagu/internal/dag"
	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	User                  string
	IP                    string
	Port                  int
	Key                   string
	StrictHostKeyChecking bool
}

type SSHExecutor struct {
	step      *dag.Step
	config    *SSHConfig
	sshConfig *ssh.ClientConfig
	stdout    io.Writer
	session   *ssh.Session
}

func (e *SSHExecutor) SetStdout(out io.Writer) {
	e.stdout = out
}

func (e *SSHExecutor) SetStderr(out io.Writer) {
	e.stdout = out
}

func (e *SSHExecutor) Kill(sig os.Signal) error {
	if e.session != nil {
		return e.session.Close()
	}
	return nil
}

func (e *SSHExecutor) Run() error {
	addr := fmt.Sprintf("%s:%d", e.config.IP, e.config.Port)
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
	command := strings.Join(append([]string{e.step.Command}, e.step.Args...), " ")
	return session.Run(command)
}

func CreateSSHExecutor(ctx context.Context, step *dag.Step) (Executor, error) {
	cfg := &SSHConfig{}
	md, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: cfg})

	if err != nil {
		return nil, err
	}

	if err := md.Decode(step.ExecutorConfig.Config); err != nil {
		return nil, err
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}

	if cfg.StrictHostKeyChecking {
		return nil, fmt.Errorf("StrictHostKeyChecking is not supported yet")
	}

	// Create the Signer for this private key.
	signer, err := getPublicKeySigner(cfg.Key)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			// Use the PublicKeys method for remote authentication.
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	return &SSHExecutor{
		step:      step,
		config:    cfg,
		sshConfig: sshConfig,
		stdout:    os.Stdout,
	}, nil
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
	Register("ssh", CreateSSHExecutor)
}
