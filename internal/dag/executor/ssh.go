// Copyright (C) 2024 The Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package executor

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/dagu-org/dagu/internal/dag"
	"github.com/mitchellh/mapstructure"
)

const (
	sshExecutorConfigUserEnvKey                  = "DAGU_SSH_CONFIG_USER"
	sshExecutorConfigIPEnvKey                    = "DAGU_SSH_CONFIG_IP"
	sshExecutorConfigPortEnvKey                  = "DAGU_SSH_CONFIG_PORT"
	sshExecutorConfigKeyEnvKey                   = "DAGU_SSH_CONFIG_KEY"
	sshExecutorConfigPasswordEnvKey              = "DAGU_SSH_CONFIG_PASSWORD"
	sshExecutorConfigStrictHostKeyCheckingEnvKey = "DAGU_SSH_CONFIG_STRICT_HOST_KEY_CHECKING"
)

type sshExec struct {
	step      dag.Step
	config    *sshExecConfig
	sshConfig *ssh.ClientConfig
	stdout    io.Writer
	session   *ssh.Session
}

type sshExecConfig struct {
	User                  string
	IP                    string
	Port                  int
	Key                   string
	Password              string
	StrictHostKeyChecking bool
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

func expendExecConfigUserKeyEnv() (any, error) {
	return os.Getenv(sshExecutorConfigUserEnvKey), nil
}

func expendExecConfigIPKeyEnv() (any, error) {
	return os.Getenv(sshExecutorConfigIPEnvKey), nil
}

func expendExecConfigPortKeyEnv() (any, error) {
	var (
		portStr = os.Getenv(sshExecutorConfigPortEnvKey)
		port    int
		err     error
	)

	if port, err = strconv.Atoi(portStr); err != nil {
		return nil, err
	}

	return port, nil
}

func expendExecConfigKeyKeyEnv() (any, error) {
	return os.Getenv(sshExecutorConfigKeyEnvKey), nil
}

func expendExecConfigPassKeyEnv() (any, error) {
	return os.Getenv(sshExecutorConfigPasswordEnvKey), nil
}

func expendExecConfigStrictHostKeyCheckingKeyEnv() (any, error) {
	var (
		strictHostKeyChecking = os.Getenv(sshExecutorConfigStrictHostKeyCheckingEnvKey)
		strictHostKey         bool
		err                   error
	)

	if strictHostKey, err = strconv.ParseBool(strictHostKeyChecking); err != nil {
		return nil, err
	}

	return strictHostKey, nil
}

func getExpendConfigEnvFunc(key string) handleExpendConfigEnv {
	switch key {
	case sshExecutorConfigUserEnvKey:
		return expendExecConfigUserKeyEnv
	case sshExecutorConfigIPEnvKey:
		return expendExecConfigIPKeyEnv
	case sshExecutorConfigPortEnvKey:
		return expendExecConfigPortKeyEnv
	case sshExecutorConfigKeyEnvKey:
		return expendExecConfigKeyKeyEnv
	case sshExecutorConfigPasswordEnvKey:
		return expendExecConfigPassKeyEnv
	case sshExecutorConfigStrictHostKeyCheckingEnvKey:
		return expendExecConfigStrictHostKeyCheckingKeyEnv
	}

	return nil
}

func expendExecSingleConfigEnv(configValue any) (any, error) {
	var (
		configValueStr      string
		ok                  bool
		expendConfigEnvFunc handleExpendConfigEnv
		expendConfigValue   interface{}
		err                 error
	)
	if configValue == nil {
		return nil, nil
	}

	if configValueStr, ok = configValue.(string); !ok || !strings.HasPrefix(configValueStr, daguConfigEnvPrefix) {
		return nil, nil
	}

	if expendConfigEnvFunc = getExpendConfigEnvFunc(strings.TrimPrefix(configValueStr, "$")); expendConfigEnvFunc == nil {
		return nil, errUnsupportedEnvKey
	}

	if expendConfigValue, err = expendConfigEnvFunc(); err != nil {
		return nil, err
	}

	return expendConfigValue, nil
}

func expendExecConfigEnv(cfg map[string]any) error {
	for configKey, configValue := range cfg {
		expendConfigValue, err := expendExecSingleConfigEnv(configValue)
		if err != nil {
			return err
		}

		if expendConfigValue != nil {
			cfg[configKey] = expendConfigValue
		}
	}

	return nil
}

func newSSHExec(_ context.Context, step dag.Step) (Executor, error) {
	cfg := new(sshExecConfig)
	md, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{Result: cfg},
	)

	if err != nil {
		return nil, err
	}

	if err = expendExecConfigEnv(step.ExecutorConfig.Config); err != nil {
		return nil, err
	}

	if err = md.Decode(step.ExecutorConfig.Config); err != nil {
		return nil, err
	}

	if cfg.Port == 0 {
		cfg.Port = 22
	}

	if cfg.StrictHostKeyChecking {
		return nil, errStrictHostKey
	}

	// Select the authentication method.
	authMethod, err := selectSSHAuthMethod(cfg)
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
		config:    cfg,
		sshConfig: sshConfig,
		stdout:    os.Stdout,
	}, nil
}

var errStrictHostKey = errors.New("StrictHostKeyChecking is not supported yet")
var errUnsupportedEnvKey = errors.New("unsupported environment key")

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

func (e *sshExec) Run() error {
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
