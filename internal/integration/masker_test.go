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

	t.Run("EmptySecretValueDoesNotMaskEverything", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Test that empty secret values don't cause masking of all output
		// (which would happen if strings.ReplaceAll is called with empty string)
		normalOutput := "This is normal output that should not be masked"

		dag := th.DAG(t, `
env:
  - EMPTY_SECRET=

secrets:
  - name: EMPTY_SECRET
    provider: env
    key: EMPTY_SECRET

steps:
  - name: test-empty-secret
    command: echo "`+normalOutput+`"
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

		// The output should NOT be masked (no stream of *******)
		require.Contains(t, string(stdoutContent), normalOutput, "output should not be masked when secret is empty")
		// Should not have replaced every character with *******
		require.NotContains(t, string(stdoutContent), "*******T*******h*******i*******s", "should not mask between every character")
	})

	t.Run("RelativePathFromDAGLocation", func(t *testing.T) {
		t.Parallel()

		th := test.Setup(t)

		// Create a working directory that's different from where the DAG file is
		workDir := t.TempDir()

		secretValue := "dag-location-secret-123"

		// Ensure DAGsDir exists
		require.NoError(t, os.MkdirAll(th.Config.Paths.DAGsDir, 0750))

		// Create secret file in the DAG directory (th.Config.Paths.DAGsDir)
		// The DAG method creates DAG files in this directory, so we put the secret there too
		secretFilePath := th.Config.Paths.DAGsDir + "/secret.txt"
		require.NoError(t, os.WriteFile(secretFilePath, []byte(secretValue), 0600))

		// Create DAG with workingDir set to a different directory
		// The secret file is NOT in workingDir, but in the DAG file's directory
		dag := th.DAG(t, `
workingDir: `+workDir+`

secrets:
  - name: DAG_SECRET
    provider: file
    key: secret.txt

steps:
  - name: use-dag-location-secret
    command: |
      echo "Secret from DAG location ${DAG_SECRET}"
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

		// Secret should be found (from DAG location) and masked
		require.NotContains(t, string(stdoutContent), secretValue, "secret loaded from DAG location should be masked")
		require.Contains(t, string(stdoutContent), "*******", "masked value should appear")
		require.Contains(t, string(stdoutContent), "Secret from DAG location", "log message should be present")
	})
}
