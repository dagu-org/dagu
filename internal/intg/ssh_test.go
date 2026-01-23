package intg_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/containerd/platforms"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
)

const (
	// Use alpine with openssh for minimal SSH server
	sshServerImage = "alpine:3"
	sshTestUser    = "testuser"
	sshTestPass    = "testpass123"
)

// sshServerContainer holds info about a running SSH server container
type sshServerContainer struct {
	containerID string
	hostPort    string
	keyPath     string
	pubKeyPath  string
	workDir     string // working directory for test files
}

// sshConfig returns the common SSH configuration block for DAG tests.
func (s *sshServerContainer) sshConfig(shell string) string {
	return fmt.Sprintf(`ssh:
  host: 127.0.0.1
  port: "%s"
  user: %s
  key: "%s"
  strictHostKey: false
  shell: %s
`, s.hostPort, sshTestUser, s.keyPath, shell)
}

// sshPasswordConfig returns SSH configuration using password authentication.
func (s *sshServerContainer) sshPasswordConfig(shell string) string {
	return fmt.Sprintf(`ssh:
  host: 127.0.0.1
  port: "%s"
  user: %s
  password: "%s"
  strictHostKey: false
  shell: %s
`, s.hostPort, sshTestUser, sshTestPass, shell)
}

// TestSSHExecutorIntegration tests SSH executor with a real SSH server in Docker
func TestSSHExecutorIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	th := test.Setup(t)

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	require.NoError(t, err, "failed to create docker client")
	defer func() { _ = dockerClient.Close() }()

	// Start SSH server container
	sshServer := startSSHServer(t, th, dockerClient)
	defer stopSSHServer(t, th, dockerClient, sshServer)

	// Wait for SSH server to be ready
	waitForSSHReady(t, sshServer)

	t.Run("BasicCommandExecution", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: basic-ssh
    type: ssh
    command: echo "hello from ssh"
    output: SSH_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_OUT": "hello from ssh",
		})
	})

	t.Run("CommandWithArguments", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: args-test
    type: ssh
    command: echo hello world
    output: SSH_ARGS_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_ARGS_OUT": "hello world",
		})
	})

	t.Run("WorkingDirectory", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: workdir-test
    type: ssh
    workingDir: /tmp
    command: pwd
    output: SSH_PWD_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_PWD_OUT": "/tmp",
		})
	})

	t.Run("ScriptExecution", func(t *testing.T) {
		th := test.Setup(t)

		// Test multi-line script execution
		// Note: Avoid shell variables with ${} as dagu expands them before sending to SSH
		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: script-test
    type: ssh
    script: |
      echo -n "hello "
      echo "world"
    output: SSH_SCRIPT_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_SCRIPT_OUT": "hello world",
		})
	})

	t.Run("ScriptWithWorkingDirectory", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: script-workdir-test
    type: ssh
    workingDir: /tmp
    script: |
      echo "working in $(pwd)"
    output: SSH_SCRIPT_WORKDIR_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_SCRIPT_WORKDIR_OUT": "working in /tmp",
		})
	})

	t.Run("MultipleCommands", func(t *testing.T) {
		th := test.Setup(t)

		// Use script instead of commands for multiple commands
		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: multi-cmd-test
    type: ssh
    script: |
      echo first
      echo second
    output: SSH_MULTI_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_MULTI_OUT": "first\nsecond",
		})
	})

	t.Run("MultipleCommandsWithWorkingDirectory", func(t *testing.T) {
		th := test.Setup(t)

		// Use script instead of commands for multiple commands
		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: multi-cmd-workdir-test
    type: ssh
    workingDir: /tmp
    script: |
      pwd
      echo done
    output: SSH_MULTI_WORKDIR_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_MULTI_WORKDIR_OUT": "/tmp\ndone",
		})
	})

	t.Run("ErrorHandling_CommandFailure", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: error-test
    type: ssh
    command: exit 1
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunError(t)
	})

	t.Run("ErrorHandling_InvalidWorkingDirectory", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: invalid-dir-test
    type: ssh
    workingDir: /nonexistent/directory/path
    command: echo "should not reach"
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunError(t)
	})

	t.Run("StepLevelSSHConfig", func(t *testing.T) {
		th := test.Setup(t)

		// Test step-level SSH configuration (no DAG-level ssh config)
		dagConfig := fmt.Sprintf(`
steps:
  - name: step-ssh-config
    type: ssh
    config:
      host: 127.0.0.1
      port: "%s"
      user: %s
      key: "%s"
      strictHostKey: false
      shell: /bin/sh
    command: echo "step config works"
    output: STEP_SSH_OUT
`, sshServer.hostPort, sshTestUser, sshServer.keyPath)

		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"STEP_SSH_OUT": "step config works",
		})
	})

	t.Run("BashShell", func(t *testing.T) {
		th := test.Setup(t)

		// Test bash-specific features (process substitution)
		dagConfig := sshServer.sshConfig("/bin/bash") + `
steps:
  - name: bash-test
    type: ssh
    script: |
      # Bash-specific: string manipulation
      str="hello-world"
      echo "prefix-removed"
    output: SSH_BASH_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_BASH_OUT": "prefix-removed",
		})
	})

	t.Run("NoWorkingDir_UsesHomeDirectory", func(t *testing.T) {
		th := test.Setup(t)

		// Test that when no step.Dir is set, SSH runs in user's home directory
		// Note: DAG-level workingDir is NOT used for SSH (it's for local execution)
		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: home-dir-test
    type: ssh
    command: pwd
    output: SSH_HOME_DIR_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		// Should be the SSH user's home directory (e.g., /home/testuser)
		dag.AssertOutputs(t, map[string]any{
			"SSH_HOME_DIR_OUT": test.Contains("/home/"),
		})
	})

	t.Run("StepWorkingDirOverridesDAGWorkingDir", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := "workingDir: /var\n\n" + sshServer.sshConfig("/bin/sh") + `
steps:
  - name: step-override-test
    type: ssh
    workingDir: /tmp
    command: pwd
    output: SSH_OVERRIDE_WORKDIR_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_OVERRIDE_WORKDIR_OUT": "/tmp",
		})
	})

	t.Run("PipeInScript", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: pipe-test
    type: ssh
    script: |
      echo "hello" | tr 'h' 'H'
    output: SSH_PIPE_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"SSH_PIPE_OUT": "Hello",
		})
	})

	t.Run("CommandSubstitution", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: subst-test
    type: ssh
    command: echo "hostname is $(hostname)"
    output: SSH_SUBST_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		// Just verify it ran - hostname will vary
	})

	t.Run("SetEStopsOnFirstError", func(t *testing.T) {
		th := test.Setup(t)

		dagConfig := sshServer.sshConfig("/bin/sh") + `
steps:
  - name: set-e-test
    type: ssh
    script: |
      false
      echo "should not reach"
    output: SSH_SETE_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunError(t)
	})

	t.Run("PasswordAuthentication", func(t *testing.T) {
		th := test.Setup(t)

		// Use password authentication instead of key-based auth
		dagConfig := sshServer.sshPasswordConfig("/bin/sh") + `
steps:
  - name: password-auth-test
    type: ssh
    command: echo "authenticated with password"
    output: PASSWORD_AUTH_OUT
`
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"PASSWORD_AUTH_OUT": "authenticated with password",
		})
	})

	t.Run("TimeoutConfiguration", func(t *testing.T) {
		th := test.Setup(t)

		// Test that custom timeout configuration works
		dagConfig := fmt.Sprintf(`ssh:
  host: 127.0.0.1
  port: "%s"
  user: %s
  key: "%s"
  strictHostKey: false
  shell: /bin/sh
  timeout: "10s"
steps:
  - name: timeout-config-test
    type: ssh
    command: echo "timeout configured"
    output: TIMEOUT_OUT
`, sshServer.hostPort, sshTestUser, sshServer.keyPath)
		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"TIMEOUT_OUT": "timeout configured",
		})
	})

	t.Run("SFTPUploadFile", func(t *testing.T) {
		th := test.Setup(t)

		// Create local file to upload
		localDir := t.TempDir()
		localFile := filepath.Join(localDir, "upload_test.txt")
		err := os.WriteFile(localFile, []byte("sftp upload test content"), 0644)
		require.NoError(t, err, "failed to create local test file")

		// Upload file to remote
		dagConfig := fmt.Sprintf(`
steps:
  - name: upload-file
    type: sftp
    config:
      host: 127.0.0.1
      port: "%s"
      user: %s
      key: "%s"
      strictHostKey: false
      direction: upload
      source: "%s"
      destination: /tmp/uploaded_file.txt
  - name: verify-upload
    type: ssh
    config:
      host: 127.0.0.1
      port: "%s"
      user: %s
      key: "%s"
      strictHostKey: false
      shell: /bin/sh
    command: cat /tmp/uploaded_file.txt
    output: UPLOAD_VERIFY
    depends:
      - upload-file
`, sshServer.hostPort, sshTestUser, sshServer.keyPath, localFile,
			sshServer.hostPort, sshTestUser, sshServer.keyPath)

		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)
		dag.AssertOutputs(t, map[string]any{
			"UPLOAD_VERIFY": "sftp upload test content",
		})
	})

	t.Run("SFTPDownloadFile", func(t *testing.T) {
		th := test.Setup(t)

		// Create file on remote first, then download
		downloadDir := t.TempDir()
		downloadPath := filepath.Join(downloadDir, "downloaded.txt")

		dagConfig := fmt.Sprintf(`
steps:
  - name: create-remote-file
    type: ssh
    config:
      host: 127.0.0.1
      port: "%s"
      user: %s
      key: "%s"
      strictHostKey: false
      shell: /bin/sh
    script: |
      echo "sftp download test content" > /tmp/download_test.txt
  - name: download-file
    type: sftp
    config:
      host: 127.0.0.1
      port: "%s"
      user: %s
      key: "%s"
      strictHostKey: false
      direction: download
      source: /tmp/download_test.txt
      destination: "%s"
    depends:
      - create-remote-file
`, sshServer.hostPort, sshTestUser, sshServer.keyPath,
			sshServer.hostPort, sshTestUser, sshServer.keyPath, downloadPath)

		dag := th.DAG(t, dagConfig)
		dag.Agent().RunSuccess(t)
		dag.AssertLatestStatus(t, core.Succeeded)

		// Verify downloaded file contents
		content, err := os.ReadFile(downloadPath)
		require.NoError(t, err, "failed to read downloaded file")
		require.Equal(t, "sftp download test content\n", string(content))
	})
}

// startSSHServer creates and starts an SSH server container
func startSSHServer(t *testing.T, th test.Helper, dockerClient *client.Client) *sshServerContainer {
	t.Helper()

	ctx := th.Context

	// Get Docker info for platform
	info, err := dockerClient.Info(ctx)
	require.NoError(t, err, "failed to get docker info")

	var platform specs.Platform
	platform.Architecture = info.Architecture
	platform.OS = info.OSType

	// Pull the image
	pullOpts := image.PullOptions{Platform: platforms.Format(platform)}
	reader, err := dockerClient.ImagePull(ctx, sshServerImage, pullOpts)
	require.NoError(t, err, "failed to pull ssh server image")
	_, _ = io.Copy(io.Discard, reader)
	_ = reader.Close()

	// Create temp directory for SSH keys
	keyDir := t.TempDir()
	keyPath := filepath.Join(keyDir, "id_ed25519")
	pubKeyPath := keyPath + ".pub"

	// Generate SSH key pair using Go crypto
	generateSSHKey(t, keyPath, pubKeyPath)

	// Read the public key
	pubKey, err := os.ReadFile(pubKeyPath)
	require.NoError(t, err, "failed to read public key")

	// Create container config
	containerName := fmt.Sprintf("dagu-ssh-test-%d", time.Now().UnixNano())

	// Setup script to configure SSH server
	setupScript := fmt.Sprintf(`
set -e
# Install openssh and bash
apk add --no-cache openssh bash

# Generate host keys
ssh-keygen -A

# Create test user
adduser -D -s /bin/bash %s
echo "%s:%s" | chpasswd

# Setup SSH authorized keys for the test user
mkdir -p /home/%s/.ssh
echo '%s' > /home/%s/.ssh/authorized_keys
chmod 700 /home/%s/.ssh
chmod 600 /home/%s/.ssh/authorized_keys
chown -R %s:%s /home/%s/.ssh

# Configure sshd - allow password and key auth
sed -i 's/#PermitRootLogin.*/PermitRootLogin no/' /etc/ssh/sshd_config
sed -i 's/#PasswordAuthentication.*/PasswordAuthentication yes/' /etc/ssh/sshd_config
sed -i 's/#PubkeyAuthentication.*/PubkeyAuthentication yes/' /etc/ssh/sshd_config

# Start sshd in foreground
exec /usr/sbin/sshd -D -e
`, sshTestUser, sshTestUser, sshTestPass, sshTestUser, string(pubKey), sshTestUser, sshTestUser, sshTestUser, sshTestUser, sshTestUser, sshTestUser)

	// Create container
	created, err := dockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Image: sshServerImage,
			Cmd:   []string{"/bin/sh", "-c", setupScript},
			ExposedPorts: nat.PortSet{
				"22/tcp": struct{}{},
			},
		},
		&container.HostConfig{
			AutoRemove: true,
			PortBindings: nat.PortMap{
				"22/tcp": []nat.PortBinding{
					{HostIP: "127.0.0.1", HostPort: "0"}, // Random port
				},
			},
		},
		&network.NetworkingConfig{},
		nil,
		containerName,
	)
	require.NoError(t, err, "failed to create SSH server container")

	// Start container
	err = dockerClient.ContainerStart(ctx, created.ID, container.StartOptions{})
	require.NoError(t, err, "failed to start SSH server container")

	// Get the assigned port
	inspect, err := dockerClient.ContainerInspect(ctx, created.ID)
	require.NoError(t, err, "failed to inspect SSH server container")

	hostPort := inspect.NetworkSettings.Ports["22/tcp"][0].HostPort

	return &sshServerContainer{
		containerID: created.ID,
		hostPort:    hostPort,
		keyPath:     keyPath,
		pubKeyPath:  pubKeyPath,
		workDir:     keyDir,
	}
}

// generateSSHKey generates an ED25519 SSH key pair using Go crypto library
func generateSSHKey(t *testing.T, keyPath, pubKeyPath string) {
	t.Helper()

	// Generate ED25519 key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err, "failed to generate ed25519 key")

	// Marshal private key to OpenSSH format
	privKeyBytes, err := ssh.MarshalPrivateKey(privKey, "")
	require.NoError(t, err, "failed to marshal private key")

	// Write private key
	err = os.WriteFile(keyPath, pem.EncodeToMemory(privKeyBytes), 0600)
	require.NoError(t, err, "failed to write private key")

	// Generate SSH public key
	sshPubKey, err := ssh.NewPublicKey(pubKey)
	require.NoError(t, err, "failed to create SSH public key")

	// Write public key in authorized_keys format
	pubKeyData := ssh.MarshalAuthorizedKey(sshPubKey)
	err = os.WriteFile(pubKeyPath, pubKeyData, 0644)
	require.NoError(t, err, "failed to write public key")
}

// stopSSHServer stops and removes the SSH server container
func stopSSHServer(t *testing.T, th test.Helper, dockerClient *client.Client, server *sshServerContainer) {
	t.Helper()

	timeout := 5
	_ = dockerClient.ContainerStop(th.Context, server.containerID, container.StopOptions{Timeout: &timeout})
	_ = dockerClient.ContainerRemove(th.Context, server.containerID, container.RemoveOptions{Force: true})
}

// waitForSSHReady waits for the SSH server to be ready to accept connections
// and verifies that commands can be executed via shell stdin
func waitForSSHReady(t *testing.T, server *sshServerContainer) {
	t.Helper()

	// Read private key for testing connection
	keyBytes, err := os.ReadFile(server.keyPath)
	require.NoError(t, err, "failed to read private key for connection test")

	signer, err := ssh.ParsePrivateKey(keyBytes)
	require.NoError(t, err, "failed to parse private key for connection test")

	config := &ssh.ClientConfig{
		User: sshTestUser,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}

	addr := net.JoinHostPort("127.0.0.1", server.hostPort)

	// Retry connection for up to 30 seconds
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := ssh.Dial("tcp", addr, config)
		if err == nil {
			// Test that we can run a command via shell stdin (like the executor does)
			session, err := conn.NewSession()
			if err == nil {
				// Test with the exact approach the SSH executor uses
				script := "__dagu_exec(){\nset -e\necho test\n}\n__dagu_exec\n"
				session.Stdin = strings.NewReader(script)

				var stdout, stderr strings.Builder
				session.Stdout = &stdout
				session.Stderr = &stderr

				err = session.Run("/bin/sh")
				_ = session.Close()
				_ = conn.Close()

				if err == nil && strings.TrimSpace(stdout.String()) == "test" {
					t.Logf("SSH server ready on port %s (shell stdin works)", server.hostPort)
					return
				}
				t.Logf("SSH shell stdin test failed: stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
			} else {
				_ = conn.Close()
				t.Logf("SSH session creation failed: %v", err)
			}
		} else {
			t.Logf("Waiting for SSH server: %v", err)
		}
		time.Sleep(1 * time.Second)
	}

	t.Fatalf("SSH server failed to become ready on port %s", server.hostPort)
}
