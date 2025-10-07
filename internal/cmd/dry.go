package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/stringutil"
	"github.com/spf13/cobra"
)

func CmdDry() *cobra.Command {
	return NewCommand(
		&cobra.Command{
			Use:   "dry [flags] <DAG definition> [-- param1 param2 ...]",
			Short: "Simulate a DAG-run without executing actual commands",
			Long: `Perform a dry-run simulation of a DAG-run without executing any real actions.

This command simulates the DAG-run based on the provided DAG definition,
without producing any side effects or running actual commands.

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
		loadOpts = append(loadOpts, digraph.WithParams(stringutil.RemoveQuotes(params)))
	}

	dag, err := digraph.Load(ctx, args[0], loadOpts...)
	if err != nil {
		return fmt.Errorf("failed to load DAG from %s: %w", args[0], err)
	}

	dagRunID, err := genRunID()
	if err != nil {
		return fmt.Errorf("failed to generate dag-run ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for dag-run %s: %w", dag.Name, err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	ctx.LogToFile(logFile)

	dr, err := ctx.dagStore(nil, []string{filepath.Dir(dag.Location)})
	if err != nil {
		return err
	}

	root := digraph.NewDAGRunRef(dag.Name, dagRunID)

	agentInstance := agent.New(
		dagRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dr,
		ctx.DAGRunStore,
		ctx.ServiceRegistry,
		root,
		ctx.Config.Global.Peer,
		agent.Options{Dry: true},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute the dag-run %s (dag-run ID: %s): %w", dag.Name, dagRunID, err)
	}

	agentInstance.PrintSummary(ctx)

	return nil
}
