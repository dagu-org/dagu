package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/admin"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/runner"
	"github.com/yohamta/dagu/internal/scheduler"
)

func regisgterCommands(root *cobra.Command) {
	rootCmd.AddCommand(startCommand())
	rootCmd.AddCommand(stopCommand())
	rootCmd.AddCommand(restartCommand())
	rootCmd.AddCommand(dryCommand())
	rootCmd.AddCommand(statusCommand())
	rootCmd.AddCommand(versionCommand())
	rootCmd.AddCommand(serverCommand())
	rootCmd.AddCommand(schedulerCommand())
	rootCmd.AddCommand(retryCommand())
}

func startCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] <DAG file>",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params, err := cmd.Flags().GetString("params")
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], strings.Trim(params, `"`))
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, false))
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}

func dryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			params, err := cmd.Flags().GetString("params")
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], strings.Trim(params, `"`))
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, true))
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}

func restartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <DAG file>",
		Short: "Restart the DAG",
		Long:  `dagu restart <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dagFile := args[0]
			d, err := loadDAG(dagFile, "")
			cobra.CheckErr(err)

			ctrl := controller.NewDAGController(d)

			// Check the current status.
			st, err := ctrl.GetStatus()
			cobra.CheckErr(err)

			// Stop the DAG if it is running.
			if st.Status == scheduler.SchedulerStatus_Running {
				log.Printf("Stopping %s for restart...", d.Name)
				cobra.CheckErr(stopRunningDAG(ctrl))
			}

			// Wait for the specified amount of time before restarting.
			if d.RestartWait > 0 {
				log.Printf("Waiting for %s...", d.RestartWait)
				time.Sleep(d.RestartWait)
			}

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", d.Name)
			st, err = ctrl.GetLastStatus()
			cobra.CheckErr(err)

			// Start the DAG with the same parmaeter.
			d, err = loadDAG(dagFile, st.Params)
			cobra.CheckErr(err)
			cobra.CheckErr(start(d, false))
		},
	}
}

func retryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> <DAG file>",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			f, _ := filepath.Abs(args[0])
			reqID, err := cmd.Flags().GetString("req")
			cobra.CheckErr(err)
			status, err := database.New().FindByRequestId(f, reqID)
			cobra.CheckErr(err)
			d, err := loadDAG(args[0], status.Status.Params)
			cobra.CheckErr(err)
			a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: d},
				RetryConfig: &agent.RetryConfig{Status: status.Status}}
			listenSignals(func(sig os.Signal) { a.Signal(sig) })
			cobra.CheckErr(a.Run())
		},
	}
	cmd.Flags().StringP("req", "r", "", "request-id")
	cmd.MarkFlagRequired("req")
	return cmd
}

func schedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			config.Get().DAGs = getFlagString(cmd, "dags", config.Get().DAGs)
			agent := runner.NewAgent(config.Get())
			listenSignals(func(sig os.Signal) { agent.Stop() })
			cobra.CheckErr(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}

func serverCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		Long:  `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		Run: func(cmd *cobra.Command, args []string) {
			server := admin.NewServer(config.Get())
			listenSignals(func(sig os.Signal) { server.Shutdown() })
			cobra.CheckErr(server.Serve())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	cmd.Flags().StringP("host", "s", "", "server port (default is 8080)")
	cmd.Flags().StringP("port", "p", "", "server host (default is localhost)")

	viper.BindPFlag("port", cmd.Flags().Lookup("port"))
	viper.BindPFlag("host", cmd.Flags().Lookup("host"))
	viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
	return cmd
}

func statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			d, err := loadDAG(args[0], "")
			cobra.CheckErr(err)

			status, err := controller.NewDAGController(d).GetStatus()
			cobra.CheckErr(err)

			res := &models.StatusResponse{Status: status}
			log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
		},
	}
}

func stopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			d, err := loadDAG(args[0], "")
			cobra.CheckErr(err)

			log.Printf("Stopping...")
			cobra.CheckErr(controller.NewDAGController(d).Stop())
		},
	}
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Display the binary version",
		Long:  `dagu version`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(constants.Version)
		},
	}
}

func stopRunningDAG(ctrl *controller.DAGController) error {
	for {
		st, err := ctrl.GetStatus()
		cobra.CheckErr(err)

		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		cobra.CheckErr(ctrl.Stop())
		time.Sleep(time.Millisecond * 100)
	}
}

func start(d *dag.DAG, dry bool) error {
	a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: d, Dry: dry}}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
