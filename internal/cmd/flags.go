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

	// Additional parameters to pass to the DAG-run.
	// These parameters override the default values defined in the DAG.
	// They can be specified either inline or following a "--" separator to distinguish them from other flags.
	// Accepted formats include positional parameters and key=value pairs (e.g., "P1=foo P2=bar").
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "Parameters to pass to the DAG-run (overrides DAG defaults; supports positional values and key=value pairs, e.g., P1=foo P2=bar)",
	}

	// Unique DAG-run ID required for retrying a DAG-run.
	// This flag must be provided when using the retry command.
	dagRunIDFlagRetry = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "Unique DAG-run ID to retry a DAG-run",
		required:  true,
	}

	// Unique DAG-run ID used for starting a new DAG-run.
	// This is used to track and identify the execution instance and its status.
	dagRunIDFlag = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "Unique DAG-run ID for starting a new DAG-run",
	}

	// Unique DAG-run ID used for stopping a DAG-run.
	dagRunIDFlagStop = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "DAG-run ID for stopping a DAG-run",
	}

	// Unique DAG-run ID used for restarting a DAG-run.
	dagRunIDFlagRestart = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "DAG-run ID for restarting a DAG-run",
	}

	// Unique DAG-run ID used for checking the status of a DAG-run.
	dagRunIDFlagStatus = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "DAG-run ID for checking the status of a DAG-run",
	}

	// Unique DAG-run reference used for dequeueing a DAG-run.
	dagRunFlagDequeue = commandLineFlag{
		name:      "dag-run",
		shorthand: "d",
		usage:     "<DAG-name>:<run-id> to dequeue a DAG-run",
		required:  true,
	}

	// rootDAGRunFlag reads the root DAG name for starting a child DAG-run.
	rootDAGRunFlag = commandLineFlag{
		name:  "root",
		usage: "[only for child DAG-runs] reference for the root DAG-run",
	}

	// parentDAGRunFlag reads the parent ref for starting a child DAG-run.
	parentDAGRunFlag = commandLineFlag{
		name:  "parent",
		usage: "[only for child DAG-runs] reference for the parent DAG-run",
	}

	// quietFlag is used to suppress output during command execution.
	quietFlag = commandLineFlag{
		name:      "quiet",
		shorthand: "q",
		usage:     "Suppress output during DAG-run",
		isBool:    true,
	}

	// cpuProfileFlag is used to enable CPU profiling.
	cpuProfileFlag = commandLineFlag{
		name:   "cpu-profile",
		usage:  "Enable CPU profiling (saves to cpu.pprof)",
		isBool: true,
	}
)

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
	isBool                               bool
	bindViper                            bool
}

func initFlags(cmd *cobra.Command, additionalFlags ...commandLineFlag) {
	flags := append([]commandLineFlag{configFlag, quietFlag, cpuProfileFlag}, additionalFlags...)

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
