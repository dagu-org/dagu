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

// TestWorkingDirectoryResolution verifies that working directory resolution works correctly:
// 1. DAG-level workingDir sets the working directory for steps
// 2. Step-level relative dir resolves against DAG's workingDir
// 3. SubDAG with explicit workingDir uses its own context
// 4. SubDAG without workingDir inherits from its DAG file location (not parent's workingDir)
func TestWorkingDirectoryResolution(t *testing.T) {
	th := test.Setup(t)

	// Create test directory structure:
	// dagsDir/
	//   parent_scripts/
	//   child_scripts/
	dagsDir := th.Config.Paths.DAGsDir
	parentScripts := filepath.Join(dagsDir, "parent_scripts")
	childScripts := filepath.Join(dagsDir, "child_scripts")
	require.NoError(t, os.MkdirAll(parentScripts, 0755))
	require.NoError(t, os.MkdirAll(childScripts, 0755))

	// Platform-specific shell configuration
	shell := "bash"
	pwdCmd := "pwd"
	if runtime.GOOS == "windows" {
		shell = "powershell"
		pwdCmd = "(Get-Location).Path"
	}

	dag := th.DAG(t, `
shell: `+shell+`
workingDir: `+parentScripts+`
steps:
  # Step 1: Verify DAG workingDir is set correctly
  - name: parent_pwd
    command: `+pwdCmd+`
    output: PARENT_DIR

  # Step 2: Verify relative dir resolves against DAG's workingDir
  # ../child_scripts from parent_scripts -> child_scripts
  - name: parent_relative_step
    dir: ../child_scripts
    command: `+pwdCmd+`
    output: PARENT_STEP_DIR

  # Step 3: Call subDAG with explicit workingDir
  - name: call_child_with_wd
    call: child_with_wd

  # Step 4: Call subDAG without workingDir
  - name: call_child_no_wd
    call: child_no_wd

---

# SubDAG with explicit workingDir - should use childScripts
name: child_with_wd
shell: `+shell+`
workingDir: `+childScripts+`
steps:
  - name: child_pwd
    command: `+pwdCmd+`

---

# SubDAG without workingDir - should inherit from its DAG file location
name: child_no_wd
shell: `+shell+`
steps:
  - name: child_inherited_pwd
    command: `+pwdCmd+`
`)

	agent := dag.Agent()
	agent.RunSuccess(t)

	// Verify using AssertOutputs for parent steps (uses Contains for path matching)
	dag.AssertOutputs(t, map[string]any{
		"PARENT_DIR":      test.Contains(parentScripts),
		"PARENT_STEP_DIR": test.Contains(childScripts),
	})

	// For subDAG verification, we need to check the sub-run status
	dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
	require.NoError(t, err)
	require.Equal(t, core.Succeeded, dagRunStatus.Status)

	// Create DAGRunRef from the status
	dagRunRef := execution.NewDAGRunRef(dagRunStatus.Name, dagRunStatus.DAGRunID)

	// Find the subDAG nodes and verify their working directories via stdout logs
	for _, node := range dagRunStatus.Nodes {
		switch node.Step.Name {
		case "call_child_with_wd":
			// SubDAG with explicit workingDir should have run in childScripts
			require.Len(t, node.SubRuns, 1, "Expected one sub-run for call_child_with_wd")
			subRunID := node.SubRuns[0].DAGRunID
			subAttempt, err := th.DAGRunStore.FindSubAttempt(th.Context, dagRunRef, subRunID)
			require.NoError(t, err)
			subStatus, err := subAttempt.ReadStatus(th.Context)
			require.NoError(t, err)
			require.Len(t, subStatus.Nodes, 1)
			// Read stdout log to verify working directory
			logContent, err := os.ReadFile(subStatus.Nodes[0].Stdout)
			require.NoError(t, err)
			assert.Contains(t, strings.TrimSpace(string(logContent)), childScripts,
				"SubDAG with explicit workingDir should run in childScripts")

		case "call_child_no_wd":
			// SubDAG without workingDir should NOT inherit parent's workingDir
			require.Len(t, node.SubRuns, 1, "Expected one sub-run for call_child_no_wd")
			subRunID := node.SubRuns[0].DAGRunID
			subAttempt, err := th.DAGRunStore.FindSubAttempt(th.Context, dagRunRef, subRunID)
			require.NoError(t, err)
			subStatus, err := subAttempt.ReadStatus(th.Context)
			require.NoError(t, err)
			require.Len(t, subStatus.Nodes, 1)
			// Read stdout log to verify working directory
			logContent, err := os.ReadFile(subStatus.Nodes[0].Stdout)
			require.NoError(t, err)
			childDir := strings.TrimSpace(string(logContent))
			assert.NotEqual(t, parentScripts, childDir,
				"SubDAG without workingDir should NOT inherit parent's workingDir")
		}
	}
}
