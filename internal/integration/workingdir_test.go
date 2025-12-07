package integration_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkingDirectoryResolution verifies working directory resolution:
//  1. DAG-level workingDir sets the working directory for steps
//  2. Step-level relative dir resolves against DAG's workingDir
//  3. SubDAG with explicit workingDir uses its own context (overrides inherited)
//  4. SubDAG without workingDir inherits parent's workingDir (for local execution)
func TestWorkingDirectoryResolution(t *testing.T) {
	th := test.Setup(t)

	// Create test directories
	dagsDir := th.Config.Paths.DAGsDir
	parentDir := filepath.Join(dagsDir, "parent_scripts")
	childDir := filepath.Join(dagsDir, "child_scripts")
	require.NoError(t, os.MkdirAll(parentDir, 0755))
	require.NoError(t, os.MkdirAll(childDir, 0755))

	// Platform-specific configuration
	shell, pwdCmd := "bash", "pwd"
	if runtime.GOOS == "windows" {
		shell, pwdCmd = "powershell", "(Get-Location).Path"
	}

	dag := th.DAG(t, `
shell: `+shell+`
workingDir: `+parentDir+`
steps:
  - name: parent_pwd
    command: `+pwdCmd+`
    output: PARENT_DIR

  - name: parent_relative_step
    dir: ../child_scripts
    command: `+pwdCmd+`
    output: PARENT_STEP_DIR

  - name: call_child_with_wd
    call: child_with_wd

  - name: call_child_no_wd
    call: child_no_wd

---

name: child_with_wd
shell: `+shell+`
workingDir: `+childDir+`
steps:
  - name: child_pwd
    command: `+pwdCmd+`

---

name: child_no_wd
shell: `+shell+`
steps:
  - name: child_pwd
    command: `+pwdCmd+`
`)

	dag.Agent().RunSuccess(t)

	// Verify parent DAG outputs
	dag.AssertOutputs(t, map[string]any{
		"PARENT_DIR":      test.Contains(parentDir),
		"PARENT_STEP_DIR": test.Contains(childDir),
	})

	// Verify subDAG working directories
	status, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, status.Status)

	ref := execution.NewDAGRunRef(status.Name, status.DAGRunID)

	for _, node := range status.Nodes {
		if len(node.SubRuns) == 0 {
			continue
		}

		subDir := getSubDAGWorkingDir(t, th, ref, node.SubRuns[0].DAGRunID)

		switch node.Step.Name {
		case "call_child_with_wd":
			assert.Contains(t, subDir, childDir,
				"SubDAG with explicit workingDir should run in childDir (overriding inherited)")
		case "call_child_no_wd":
			assert.Contains(t, subDir, parentDir,
				"SubDAG without workingDir should inherit parent's workingDir")
		}
	}
}

// getSubDAGWorkingDir retrieves the working directory from a subDAG's stdout log.
func getSubDAGWorkingDir(t *testing.T, th test.Helper, ref execution.DAGRunRef, subRunID string) string {
	t.Helper()

	subAttempt, err := th.DAGRunStore.FindSubAttempt(th.Context, ref, subRunID)
	require.NoError(t, err)

	subStatus, err := subAttempt.ReadStatus(th.Context)
	require.NoError(t, err)
	require.NotEmpty(t, subStatus.Nodes)

	logContent, err := os.ReadFile(subStatus.Nodes[0].Stdout)
	require.NoError(t, err)

	return strings.TrimSpace(string(logContent))
}
