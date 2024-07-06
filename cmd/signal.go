package cmd

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

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
