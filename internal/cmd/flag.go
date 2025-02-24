package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Path to the configuration file.
	// If not provided, the default is "$HOME/.config/dagu/config.yaml".
	configFlag = commandLineFlag{
		name:      "config",
		shorthand: "c",
		usage:     "Path to the configuration file (default: $HOME/.config/dagu/config.yaml)",
	}

	// Directory where DAG definition files are stored.
	// If not provided, the default is "$HOME/.config/dagu/dags".
	dagsFlag = commandLineFlag{
		name:      "dags",
		shorthand: "d",
		usage:     "Directory containing DAG files (default: $HOME/.config/dagu/dags)",
	}

	// The hostname or IP address on which the server will listen.
	hostFlag = commandLineFlag{
		name:         "host",
		shorthand:    "s",
		defaultValue: "localhost",
		usage:        "Server hostname or IP address (default: localhost)",
	}

	// The port number for the server.
	portFlag = commandLineFlag{
		name:         "port",
		shorthand:    "p",
		defaultValue: "8080",
		usage:        "Server port number (default: 8080)",
	}

	// Additional parameters to pass to the DAG execution.
	// These parameters override the default values defined in the DAG.
	// They can be specified either inline or following a "--" separator to distinguish them from other flags.
	// Accepted formats include positional parameters and key=value pairs (e.g., "P1=foo P2=bar").
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "Parameters to pass to the DAG execution (overrides DAG defaults; supports positional values and key=value pairs, e.g., P1=foo P2=bar)",
	}

	// Unique request ID required for retrying a DAG execution.
	// This flag must be provided when using the retry command.
	requestIDFlagRetry = commandLineFlag{
		name:      "request-id",
		shorthand: "r",
		usage:     "Unique request ID for retrying a DAG execution (required)",
		required:  true,
	}

	// Unique request ID used for starting a new DAG execution.
	// This is used to track and identify the execution instance and its status.
	requestIDFlagStart = commandLineFlag{
		name:      "request-id",
		shorthand: "r",
		usage:     "Unique request ID for a DAG execution",
	}

	// Suppresses output during DAG execution (e.g., logs, status updates).
	quietFlag = commandLineFlag{
		name:      "quiet",
		shorthand: "q",
		usage:     "Suppress output during DAG execution",
		isBool:    true,
	}
)

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
	isBool                               bool
}

func initFlags(cmd *cobra.Command, addFlags ...commandLineFlag) {
	addFlags = append(addFlags, configFlag)
	for _, flag := range addFlags {
		if flag.isBool {
			cmd.Flags().BoolP(flag.name, flag.shorthand, false, flag.usage)
		} else {
			cmd.Flags().StringP(flag.name, flag.shorthand, flag.defaultValue, flag.usage)
		}
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
