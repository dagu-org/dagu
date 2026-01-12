package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/cmn/stringutil"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/core/spec"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/spf13/cobra"
)

func Dry() *cobra.Command {
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

var dryFlags = []commandLineFlag{paramsFlag, nameFlag}

// runDry executes a dry-run simulation of the DAG named by args[0] using ctx for configuration, logging, and services.
// It loads the DAG with any name override or parameters provided via flags or arguments, generates a dag-run ID,
// initializes a log file for the run, creates an agent configured for dry mode, runs the agent to simulate execution,
// and prints a summary. Any error encountered during these steps is returned.
func runDry(ctx *Context, args []string) error {
	loadOpts := []spec.LoadOption{
		spec.WithBaseConfig(ctx.Config.Paths.BaseConfig),
		spec.WithDAGsDir(ctx.Config.Paths.DAGsDir),
	}

	// Get name override from flags if provided
	nameOverride, err := ctx.StringParam("name")
	if err != nil {
		return fmt.Errorf("failed to get name override: %w", err)
	}
	if nameOverride != "" {
		loadOpts = append(loadOpts, spec.WithName(nameOverride))
	}

	if argsLenAtDash := ctx.Command.ArgsLenAtDash(); argsLenAtDash != -1 {
		// Get parameters from command line arguments after "--"
		loadOpts = append(loadOpts, spec.WithParams(args[argsLenAtDash:]))
	} else {
		// Get parameters from flags
		params, err := ctx.Command.Flags().GetString("params")
		if err != nil {
			return fmt.Errorf("failed to get parameters: %w", err)
		}
		loadOpts = append(loadOpts, spec.WithParams(stringutil.RemoveQuotes(params)))
	}

	dag, err := spec.Load(ctx, args[0], loadOpts...)
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

	root := exec.NewDAGRunRef(dag.Name, dagRunID)

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
		ctx.Config.Core.Peer,
		agent.Options{Dry: true},
	)

	listenSignals(ctx, agentInstance)

	if err := agentInstance.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute the dag-run %s (dag-run ID: %s): %w", dag.Name, dagRunID, err)
	}

	agentInstance.PrintSummary(ctx)

	return nil
}
