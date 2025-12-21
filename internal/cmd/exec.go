package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dagu-org/dagu/internal/common/fileutil"
	"github.com/dagu-org/dagu/internal/common/logger"
	"github.com/dagu-org/dagu/internal/common/logger/tag"
	"github.com/dagu-org/dagu/internal/core/execution"
	"github.com/spf13/cobra"
)

const (
	flagEnv          = "env"
	flagDotenv       = "dotenv"
	flagWorkerLabel  = "worker-label"
	flagWorkdir      = "workdir"
	flagShell        = "shell"
	flagBase         = "base"
	flagSingleton    = "singleton"
	defaultStepName  = "main"
	execCommandUsage = "exec [flags] -- <command> [args...]"
)

var (
	execFlags = []commandLineFlag{
		dagRunIDFlag,
		nameFlag,
		queueFlag,
		workdirFlag,
		shellFlag,
		baseFlag,
		singletonFlag,
	}
)

// Exec returns the cobra command for executing inline commands without a DAG spec.
func Exec() *cobra.Command {
	cmd := &cobra.Command{
		Use:   execCommandUsage,
		Short: "Execute a one-off command as a DAG run",
		Long: `Execute a one-off command as a DAG run without creating a DAG YAML file.

Examples:
  dagu exec -- echo "hello world"
  dagu exec --env FOO=bar -- sh -c 'echo $FOO'
  dagu exec --queue nightly --worker-label role=batch -- python nightly.py`,
		Args: cobra.ArbitraryArgs,
	}

	command := NewCommand(cmd, execFlags, runExec)

	command.Flags().StringArrayP(flagEnv, "E", nil, "Environment variable (KEY=VALUE) to include in the run; repeatable")
	command.Flags().StringArray(flagDotenv, nil, "Path to a dotenv file to load before execution; repeatable")
	command.Flags().StringArray(flagWorkerLabel, nil, "Worker label selector (key=value) for distributed execution; repeatable")

	return command
}

// runExec parses flags and arguments and executes the provided command as an inline DAG run,
// either enqueueing it for distributed execution or running it immediately in-process.
// It validates inputs (run-id, working directory, base and dotenv files, env vars, worker labels,
// queue/singleton flags), builds the DAG for the inline command, and chooses between enqueueing
// (when queues/worker labels require it or when max runs are reached) or direct execution.
// ctx provides CLI and application context; args are the command and its arguments.
// Returns an error for validation failures, when a dag-run with the same run-id already exists,
// or if enqueueing/execution fails.
func runExec(ctx *Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("command is required (try: dagu exec -- <command>)")
	}

	runID, err := ctx.StringParam("run-id")
	if err != nil {
		return fmt.Errorf("failed to read run-id flag: %w", err)
	}
	if runID != "" {
		if err := validateRunID(runID); err != nil {
			return err
		}
	} else {
		runID, err = genRunID()
		if err != nil {
			return fmt.Errorf("failed to generate dag-run ID: %w", err)
		}
	}

	nameOverride, err := ctx.StringParam("name")
	if err != nil {
		return fmt.Errorf("failed to read name flag: %w", err)
	}

	workdirFlag, err := ctx.Command.Flags().GetString(flagWorkdir)
	if err != nil {
		return fmt.Errorf("failed to read workdir flag: %w", err)
	}

	var workingDir string
	if workdirFlag != "" {
		workingDir = fileutil.ResolvePathOrBlank(workdirFlag)
	} else {
		workingDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current directory: %w", err)
		}
	}
	info, err := os.Stat(workingDir)
	if err != nil {
		return fmt.Errorf("working directory %q not found: %w", workingDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("working directory %q is not a directory", workingDir)
	}

	shellOverride, err := ctx.Command.Flags().GetString(flagShell)
	if err != nil {
		return fmt.Errorf("failed to read shell flag: %w", err)
	}

	baseConfigFlag, err := ctx.Command.Flags().GetString(flagBase)
	if err != nil {
		return fmt.Errorf("failed to read base flag: %w", err)
	}
	var baseConfig string
	if baseConfigFlag != "" {
		baseConfig = fileutil.ResolvePathOrBlank(baseConfigFlag)
		if baseConfig == "" || !fileutil.FileExists(baseConfig) {
			return fmt.Errorf("base DAG file %q not found", baseConfigFlag)
		}
	}

	envVars, err := ctx.Command.Flags().GetStringArray(flagEnv)
	if err != nil {
		return fmt.Errorf("failed to read env flags: %w", err)
	}
	for _, env := range envVars {
		if !strings.Contains(env, "=") || strings.HasPrefix(env, "=") {
			return fmt.Errorf("invalid --env value %q: expected KEY=VALUE", env)
		}
	}

	dotenvPathsRaw, err := ctx.Command.Flags().GetStringArray(flagDotenv)
	if err != nil {
		return fmt.Errorf("failed to read dotenv flags: %w", err)
	}
	var dotenvPaths []string
	for _, path := range dotenvPathsRaw {
		if strings.TrimSpace(path) == "" {
			continue
		}
		resolved := fileutil.ResolvePathOrBlank(path)
		if resolved == "" || !fileutil.FileExists(resolved) {
			return fmt.Errorf("dotenv file %q not found", path)
		}
		dotenvPaths = append(dotenvPaths, resolved)
	}

	workerLabelPairs, err := ctx.Command.Flags().GetStringArray(flagWorkerLabel)
	if err != nil {
		return fmt.Errorf("failed to read worker-label flags: %w", err)
	}
	workerLabels := make(map[string]string, len(workerLabelPairs))
	for _, pair := range workerLabelPairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		key, value, found := strings.Cut(pair, "=")
		if !found || key == "" || value == "" {
			return fmt.Errorf("invalid worker label %q: expected key=value", pair)
		}
		workerLabels[key] = value
	}

	queueName, err := ctx.Command.Flags().GetString("queue")
	if err != nil {
		return fmt.Errorf("failed to read queue flag: %w", err)
	}

	singleton, err := ctx.Command.Flags().GetBool(flagSingleton)
	if err != nil {
		return fmt.Errorf("failed to read singleton flag: %w", err)
	}

	if len(workerLabels) > 0 {
		if !ctx.Config.Queues.Enabled {
			return fmt.Errorf("worker selector requires queues; enable queues or remove --worker-label")
		}
	}

	opts := ExecOptions{
		Name:          nameOverride,
		CommandArgs:   args,
		ShellOverride: shellOverride,
		WorkingDir:    workingDir,
		Env:           envVars,
		DotenvFiles:   dotenvPaths,
		BaseConfig:    baseConfig,
		Queue:         queueName,
		Singleton:     singleton,
		WorkerLabels:  workerLabels,
	}

	dag, _, err := buildExecDAG(ctx, opts)
	if err != nil {
		return err
	}

	dagRunRef := execution.NewDAGRunRef(dag.Name, runID)

	attempt, _ := ctx.DAGRunStore.FindAttempt(ctx, dagRunRef)
	if attempt != nil {
		return fmt.Errorf("dag-run ID %s already exists for DAG %s", runID, dag.Name)
	}

	logger.Info(ctx, "Executing inline dag-run",
		tag.DAG(dag.Name),
		tag.Command(strings.Join(args, " ")),
		tag.RunID(runID),
	)

	return tryExecuteDAG(ctx, dag, runID, dagRunRef)
}

var (
	workdirFlag = commandLineFlag{
		name:  flagWorkdir,
		usage: "Working directory for executing the command (default: current directory)",
	}
	shellFlag = commandLineFlag{
		name:  flagShell,
		usage: "Override shell binary for the command (default: use DAGU default shell)",
	}
	baseFlag = commandLineFlag{
		name:  flagBase,
		usage: "Path to a base DAG YAML whose defaults are applied before inline overrides",
	}
	singletonFlag = commandLineFlag{
		name:   flagSingleton,
		usage:  "Limit execution to a single active run (sets maxActiveRuns=1)",
		isBool: true,
	}
)
