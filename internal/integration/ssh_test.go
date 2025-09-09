package integration_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/digraph/status"
	"github.com/dagu-org/dagu/internal/test"
)

// TestSSHIntegration spins up an SSH server inside a container using the
// parent DAG, then runs a child SSH DAG that echoes a deterministic string over SSH.
// Pattern mirrors container_test.go: define YAML inline and assert outputs.
func TestSSHIntegration(t *testing.T) {
	t.Parallel()

	if os.Getenv("GITHUB_ACTIONS") == "true" {
		t.Skip("Skipping SSH integration test on GitHub Actions environment")
	}

	th := test.Setup(t)

	// Parent DAG runs a long-lived container that starts sshd on port 2222
	// and exposes it to the host; child DAG connects via SSH and runs `hostname`.
	dag := th.DAG(t, `
container:
  image: alpine:3
  # Expose container port 2222 to host port 2222
  ports:
    - "2222:2222"
  # Run a long-lived sshd process and log to stderr (-e) so LogPattern can match
  startup: command
  command: ["sh", "-c", "apk add --no-cache openssh >/dev/null 2>&1 && ssh-keygen -A >/dev/null 2>&1 && echo 'root:root' | chpasswd && /usr/sbin/sshd -D -e -p 2222 -o PermitRootLogin=yes -o PasswordAuthentication=yes -o UsePAM=no"]
  # Wait for sshd to be ready by matching logs
  logPattern: "Server listening"
steps:
  - name: run-ssh
    run: ssh-exec
    params: "HOST=127.0.0.1 PORT=2222 USER=root PASSWORD=root"
    output: SSH_RUN

  - name: echo-result
    command: echo "${SSH_RUN.outputs.RESULT}"
    depends: [run-ssh]
    output: SSH_RESULT

---

name: ssh-exec
env:
  HOST: 127.0.0.1
  PORT: 2222
  USER: root
  PASSWORD: root
params:
  HOST: ${HOST}
  PORT: ${PORT}
  USER: ${USER}
  PASSWORD: ${PASSWORD}
# Provide SSH defaults for the step executed via SSH.
ssh:
  host: ${HOST}
  port: ${PORT}
  user: ${USER}
  password: ${PASSWORD}
  strictHostKey: false
steps:
  - command: echo dagu-ssh-ok
    output: RESULT
`)

	dag.Agent().RunSuccess(t)
	dag.AssertLatestStatus(t, status.Success)
	dag.AssertOutputs(t, map[string]any{
		"SSH_RESULT": "dagu-ssh-ok",
	})
}
