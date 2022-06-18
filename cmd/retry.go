/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yohamta/dagu/agent"
	"github.com/yohamta/dagu/internal/config"
	"github.com/yohamta/dagu/internal/database"
	"github.com/yohamta/dagu/internal/utils"
)

var req = ""

// retryCmd represents the retry command
var retryCmd = &cobra.Command{
	Use:   "retry",
	Short: "dagu retry --req=<request-id> <config>",
	RunE: func(cmd *cobra.Command, args []string) error {
		f, _ := filepath.Abs(args[0])
		requestId := req
		return retry(f, requestId)
	},
}

func init() {
	retryCmd.Flags().StringVar(&req, "req", "", "request-id")
	retryCmd.MarkFlagRequired("req")
	rootCmd.AddCommand(retryCmd)
}

func retry(f, requestId string) error {
	cl := &config.Loader{
		HomeDir: utils.MustGetUserHomeDir(),
	}

	db := database.New(database.DefaultConfig())
	status, err := db.FindByRequestId(f, requestId)
	if err != nil {
		return err
	}

	cfg, err := cl.Load(f, status.Status.Params)
	if err != nil {
		return err
	}

	a := &agent.Agent{
		AgentConfig: &agent.AgentConfig{
			DAG: cfg,
			Dry: false,
		},
		RetryConfig: &agent.RetryConfig{
			Status: status.Status,
		},
	}

	listenSignals(func(sig os.Signal) {
		a.Signal(sig)
	})

	return a.Run()
}
