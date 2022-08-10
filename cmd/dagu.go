package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/urfave/cli/v2"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/settings"
)

var (
	version      = "0.0.0"
	stdin        io.ReadCloser
	sigs         chan os.Signal
	globalConfig *admin.Config
)

func main() {
	setVersion()
	err := run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func loadDAG(dagPath, params string) (cfg *config.Config, err error) {
	cl := &config.Loader{BaseConfig: globalConfig.BaseConfig}
	cfg, err = cl.Load(dagPath, params)
	return
}

func listenSignals(abortFunc func(sig os.Signal)) {
	sigs = make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		for sig := range sigs {
			log.Printf("\nGot signal: %v", sig)
			abortFunc(sig)
		}
	}()
}

func setVersion() {
	constants.Version = version
}

func run() error {
	stdin = os.Stdin
	app := makeApp()
	return app.Run(os.Args)
}

func makeApp() *cli.App {
	return &cli.App{
		Name:      "Dagu",
		Usage:     "Self-contained, easy-to-use workflow engine for smaller use cases",
		UsageText: "dagu [options] <start|status|stop|retry|dry|server|scheduler|version> [args]",
		Commands: []*cli.Command{
			newStartCommand(),
			newStatusCommand(),
			newStopCommand(),
			newRetryCommand(),
			newDryCommand(),
			newServerCommand(),
			newSchedulerCommand(),
			newVersionCommand(),
		},
		Before: func(c *cli.Context) error {
			l := &admin.Loader{}
			cfg, err := l.LoadAdminConfig(settings.MustGet(settings.SETTING__ADMIN_CONFIG))
			if err == admin.ErrConfigNotFound {
				cfg = admin.DefaultConfig()
				err = nil
			}
			if err != nil {
				return fmt.Errorf("loading admin config failed: %w", err)
			}
			globalConfig = cfg
			return nil
		},
	}
}
