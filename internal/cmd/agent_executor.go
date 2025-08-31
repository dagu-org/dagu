package cmd

import (
	"fmt"
	"os"

	"github.com/dagu-org/dagu/internal/agent"
	"github.com/dagu-org/dagu/internal/digraph"
	"github.com/dagu-org/dagu/internal/logger"
	"golang.org/x/term"
)

// ExecuteAgent runs an agent with optional progress display and handles common execution logic
func ExecuteAgent(ctx *Context, agentInstance *agent.Agent, dag *digraph.DAG, dagRunID string, logFile *os.File) error {
	// Check if progress display should be enabled
	enableProgress := shouldEnableProgress(ctx)

	// Configure logger for progress display if needed
	if enableProgress {
		configureLoggerForProgress(ctx, logFile)
	} else {
		// Normal logging configuration
		ctx.LogToFile(logFile)
	}

	// Set up signal handling
	listenSignals(ctx, agentInstance)

	// Run the DAG
	if err := agentInstance.Run(ctx); err != nil {
		logger.Error(ctx, "Failed to execute dag-run",
			"dag", dag.Name,
			"dagRunId", dagRunID,
			"err", err)
		if ctx.Proc != nil {
			_ = ctx.Proc.Stop(ctx)
		}
		if ctx.Quiet {
			os.Exit(1)
		} else {
			agentInstance.PrintSummary(ctx)
			// If progress display was enabled, exit directly without returning error
			// to avoid printing "exit status 1" which ruins the UI
			if enableProgress {
				os.Exit(1)
			}
			return fmt.Errorf("failed to execute the dag-run %s (dag-run ID: %s): %w",
				dag.Name, dagRunID, err)
		}
	}

	// Print summary if not quiet
	if !ctx.Quiet {
		agentInstance.PrintSummary(ctx)
	}

	return nil
}

// shouldEnableProgress checks if progress display should be enabled
func shouldEnableProgress(ctx *Context) bool {
	return !ctx.Quiet &&
		os.Getenv("DISABLE_PROGRESS") == "" &&
		isTerminal(os.Stderr)
}

// configureLoggerForProgress temporarily suppresses stderr logging to prevent UI flicker
func configureLoggerForProgress(ctx *Context, logFile *os.File) {
	var opts []logger.Option
	if ctx.Config.Global.Debug {
		opts = append(opts, logger.WithDebug())
	}
	opts = append(opts, logger.WithQuiet()) // Suppress stderr output
	if ctx.Config.Global.LogFormat != "" {
		opts = append(opts, logger.WithFormat(ctx.Config.Global.LogFormat))
	}
	if logFile != nil {
		opts = append(opts, logger.WithWriter(logFile))
	}
	ctx.Context = logger.WithLogger(ctx.Context, logger.NewLogger(opts...))
}

// isTerminal checks if the given file is a terminal
func isTerminal(f *os.File) bool {
	return term.IsTerminal(int(f.Fd()))
}
