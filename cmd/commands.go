package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yohamta/dagu/internal/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/constants"
	"github.com/yohamta/dagu/internal/controller"
	"github.com/yohamta/dagu/internal/dag"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/models"
	"github.com/yohamta/dagu/internal/runner"
	"github.com/yohamta/dagu/internal/scheduler"
	"github.com/yohamta/dagu/internal/web"
)

func registerCommands(root *cobra.Command) {
	rootCmd.AddCommand(createStartCommand())
	rootCmd.AddCommand(createStopCommand())
	rootCmd.AddCommand(createRestartCommand())
	rootCmd.AddCommand(createDryCommand())
	rootCmd.AddCommand(createStatusCommand())
	rootCmd.AddCommand(createVersionCommand())
	rootCmd.AddCommand(createServerCommand())
	rootCmd.AddCommand(createSchedulerCommand())
	rootCmd.AddCommand(createRetryCommand())
}

func createStartCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start [flags] <DAG file>",
		Short: "Runs the DAG",
		Long:  `dagu start [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			executeDAGCommand(cmd.Context(), cmd, args, false)
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}

func createDryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dry [flags] <DAG file>",
		Short: "Dry-runs specified DAG",
		Long:  `dagu dry [--params="param1 param2"] <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			executeDAGCommand(cmd.Context(), cmd, args, true)
		},
	}
	cmd.Flags().StringP("params", "p", "", "parameters")
	return cmd
}

func createRestartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <DAG file>",
		Short: "Restart the DAG",
		Long:  `dagu restart <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			dagFile := args[0]
			loadedDAG, err := loadDAG(dagFile, "")
			checkError(err)

			ctrl := controller.NewDAGController(loadedDAG)

			// Check the current status and stop the DAG if it is running.
			stopDAGIfRunning(ctrl)

			// Wait for the specified amount of time before restarting.
			waitForRestart(loadedDAG.RestartWait)

			// Retrieve the parameter of the previous execution.
			log.Printf("Restarting %s...", loadedDAG.Name)
			params := getPreviousExecutionParams(ctrl)

			// Start the DAG with the same parameter.
			loadedDAG, err = loadDAG(dagFile, params)
			checkError(err)
			cobra.CheckErr(start(cmd.Context(), loadedDAG, false))
		},
	}
}

func stopDAGIfRunning(ctrl *controller.DAGController) {
	st, err := ctrl.GetStatus()
	checkError(err)

	// Stop the DAG if it is running.
	if st.Status == scheduler.SchedulerStatus_Running {
		log.Printf("Stopping %s for restart...", ctrl.DAG.Name)
		cobra.CheckErr(stopRunningDAG(ctrl))
	}
}

func waitForRestart(restartWait time.Duration) {
	if restartWait > 0 {
		log.Printf("Waiting for %s...", restartWait)
		time.Sleep(restartWait)
	}
}

func getPreviousExecutionParams(ctrl *controller.DAGController) string {
	st, err := ctrl.GetLastStatus()
	checkError(err)

	return st.Params
}

func createRetryCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "retry --req=<request-id> <DAG file>",
		Short: "Retry the DAG execution",
		Long:  `dagu retry --req=<request-id> <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			f, _ := filepath.Abs(args[0])
			reqID, err := cmd.Flags().GetString("req")
			checkError(err)

			status, err := database.New().FindByRequestId(f, reqID)
			checkError(err)

			loadedDAG, err := loadDAG(args[0], status.Status.Params)
			checkError(err)

			a := &agent.Agent{AgentConfig: &agent.AgentConfig{DAG: loadedDAG},
				RetryConfig: &agent.RetryConfig{Status: status.Status}}
			ctx := cmd.Context()
			go listenSignals(ctx, a)
			checkError(a.Run(ctx))
		},
	}
	cmd.Flags().StringP("req", "r", "", "request-id")
	cmd.MarkFlagRequired("req")
	return cmd
}

func createSchedulerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduler",
		Short: "Start the scheduler",
		Long:  `dagu scheduler [--dags=<DAGs dir>]`,
		Run: func(cmd *cobra.Command, args []string) {
			config.Get().DAGs = getFlagString(cmd, "dags", config.Get().DAGs)
			agent := runner.NewAgent(config.Get())
			go listenSignals(cmd.Context(), agent)
			checkError(agent.Start())
		},
	}
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))

	return cmd
}

func createServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Start the server",
		Long:  `dagu server [--dags=<DAGs dir>] [--host=<host>] [--port=<port>]`,
		Run: func(cmd *cobra.Command, args []string) {
			server := web.NewServer(config.Get())
			go listenSignals(cmd.Context(), server)
			checkError(server.Serve())
		},
	}
	bindServerCommandFlags(cmd)
	return cmd
}

func bindServerCommandFlags(cmd *cobra.Command) {
	cmd.Flags().StringP("dags", "d", "", "location of DAG files (default is $HOME/.dagu/dags)")
	cmd.Flags().StringP("host", "s", "", "server host (default is localhost)")
	cmd.Flags().StringP("port", "p", "", "server port (default is 8080)")

	viper.BindPFlag("port", cmd.Flags().Lookup("port"))
	viper.BindPFlag("host", cmd.Flags().Lookup("host"))
	viper.BindPFlag("dags", cmd.Flags().Lookup("dags"))
}

func createStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status <DAG file>",
		Short: "Display current status of the DAG",
		Long:  `dagu status <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			loadedDAG, err := loadDAG(args[0], "")
			checkError(err)

			status, err := controller.NewDAGController(loadedDAG).GetStatus()
			checkError(err)

			res := &models.StatusResponse{Status: status}
			log.Printf("Pid=%d Status=%s", res.Status.Pid, res.Status.Status)
		},
	}
}

func createStopCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <DAG file>",
		Short: "Stop the running DAG",
		Long:  `dagu stop <DAG file>`,
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			loadedDAG, err := loadDAG(args[0], "")
			checkError(err)

			log.Printf("Stopping...")
			checkError(controller.NewDAGController(loadedDAG).Stop())
		},
	}
}

func createVersionCommand() *cobra.Command {
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
		checkError(err)

		if st.Status != scheduler.SchedulerStatus_Running {
			return nil
		}
		checkError(ctrl.Stop())
		time.Sleep(time.Millisecond * 100)
	}
}

func executeDAGCommand(ctx context.Context, cmd *cobra.Command, args []string, dry bool) {
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
	go listenSignals(ctx, a)
	return a.Run(ctx)
}

type signalListener interface {
	Signal(os.Signal)
}

var (
	signalChan chan os.Signal
)

func listenSignals(ctx context.Context, a signalListener) error {
	signalChan = make(chan os.Signal, 100)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
		a.Signal(os.Interrupt)
		return ctx.Err()
	case sig := <-signalChan:
		a.Signal(sig)
		return fmt.Errorf("received signal: %v", sig)
	}
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
