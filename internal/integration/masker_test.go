package integration_test

import (
	"os"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func TestSecretMasking(t *testing.T) {
	t.Parallel()

	t.Run("FileProviderMasksInStdout", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Create a temporary secret file
		secretValue := "super-secret-token-12345"
		secretFile := th.TempFile(t, "secret.txt", []byte(secretValue))

		dag := th.DAG(t, `
secrets:
  - name: API_TOKEN
    provider: file
    key: `+secretFile+`

steps:
  - name: echo-secret
    command: echo "Token is ${API_TOKEN}"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		// Get the dag-run status to find the stdout file
		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		// Read the stdout file - should contain masked value
		node := status.Nodes[0]
		stdoutFile := node.Stdout
		require.NotEmpty(t, stdoutFile, "stdout file should be set")

		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)

		// The secret value should be masked
		require.NotContains(t, string(stdoutContent), secretValue, "secret should be masked in stdout")
		require.Contains(t, string(stdoutContent), "*******", "masked placeholder should appear")
	})

	t.Run("EnvProviderMasksInOutput", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		secretValue := "my-api-key-67890"

		dag := th.DAG(t, `
env:
  - MY_SECRET=`+secretValue+`

secrets:
  - name: SECRET_FROM_ENV
    provider: env
    key: MY_SECRET

steps:
  - name: use-secret
    command: echo "Secret value is ${SECRET_FROM_ENV}"
    output: OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		// Get stdout file and verify masking
		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		node := status.Nodes[0]
		stdoutFile := node.Stdout
		stdoutContent, err := os.ReadFile(stdoutFile)
		require.NoError(t, err)

		require.NotContains(t, string(stdoutContent), secretValue, "secret should be masked")
		require.Contains(t, string(stdoutContent), "*******", "masked value should appear")
	})

	t.Run("MultipleSecretsMaskedCorrectly", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		secret1 := "secret-one-abc"
		secret2 := "secret-two-xyz"
		secretFile1 := th.TempFile(t, "secret1.txt", []byte(secret1))
		secretFile2 := th.TempFile(t, "secret2.txt", []byte(secret2))

		dag := th.DAG(t, `
secrets:
  - name: SECRET1
    provider: file
    key: `+secretFile1+`
  - name: SECRET2
    provider: file
    key: `+secretFile2+`

steps:
  - name: echo-both
    command: |
      echo "First: ${SECRET1}"
      echo "Second: ${SECRET2}"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		node := status.Nodes[0]
		stdoutContent, err := os.ReadFile(node.Stdout)
		require.NoError(t, err)

		// Both secrets should be masked
		require.NotContains(t, string(stdoutContent), secret1)
		require.NotContains(t, string(stdoutContent), secret2)
		require.Contains(t, string(stdoutContent), "*******")
	})

	t.Run("SecretMaskedInLogFile", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		secretValue := "log-secret-999"
		secretFile := th.TempFile(t, "log-secret.txt", []byte(secretValue))

		dag := th.DAG(t, `
secrets:
  - name: LOG_SECRET
    provider: file
    key: `+secretFile+`

steps:
  - name: log-step
    command: |
      echo "Before secret"
      echo "The secret is ${LOG_SECRET}"
      echo "After secret"
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		// Get node and read both stdout and the main log
		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		node := status.Nodes[0]
		stdoutContent, err := os.ReadFile(node.Stdout)
		require.NoError(t, err)

		// Verify secret is masked in stdout
		require.NotContains(t, string(stdoutContent), secretValue)
		require.Contains(t, string(stdoutContent), "Before secret")
		require.Contains(t, string(stdoutContent), "After secret")
	})

	t.Run("NoSecretsNoMasking", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		normalValue := "not-a-secret"

		dag := th.DAG(t, `
steps:
  - name: normal-output
    command: echo "Value is `+normalValue+`"
    output: RESULT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		node := status.Nodes[0]
		stdoutContent, err := os.ReadFile(node.Stdout)
		require.NoError(t, err)

		// Normal value should not be masked
		require.Contains(t, string(stdoutContent), normalValue)
		require.NotContains(t, string(stdoutContent), "*******")
	})

	t.Run("RelativePathWithWorkingDir", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Create a custom working directory
		workDir := t.TempDir()
		secretValue := "relative-path-secret-999"

		// Create secret file directly in the working directory
		secretFilePath := workDir + "/secret.txt"
		require.NoError(t, os.WriteFile(secretFilePath, []byte(secretValue), 0600))

		dag := th.DAG(t, `
workingDir: `+workDir+`

secrets:
  - name: RELATIVE_SECRET
    provider: file
    key: secret.txt

steps:
  - name: use-relative-secret
    command: |
      echo "Secret from relative path ${RELATIVE_SECRET}"
    output: OUTPUT
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertLatestStatus(t, core.Success)

		status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Len(t, status.Nodes, 1)

		node := status.Nodes[0]
		stdoutContent, err := os.ReadFile(node.Stdout)
		require.NoError(t, err)

		// Secret should be masked even when loaded from relative path
		require.NotContains(t, string(stdoutContent), secretValue, "secret loaded from relative path should be masked")
		require.Contains(t, string(stdoutContent), "*******", "masked value should appear")
		require.Contains(t, string(stdoutContent), "Secret from relative path", "log message should be present")
	})
}
