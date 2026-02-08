package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
	"github.com/dagu-org/dagu/internal/runtime/agent"
	"github.com/spf13/cobra"
)

var dryFlags = []commandLineFlag{paramsFlag, nameFlag, namespaceFlag}

// Dry returns the cobra command for dry-run simulation.
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

// runDry executes a dry-run simulation of the specified DAG.
func runDry(ctx *Context, args []string) error {
	namespaceName, dagName, err := ctx.ResolveNamespaceFromArg(args[0])
	if err != nil {
		return err
	}
	args[0] = dagName

	ns, err := ctx.NamespaceStore.Get(ctx, namespaceName)
	if err != nil {
		return fmt.Errorf("failed to get namespace %q: %w", namespaceName, err)
	}

	dag, _, err := loadDAGWithParams(ctx, args, false, ns)
	if err != nil {
		return err
	}

	dag.Namespace = namespaceName

	if err := dag.Validate(); err != nil {
		return fmt.Errorf("validation failed for %s: %w", args[0], err)
	}

	dagRunID, err := genRunID()
	if err != nil {
		return fmt.Errorf("failed to generate dag-run ID: %w", err)
	}

	logFile, err := ctx.OpenLogFile(dag, dagRunID)
	if err != nil {
		return fmt.Errorf("failed to initialize log file for dag-run %s: %w", dag.Name, err)
	}
	defer func() { _ = logFile.Close() }()

	ctx.LogToFile(logFile)

	dagStore, err := ctx.dagStore(dagStoreConfig{
		SearchPaths: []string{filepath.Dir(dag.Location)},
	})
	if err != nil {
		return err
	}

	if err := core.ValidateNamespace(dag.Namespace); err != nil {
		return fmt.Errorf("cannot dry-run DAG: %w", err)
	}

	ag := agent.New(
		dagRunID,
		dag,
		filepath.Dir(logFile.Name()),
		logFile.Name(),
		ctx.DAGRunMgr,
		dagStore,
		agent.Options{
			Dry:             true,
			DAGRunStore:     ctx.DAGRunStore,
			ServiceRegistry: ctx.ServiceRegistry,
			RootDAGRun:      exec.NewDAGRunRef(dag.Name, dagRunID),
			PeerConfig:      ctx.Config.Core.Peer,
		},
	)

	listenSignals(ctx, ag)

	if err := ag.Run(ctx); err != nil {
		return fmt.Errorf("failed to execute dag-run %s (dag-run ID: %s): %w", dag.Name, dagRunID, err)
	}

	ag.PrintSummary(ctx)

	return nil
}

