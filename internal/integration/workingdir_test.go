package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dagu-org/dagu/internal/cmd"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/dagu-org/dagu/internal/test"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	t.Parallel()

	th := test.Setup(t)

	t.Run("DAGLevelAbsoluteWorkingDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - command: pwd
    output: WORK_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"WORK_DIR": tempDir,
		})
	})

	t.Run("StepAbsoluteDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		stepDir := filepath.Join(tempDir, "step")
		require.NoError(t, os.MkdirAll(stepDir, 0755))

		dag := th.DAG(t, `
steps:
  - name: step1
    dir: `+stepDir+`
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": stepDir,
		})
	})

	t.Run("StepRelativeDirResolvesAgainstDAGWorkDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: step1
    dir: ./subdir
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": subDir,
		})
	})

	t.Run("StepRelativeDirWithoutLeadingDot", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "scripts")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: step1
    dir: scripts
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": subDir,
		})
	})

	t.Run("StepInheritsDAGWorkDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: step1
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": tempDir,
		})
	})

	t.Run("StepParentRelativeDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "subdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		dag := th.DAG(t, `
workingDir: `+subDir+`
steps:
  - name: step1
    dir: ..
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": tempDir,
		})
	})

	t.Run("StepDirWithEnvVarExpansion", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		subDir := filepath.Join(tempDir, "mysubdir")
		require.NoError(t, os.MkdirAll(subDir, 0755))

		dag := th.DAG(t, `
workingDir: `+tempDir+`
env:
  - MY_SUBDIR: mysubdir
steps:
  - name: step1
    dir: ./$MY_SUBDIR
    command: pwd
    output: STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"STEP_DIR": subDir,
		})
	})

	t.Run("MultipleStepsWithDifferentDirs", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		dir1 := filepath.Join(tempDir, "dir1")
		dir2 := filepath.Join(tempDir, "dir2")
		require.NoError(t, os.MkdirAll(dir1, 0755))
		require.NoError(t, os.MkdirAll(dir2, 0755))

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: step1
    dir: ./dir1
    command: pwd
    output: DIR1
  - name: step2
    dir: ./dir2
    command: pwd
    output: DIR2
  - name: step3
    command: pwd
    output: DIR3
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"DIR1": dir1,
			"DIR2": dir2,
			"DIR3": tempDir, // Inherits DAG workingDir
		})
	})

	t.Run("WorkingDirCreatedIfNotExists", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		newDir := filepath.Join(tempDir, "newdir")
		// Don't create the directory - it should be created automatically

		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: step1
    dir: `+newDir+`
    command: pwd
    output: NEW_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)
		dag.AssertOutputs(t, map[string]any{
			"NEW_DIR": newDir,
		})

		// Verify directory was created
		info, err := os.Stat(newDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())
	})
}

func TestWorkingDirectoryWithLocalSubDAG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	t.Parallel()

	th := test.Setup(t)

	t.Run("SubDAGInheritsParentWorkDir", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()

		// Local sub DAG without explicit workingDir should run in parent's workingDir
		dag := th.DAG(t, `
workingDir: `+tempDir+`
steps:
  - name: call_sub
    call: child
    output: SUB_OUT

---

name: child
steps:
  - command: pwd
    output: CHILD_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		// The sub DAG should have run in the parent's workingDir
		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, dagRunStatus.Status)
	})

	t.Run("SubDAGWithOwnWorkDir", func(t *testing.T) {
		t.Parallel()

		parentDir := t.TempDir()
		childDir := t.TempDir()

		dag := th.DAG(t, `
workingDir: `+parentDir+`
steps:
  - name: parent_step
    command: pwd
    output: PARENT_DIR
  - name: call_sub
    call: child
    output: SUB_OUT

---

name: child
workingDir: `+childDir+`
steps:
  - command: pwd
    output: CHILD_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dag.AssertOutputs(t, map[string]any{
			"PARENT_DIR": parentDir,
		})
	})

	t.Run("SubDAGStepRelativeDir", func(t *testing.T) {
		t.Parallel()

		parentDir := t.TempDir()
		childDir := t.TempDir()
		childSubDir := filepath.Join(childDir, "scripts")
		require.NoError(t, os.MkdirAll(childSubDir, 0755))

		dag := th.DAG(t, `
workingDir: `+parentDir+`
steps:
  - name: call_sub
    call: child
    output: SUB_OUT

---

name: child
workingDir: `+childDir+`
steps:
  - name: child_step
    dir: ./scripts
    command: pwd
    output: CHILD_STEP_DIR
`)
		agent := dag.Agent()
		agent.RunSuccess(t)

		dagRunStatus, err := th.DAGRunMgr.GetLatestStatus(th.Context, dag.DAG)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded, dagRunStatus.Status)
	})
}

func TestWorkingDirectoryWithFileSubDAG(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	th := test.SetupCommand(t)

	t.Run("SubDAGFileWithOwnWorkDir", func(t *testing.T) {
		parentDir := t.TempDir()
		childDir := t.TempDir()

		th.CreateDAGFile(t, "parent_workdir.yaml", `
workingDir: `+parentDir+`
steps:
  - name: parent_step
    command: pwd
    output: PARENT_DIR
  - name: call_sub
    call: child_workdir
    output: SUB_OUT
`)

		th.CreateDAGFile(t, "child_workdir.yaml", `
workingDir: `+childDir+`
steps:
  - name: child_step
    command: pwd
    output: CHILD_DIR
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		args := []string{"start", "--run-id", dagRunID, "parent_workdir"}
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})

		// Verify parent DAG completed successfully
		ctx := context.Background()
		ref := execution.NewDAGRunRef("parent_workdir", dagRunID)
		parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded.String(), parentStatus.Status.String())

		// Verify parent step ran in parentDir
		parentStepNode := parentStatus.Nodes[0]
		require.Equal(t, "parent_step", parentStepNode.Step.Name)
		require.Contains(t, parentStepNode.OutputVariables.Variables(), "PARENT_DIR")
		require.Equal(t, parentDir, parentStepNode.OutputVariables.Variables()["PARENT_DIR"])

		// Verify child DAG ran in childDir
		subNode := parentStatus.Nodes[1]
		require.Equal(t, "call_sub", subNode.Step.Name)
		require.Equal(t, core.NodeSucceeded, subNode.Status)

		subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		subStatus, err := subAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		childStepNode := subStatus.Nodes[0]
		require.Contains(t, childStepNode.OutputVariables.Variables(), "CHILD_DIR")
		require.Equal(t, childDir, childStepNode.OutputVariables.Variables()["CHILD_DIR"])
	})

	t.Run("SubDAGStepRelativeDirResolvesAgainstSubDAGWorkDir", func(t *testing.T) {
		parentDir := t.TempDir()
		childDir := t.TempDir()
		childSubDir := filepath.Join(childDir, "scripts")
		require.NoError(t, os.MkdirAll(childSubDir, 0755))

		th.CreateDAGFile(t, "parent_rel.yaml", `
workingDir: `+parentDir+`
steps:
  - name: call_sub
    call: child_rel
    output: SUB_OUT
`)

		th.CreateDAGFile(t, "child_rel.yaml", `
workingDir: `+childDir+`
steps:
  - name: child_step
    dir: ./scripts
    command: pwd
    output: CHILD_STEP_DIR
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		args := []string{"start", "--run-id", dagRunID, "parent_rel"}
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})

		// Verify
		ctx := context.Background()
		ref := execution.NewDAGRunRef("parent_rel", dagRunID)
		parentAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		subNode := parentStatus.Nodes[0]
		subAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, subNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		subStatus, err := subAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		childStepNode := subStatus.Nodes[0]
		require.Contains(t, childStepNode.OutputVariables.Variables(), "CHILD_STEP_DIR")
		require.Equal(t, childSubDir, childStepNode.OutputVariables.Variables()["CHILD_STEP_DIR"])
	})

	t.Run("NestedSubDAGsWithWorkDir", func(t *testing.T) {
		grandparentDir := t.TempDir()
		parentDir := t.TempDir()
		childDir := t.TempDir()

		th.CreateDAGFile(t, "grandparent.yaml", `
workingDir: `+grandparentDir+`
steps:
  - name: gp_step
    command: pwd
    output: GP_DIR
  - name: call_parent
    call: parent_nested
`)

		th.CreateDAGFile(t, "parent_nested.yaml", `
workingDir: `+parentDir+`
steps:
  - name: p_step
    command: pwd
    output: P_DIR
  - name: call_child
    call: child_nested
`)

		th.CreateDAGFile(t, "child_nested.yaml", `
workingDir: `+childDir+`
steps:
  - name: c_step
    command: pwd
    output: C_DIR
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		args := []string{"start", "--run-id", dagRunID, "grandparent"}
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})

		// Verify grandparent completed
		ctx := context.Background()
		ref := execution.NewDAGRunRef("grandparent", dagRunID)
		gpAttempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		gpStatus, err := gpAttempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded.String(), gpStatus.Status.String())

		// Verify grandparent step ran in grandparentDir
		gpStepNode := gpStatus.Nodes[0]
		require.Equal(t, grandparentDir, gpStepNode.OutputVariables.Variables()["GP_DIR"])

		// Verify parent DAG
		parentSubNode := gpStatus.Nodes[1]
		parentAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, parentSubNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		parentStatus, err := parentAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		pStepNode := parentStatus.Nodes[0]
		require.Equal(t, parentDir, pStepNode.OutputVariables.Variables()["P_DIR"])

		// Verify child DAG
		childSubNode := parentStatus.Nodes[1]
		childAttempt, err := th.DAGRunStore.FindSubAttempt(ctx, ref, childSubNode.SubRuns[0].DAGRunID)
		require.NoError(t, err)

		childStatus, err := childAttempt.ReadStatus(ctx)
		require.NoError(t, err)

		cStepNode := childStatus.Nodes[0]
		require.Equal(t, childDir, cStepNode.OutputVariables.Variables()["C_DIR"])
	})
}

func TestWorkingDirectoryWithRelativeDAGFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping on Windows - uses bash commands")
	}

	th := test.SetupCommand(t)

	t.Run("RelativeWorkDirResolvesAgainstDAGFileLocation", func(t *testing.T) {
		// Create a subdirectory structure
		dagsDir := th.Config.Paths.DAGsDir
		scriptsDir := filepath.Join(dagsDir, "scripts")
		require.NoError(t, os.MkdirAll(scriptsDir, 0755))

		th.CreateDAGFile(t, "rel_workdir.yaml", `
workingDir: ./scripts
steps:
  - name: step1
    command: pwd
    output: WORK_DIR
`)

		dagRunID := uuid.Must(uuid.NewV7()).String()
		args := []string{"start", "--run-id", dagRunID, "rel_workdir"}
		th.RunCommand(t, cmd.Start(), test.CmdTest{
			Args:        args,
			ExpectedOut: []string{"DAG run finished"},
		})

		// Verify
		ctx := context.Background()
		ref := execution.NewDAGRunRef("rel_workdir", dagRunID)
		attempt, err := th.DAGRunStore.FindAttempt(ctx, ref)
		require.NoError(t, err)

		status, err := attempt.ReadStatus(ctx)
		require.NoError(t, err)
		require.Equal(t, core.Succeeded.String(), status.Status.String())

		stepNode := status.Nodes[0]
		require.Contains(t, stepNode.OutputVariables.Variables(), "WORK_DIR")
		require.Equal(t, scriptsDir, stepNode.OutputVariables.Variables()["WORK_DIR"])
	})
}
