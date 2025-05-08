package cmd

import (
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
		bindViper: true,
	}

	// Directory where DAG definition files are stored.
	// If not provided, the default is "$HOME/.config/dagu/dags".
	dagsFlag = commandLineFlag{
		name:      "dags",
		shorthand: "d",
		usage:     "Directory containing DAG files (default: $HOME/.config/dagu/dags)",
		bindViper: true,
	}

	// The hostname or IP address on which the server will listen.
	hostFlag = commandLineFlag{
		name:         "host",
		shorthand:    "s",
		defaultValue: "localhost",
		usage:        "Server hostname or IP address (default: localhost)",
		bindViper:    true,
	}

	// The port number for the server.
	portFlag = commandLineFlag{
		name:         "port",
		shorthand:    "p",
		defaultValue: "8080",
		usage:        "Server port number (default: 8080)",
		bindViper:    true,
	}

	// Additional parameters to pass to the DAG execution.
	// These parameters override the default values defined in the DAG.
	// They can be specified either inline or following a "--" separator to distinguish them from other flags.
	// Accepted formats include positional parameters and key=value pairs (e.g., "P1=foo P2=bar").
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "Parameters to pass to the DAG run (overrides DAG defaults; supports positional values and key=value pairs, e.g., P1=foo P2=bar)",
	}

	// Unique execution ID required for retrying a DAG execution.
	// This flag must be provided when using the retry command.
	execIDFlagRetry = commandLineFlag{
		name:      "exec-id",
		shorthand: "e",
		usage:     "Unique execution ID for retrying a DAG run (required)",
		required:  true,
	}

	// Unique execution ID used for starting a new DAG execution.
	// This is used to track and identify the execution instance and its status.
	execIDFlagStart = commandLineFlag{
		name:      "exec-id",
		shorthand: "e",
		usage:     "Unique execution ID for a DAG run",
	}

	// Unique execution ID used for stopping a DAG execution.
	execIDFlagStop = commandLineFlag{
		name:      "exec-id",
		shorthand: "e",
		usage:     "execution ID for stopping a DAG run",
	}

	// Unique execution ID used for restarting a DAG execution.
	execIDFlagRestart = commandLineFlag{
		name:      "exec-id",
		shorthand: "e",
		usage:     "execution ID for restarting a DAG run",
	}

	// Unique execution ID used for checking the status of a DAG execution.
	execIDFlagStatus = commandLineFlag{
		name:      "exec-id",
		shorthand: "e",
		usage:     "execution ID for checking the status of a DAG run",
	}

	// rootDAGNameFlag reads the root DAG name for starting a child execution.
	rootFlag = commandLineFlag{
		name:  "root",
		usage: "[only for child executions] Root execution ID for a DAG run",
	}

	// parentFlag reads the parent ref for starting a child execution.
	parentFlag = commandLineFlag{
		name:  "parent",
		usage: "[only for child executions] Parent execution ID for a DAG run",
	}

	// Suppresses output during DAG run (e.g., logs, status updates).
	quietFlag = commandLineFlag{
		name:      "quiet",
		shorthand: "q",
		usage:     "Suppress output during DAG run",
		isBool:    true,
	}
)

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
	isBool                               bool
	bindViper                            bool
}

func initFlags(cmd *cobra.Command, additionalFlags ...commandLineFlag) {
	flags := append([]commandLineFlag{configFlag, quietFlag}, additionalFlags...)

	for _, flag := range flags {
		if flag.isBool {
			cmd.Flags().BoolP(flag.name, flag.shorthand, false, flag.usage)
		} else {
			cmd.Flags().StringP(flag.name, flag.shorthand, flag.defaultValue, flag.usage)
		}
		if flag.required {
			_ = cmd.MarkFlagRequired(flag.name)
		}
	}
}

func bindFlags(cmd *cobra.Command, additionalFlags ...commandLineFlag) {
	flags := append([]commandLineFlag{configFlag}, additionalFlags...)

	for _, flag := range flags {
		if flag.bindViper {
			_ = viper.BindPFlag(flag.name, cmd.Flags().Lookup(flag.name))
		}
	}
}
