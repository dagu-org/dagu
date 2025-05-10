package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/spf13/cobra"
)

func CmdDry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dry [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Simulate a workflow without executing actual commands",
			Long: `Perform a dry-run simulation of a workflow without executing any real actions.

This command processes a DAG definition and simulates the entire workflow execution,
showing the execution plan, step dependencies, and configuration. It validates the
workflow structure and parameters without producing any side effects or running
actual commands.

Parameters after the "--" separator are passed as execution parameters (either positional or key=value pairs),
allowing you to test different parameter configurations.

Example:
  dagu dry my_dag.yaml -- P1=foo P2=bar
`,
			Args: cobra.MinimumNArgs(1),
		}, dryFlags,
		runDry,
	)
}

var dryFlags = []commandLineFlag{paramsFlag}

func runDry(ctx *Context, args []string) error {
	loadOpts := []digraph.LoadOption{
		digraph.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		digraph.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, digraph.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err := ctx.Command.Flags().GetString("params")
		if err != nil {
			return fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, digraph.WithParams(removeQuotes(params)))
	}

	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	workflowID, err := getWorkflowID()
	if err != nil {
		return fmt.Errorf("failed to generate workflow ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, workflowID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for workflow %s: %w", dag.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	dr, err := ctx.dagRepo(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return err
	}

	root := digraph.NewWorkflowRef(dag.Name, workflowID)

	agentInstance := agent.New(
		workflowID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.HistoryMgr,
		dr,
		ctx.HistoryRepo,
		root,
		agent.Options{Dry: true},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute the workflow %s (workflow ID: %s): %w", dag.Name, workflowID, err)
	}

	agentInstance.PrintSummary(ctx)

	return nil
}
