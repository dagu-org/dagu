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
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/settings"
	"github.com/yohamta/dagu/internal/utils"
)

var (
	version     = "0.0.0"
	stdin       io.ReadCloser
	sigs        chan os.Signal
	globalFlags = []cli.Flag{
		&cli.StringFlag{
			Name:     "config",
			Usage:    "Admin config",
			Value:    "",
			Required: false,
		},
	}
)

func main() {
	setVersion()
	err := run()
	if err != nil {
		log.Fatalf("%v", err)
	}
}

func loadGlobalConfig(c *cli.Context) (cfg *admin.Config, err error) {
	l := &admin.Loader{}
	cf := utils.StringWithFallback(c.String("config"), settings.MustGet(settings.SETTING__ADMIN_CONFIG))
	cfg, err = l.LoadAdminConfig(cf)
	if err == admin.ErrConfigNotFound {
		cfg, err = admin.DefaultConfig()
	}
	if err != nil {
		return nil, fmt.Errorf("loading admin config failed: %w", err)
	}
	return cfg, err
}

func loadDAG(c *cli.Context, dagPath, params string) (d *dag.DAG, err error) {
	cfg, err := loadGlobalConfig(c)
	if err != nil {
		return nil, err
	}
	cl := &dag.Loader{BaseConfig: cfg.BaseConfig}
	d, err = cl.Load(dagPath, params)
	return d, err
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
	}
}
