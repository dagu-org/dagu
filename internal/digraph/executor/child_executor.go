package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
)

// ChildDAGExecutor is a helper for executing child DAGs.
// It handles both regular DAGs and local DAGs (defined in the same file).
type ChildDAGExecutor struct {
	// DAG is the child DAG to execute.
	// For local DAGs, this DAG's Location will be set to a temporary file.
	DAG *digraph.DAG

	// tempFile holds the temporary file path for local DAGs.
	// This will be cleaned up after execution.
	tempFile string

	// isLocal indicates whether this is a local DAG.
	isLocal bool
}

// NewChildDAGExecutor creates a new ChildDAGExecutor.
// It handles the logic for finding the DAG - either from the database
// or from local DAGs defined in the parent.
func NewChildDAGExecutor(ctx context.Context, childName string) (*ChildDAGExecutor, error) {
	env := GetEnv(ctx)

	// First, check if it's a local DAG in the parent
	if env.DAG != nil && env.DAG.LocalDAGs != nil {
		if localDAG, ok := env.DAG.LocalDAGs[childName]; ok {
			// Create a temporary file for the local DAG
			tempFile, err := createTempDAGFile(childName, localDAG.YamlData)
			if err != nil {
				return nil, fmt.Errorf("failed to create temp file for local DAG: %w", err)
			}

			// Set the location to the temporary file
			dag := localDAG.DAG
			dag.Location = tempFile

			return &ChildDAGExecutor{
				DAG:      dag,
				tempFile: tempFile,
				isLocal:  true,
			}, nil
		}
	}

	// If not found as local DAG, look it up in the database
	dag, err := env.DB.GetDAG(ctx, childName)
	if err != nil {
		return nil, fmt.Errorf("failed to find DAG %q: %w", childName, err)
	}

	return &ChildDAGExecutor{
		DAG:     dag,
		isLocal: false,
	}, nil
}

// BuildCommand builds the command to execute the child DAG.
func (e *ChildDAGExecutor) BuildCommand(
	ctx context.Context,
	runParams RunParams,
	workDir string,
) (*exec.Cmd, error) {
	executable, err := executablePath()
	if err != nil {
		return nil, fmt.Errorf("failed to find executable path: %w", err)
	}

	if runParams.RunID == "" {
		return nil, fmt.Errorf("dag-run ID is not set")
	}

	env := GetEnv(ctx)
	if env.RootDAGRun.Zero() {
		return nil, fmt.Errorf("root dag-run ID is not set")
	}

	args := []string{
		"start",
		fmt.Sprintf("--root=%s", env.RootDAGRun.String()),
		fmt.Sprintf("--parent=%s", env.DAGRunRef().String()),
		fmt.Sprintf("--run-id=%s", runParams.RunID),
		"--no-queue",
		e.DAG.Location,
	}

	if configFile := config.UsedConfigFile.Load(); configFile != nil {
		if configFile, ok := configFile.(string); ok {
			args = append(args, "--config", configFile)
		}
	}

	if runParams.Params != "" {
		args = append(args, "--", runParams.Params)
	}

	cmd := exec.CommandContext(ctx, executable, args...) // nolint:gosec
	cmd.Dir = workDir
	cmd.Env = append(cmd.Env, env.AllEnvs()...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	logger.Info(ctx, "Prepared child DAG command",
		"dagRunId", runParams.RunID,
		"target", e.DAG.Name,
		"args", args,
		"isLocal", e.isLocal,
	)

	return cmd, nil
}

// Cleanup removes any temporary files created for local DAGs.
// This should be called after the child DAG execution is complete.
func (e *ChildDAGExecutor) Cleanup(ctx context.Context) error {
	if e.tempFile == "" {
		return nil
	}

	logger.Info(ctx, "Cleaning up temporary DAG file",
		"dag", e.DAG.Name,
		"tempFile", e.tempFile,
	)

	if err := os.Remove(e.tempFile); err != nil && !os.IsNotExist(err) {
		logger.Error(ctx, "Failed to remove temporary DAG file",
			"dag", e.DAG.Name,
			"tempFile", e.tempFile,
			"error", err,
		)
		return fmt.Errorf("failed to remove temp file: %w", err)
	}

	return nil
}

// createTempDAGFile creates a temporary file with the DAG YAML content.
func createTempDAGFile(dagName string, yamlData []byte) (string, error) {
	// Create a temporary directory if it doesn't exist
	tempDir := filepath.Join(os.TempDir(), "dagu", "local-dags")
	if err := os.MkdirAll(tempDir, 0750); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a temporary file with a meaningful name
	pattern := fmt.Sprintf("%s-*.yaml", dagName)
	tempFile, err := os.CreateTemp(tempDir, pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		_ = tempFile.Close()
	}()

	// Write the YAML data
	if _, err := tempFile.Write(yamlData); err != nil {
		_ = os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to write YAML data: %w", err)
	}

	return tempFile.Name(), nil
}

// executablePath returns the path to the dagu executable.
func executablePath() (string, error) {
	if os.Getenv("DAGU_EXECUTABLE") != "" {
		return os.Getenv("DAGU_EXECUTABLE"), nil
	}
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}
	return executable, nil
}