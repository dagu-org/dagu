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

	// Additional parameters to pass to the workflow.
	// These parameters override the default values defined in the DAG.
	// They can be specified either inline or following a "--" separator to distinguish them from other flags.
	// Accepted formats include positional parameters and key=value pairs (e.g., "P1=foo P2=bar").
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "Parameters to pass to the workflow (overrides DAG defaults; supports positional values and key=value pairs, e.g., P1=foo P2=bar)",
	}

	// Unique workflow ID required for retrying a workflow.
	// This flag must be provided when using the retry command.
	workflowIDFlagRetry = commandLineFlag{
		name:      "workflow-id",
		shorthand: "e",
		usage:     "Unique workflow ID for retrying a workflow (required)",
		required:  true,
	}

	// Unique workflow ID used for starting a new workflow.
	// This is used to track and identify the execution instance and its status.
	workflowIDFlag = commandLineFlag{
		name:      "workflow-id",
		shorthand: "e",
		usage:     "Unique workflow ID for a workflow",
	}

	// Unique workflow ID used for stopping a workflow.
	workflowIDFlagStop = commandLineFlag{
		name:      "workflow-id",
		shorthand: "e",
		usage:     "workflow ID for stopping a workflow",
	}

	// Unique workflow ID used for restarting a workflow.
	workflowIDFlagRestart = commandLineFlag{
		name:      "workflow-id",
		shorthand: "e",
		usage:     "workflow ID for restarting a workflow",
	}

	// Unique workflow ID used for checking the status of a workflow.
	workflowIDFlagStatus = commandLineFlag{
		name:      "workflow-id",
		shorthand: "e",
		usage:     "workflow ID for checking the status of a workflow",
	}

	// Workflow name to dequeue a workflow.
	workflowFlagDequeue = commandLineFlag{
		name:      "workflow",
		shorthand: "w",
		usage:     "workflow name to dequeue a workflow",
		required:  true,
	}

	// rootRefNameFlag reads the root DAG name for starting a child workflow.
	rootWorkflowFlag = commandLineFlag{
		name:  "root",
		usage: "[only for child workflows] workflow-reference for the root workflow",
	}

	// parentWorkflowFlag reads the parent ref for starting a child workflow.
	parentWorkflowFlag = commandLineFlag{
		name:  "parent",
		usage: "[only for child workflows] workflow-reference for the parent workflow",
	}

	// Suppresses output during workflow (e.g., logs, status updates).
	quietFlag = commandLineFlag{
		name:      "quiet",
		shorthand: "q",
		usage:     "Suppress output during workflow",
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
