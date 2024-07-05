package cmd

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dagu-dev/dagu/internal/agent"
	"github.com/dagu-dev/dagu/internal/config"
	"github.com/dagu-dev/dagu/internal/dag"
	"github.com/dagu-dev/dagu/internal/engine"
	"github.com/spf13/cobra"
)

// newFrontendApp initializes the web server that serves the frontend.
// runDAG reads the DAG file and executes it with the given parameters.
func runDAG(
	ctx context.Context,
	cfg *config.Config,
	eng engine.Engine,
	cmd *cobra.Command,
	args []string,
	dry bool,
) {
	params, err := cmd.Flags().GetString("params")
	if err != nil {
		log.Fatalf("Failed to get params: %v", err)
	}

	dg, err := loadDAG(cfg, args[0], removeQuotes(params))
	if err != nil {
		log.Fatalf("Failed to load DAG: %v", err)
	}

	if err := start(ctx, cfg, eng, dg, dry); err != nil {
		log.Fatalf("Failed to start DAG: %v", err) // nolint // deep-exit
	}
}

// start is responsible for actually starting the DAG.
func start(
	ctx context.Context,
	cfg *config.Config,
	eng engine.Engine,
	dg *dag.DAG,
	dry bool,
) error {
	dagAgent := agent.New(
		&agent.Config{DAG: dg, Dry: dry}, eng, newDataStoreFactory(cfg),
	)
	listenSignals(ctx, dagAgent)
	return dagAgent.Run(ctx)
}

type signalListener interface {
	Signal(os.Signal)
}

var signalChan = make(chan os.Signal, 100)

// listenSignals subscribes to the OS signals and passes them to the listener.
// It listens for the context cancellation as well.
func listenSignals(ctx context.Context, listener signalListener) {
	go func() {
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			listener.Signal(os.Interrupt)
		case sig := <-signalChan:
			listener.Signal(sig)
		}
	}()
}

// removeQuotes removes the surrounding quotes from the string.
func removeQuotes(s string) string {
	if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
