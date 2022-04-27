package main

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
)

var stdin io.ReadCloser

func main() {
	err := run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func listenSignals(abortFunc func(sig os.Signal)) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigs {
			log.Printf("\nSignal: %v", sig)
			abortFunc(sig)
		}
	}()
}

func run() error {
	stdin = os.Stdin
	app := makeApp()
	return app.Run(os.Args)
}

func makeApp() *cli.App {
	return &cli.App{
		Name:      "dagu",
		Usage:     "A simple command to run workflows (DAGs)",
		UsageText: "dagu [options] <start|status|stop|retry|dry|server> [args]",
		Commands: []*cli.Command{
			newStartCommand(),
			newStatusCommand(),
			newStopCommand(),
			newRetryCommand(),
			newDryCommand(),
			newServerCommand(),
		},
	}
}
