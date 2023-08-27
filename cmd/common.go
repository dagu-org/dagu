package cmd

import (
	"context"
	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/dag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func execDAG(ctx context.Context, cmd *cobra.Command, args []string, dry bool) {
	params, err := cmd.Flags().GetString("params")
	checkError(err)

	loadedDAG, err := loadDAG(args[0], removeQuotes(params))
	checkError(err)

	err = start(ctx, loadedDAG, dry)
	if err != nil {
		log.Fatalf("Failed to start DAG: %v", err)
	}
}

func start(ctx context.Context, d *dag.DAG, dry bool) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: d, Dry: dry}}
	listenSignals(ctx, a)
	return a.Run(ctx)
}

type signalListener interface {
	Signal(os.Signal)
}

var (
	signalChan chan os.Signal
)

func listenSignals(ctx context.Context, a signalListener) {
	go func() {
		signalChan = make(chan os.Signal, 100)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-ctx.Done():
			a.Signal(os.Interrupt)
		case sig := <-signalChan:
			a.Signal(sig)
		}
	}()
}

func checkError(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func removeQuotes(s string) string {
	if len(s) > 1 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
