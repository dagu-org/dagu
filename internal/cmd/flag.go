package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Default values for the server.
const (
	defaultHost = "localhost"
	defaultPort = "8080"
)

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
}

var (
	configFlag = commandLineFlag{
		name:      "config",
		shorthand: "c",
		usage:     "config file (default is $HOME/.config/dagu/config.yaml)",
	}
	dagsFlag = commandLineFlag{
		name:      "dags",
		shorthand: "d",
		usage:     "location of DAG files (default is $HOME/.config/dagu/dags)",
	}
	hostFlag = commandLineFlag{
		name:         "host",
		shorthand:    "s",
		defaultValue: defaultHost,
		usage:        "server host",
	}
	portFlag = commandLineFlag{
		name:         "port",
		shorthand:    "p",
		defaultValue: defaultPort,
		usage:        "server port",
	}
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "parameters to pass to the DAG",
	}
	requestIDFlagRetry = commandLineFlag{
		name:      "request-id",
		shorthand: "r",
		usage:     "request ID (required)",
		required:  true,
	}
	requestIDFlagStart = commandLineFlag{
		name:      "request-id",
		shorthand: "r",
		usage:     "request ID for the DAG execution",
		required:  true,
	}
)

func initCommonFlags(cmd *cobra.Command, addFlags []commandLineFlag) {
	addFlags = append(addFlags, configFlag)
	for _, flag := range addFlags {
		cmd.Flags().StringP(flag.name, flag.shorthand, flag.defaultValue, flag.usage)
		if flag.required {
			if err := cmd.MarkFlagRequired(flag.name); err != nil {
				fmt.Printf("failed to mark flag %s as required: %v\n", flag.name, err)
			}
		}
	}
}

func bindCommonFlags(cmd *cobra.Command, addFlags []string) error {
	flags := []string{"config"}
	flags = append(flags, addFlags...)
	for _, flag := range flags {
		if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
			return fmt.Errorf("failed to bind flag %s: %w", flag, err)
		}
	}
	return nil
}
