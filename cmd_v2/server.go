package cmd_v2

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/internal/admin"
)

func serverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		Long:  `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		Run: func(cmd *cobra.Command, args []string) {
			cfg.DAGs = getFlagString(cmd, "dags", cfg.DAGs)
			cfg.Host = getFlagString(cmd, "host", cfg.Host)
			cfg.Port = getFlagString(cmd, "port", cfg.Port)
			server := admin.NewServer(cfg)
			listenSignals(func(sig os.Signal) { server.Shutdown() })
			cobra.CheckErr(server.Serve())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "DAGs dir")
	cmd.Flags().StringP("host", "s", "", "host")
	cmd.Flags().StringP("port", "p", "", "port")
	return cmd
}
