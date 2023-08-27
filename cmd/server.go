package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/web"
	"net"
	"testing"
)

func serverCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		Long:  `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		Run: func(cmd *cobra.Command, args []string) {
			server := web.NewServer(config.Get())
			listenSignals(cmd.Context(), server)
			checkError(server.Serve())
		},
	}
	bindServerCommandFlags(cmd)
	return cmd
}

func findPort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	return fmt.Sprintf("%d", port)
}

func bindServerCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	cmd.Flags().StringP("host", "s", "", "server host (default is localhost)")
	cmd.Flags().StringP("port", "p", "", "server port (default is 8080)")

	_ = viper.BindPFlag("port", cmd.Flags().Lookup("port"))
	_ = viper.BindPFlag("host", cmd.Flags().Lookup("host"))
	_ = viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
}
