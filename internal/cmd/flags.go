package cmd

import (
	"github.com/dagu-org/dagu/internal/cmn/stringutil"
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

	// Override DAGU_HOME for this command invocation.
	daguHomeFlag = commandLineFlag{
		name:  "dagu-home",
		usage: "Override DAGU_HOME for this command",
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

	// Additional parameters to pass to the dag-run.
	// These parameters override the default values defined in the DAG.
	// They can be specified either inline or following a "--" separator to distinguish them from other flags.
	// Accepted formats include positional parameters and key=value pairs (e.g., "P1=foo P2=bar").
	paramsFlag = commandLineFlag{
		name:      "params",
		shorthand: "p",
		usage:     "Parameters to pass to the dag-run (overrides DAG defaults; supports positional values and key=value pairs, e.g., P1=foo P2=bar)",
	}

	// nameFlag is used to override the DAG name from the CLI.
	// If not provided, the DAG name will be determined from the DAG definition or filename.
	nameFlag = commandLineFlag{
		name:      "name",
		shorthand: "N",
		usage:     "Override the DAG name (default: name from DAG definition or filename)",
	}

	// Unique dag-run ID required for retrying a dag-run.
	// This flag must be provided when using the retry command.
	dagRunIDFlagRetry = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "Unique dag-run ID to retry a dag-run",
		required:  true,
	}

	// stepNameForRetry is used to indicate a specific step to retry
	stepNameForRetry = commandLineFlag{
		name:         "step",
		shorthand:    "",
		usage:        "Retry only the specified step (optional)",
		defaultValue: "",
	}

	// Unique dag-run ID used for starting a new dag-run.
	// This is used to track and identify the execution instance and its status.
	dagRunIDFlag = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "Unique dag-run ID for starting a new dag-run",
	}

	// Unique dag-run ID used for stopping a dag-run.
	dagRunIDFlagStop = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "dag-run ID for stopping a dag-run",
	}

	// Unique dag-run ID used for restarting a dag-run.
	dagRunIDFlagRestart = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "dag-run ID for restarting a dag-run",
	}

	// Unique dag-run ID used for checking the status of a dag-run.
	dagRunIDFlagStatus = commandLineFlag{
		name:      "run-id",
		shorthand: "r",
		usage:     "dag-run ID for checking the status of a dag-run",
	}

	// Unique dag-run reference used for dequeueing a dag-run.
	dagRunFlagDequeue = commandLineFlag{
		name:      "dag-run",
		shorthand: "d",
		usage:     "<DAG-name>:<run-id> to dequeue a dag-run",
	}

	// queueFlag is used to override the DAG-level queue definition.
	queueFlag = commandLineFlag{
		name:      "queue",
		shorthand: "u",
		usage:     "Override the DAG-level queue definition",
	}

	// rootDAGRunFlag reads the root DAG name for starting a sub dag-run.
	rootDAGRunFlag = commandLineFlag{
		name:  "root",
		usage: "[only for sub dag-runs] reference for the root dag-run",
	}

	// parentDAGRunFlag reads the parent ref for starting a sub dag-run.
	parentDAGRunFlag = commandLineFlag{
		name:  "parent",
		usage: "[only for sub dag-runs] reference for the parent dag-run",
	}

	// defaultWorkingDirFlag is the default working directory for DAGs without explicit workingDir.
	// This is used for sub-DAG execution to inherit the parent's working directory.
	defaultWorkingDirFlag = commandLineFlag{
		name:  "default-working-dir",
		usage: "Default working directory for DAGs without explicit workingDir",
	}

	// quietFlag is used to suppress output during command execution.
	quietFlag = commandLineFlag{
		name:      "quiet",
		shorthand: "q",
		usage:     "Suppress output during dag-run",
		isBool:    true,
	}

	// cpuProfileFlag is used to enable CPU profiling.
	cpuProfileFlag = commandLineFlag{
		name:   "cpu-profile",
		usage:  "Enable CPU profiling (saves to cpu.pprof)",
		isBool: true,
	}

	// coordinatorHostFlag is the hostname or IP address for the coordinator gRPC server.
	coordinatorHostFlag = commandLineFlag{
		name:         "coordinator.host",
		shorthand:    "H",
		defaultValue: "127.0.0.1",
		usage:        "Coordinator gRPC server host (default: 127.0.0.1)",
		bindViper:    true,
	}

	// coordinatorPortFlag is the port number for the coordinator gRPC server.
	coordinatorPortFlag = commandLineFlag{
		name:         "coordinator.port",
		shorthand:    "P",
		defaultValue: "50055",
		usage:        "Coordinator gRPC server port (default: 50055)",
		bindViper:    true,
	}

	// coordinatorAdvertiseFlag is the address to advertise in the service registry.
	coordinatorAdvertiseFlag = commandLineFlag{
		name:         "coordinator.advertise",
		shorthand:    "A",
		defaultValue: "",
		usage:        "Address to advertise in service registry (default: auto-detected hostname)",
		bindViper:    true,
	}

	// workerIDFlag is the unique identifier for the worker instance.
	workerIDFlag = commandLineFlag{
		name:      "worker.id",
		shorthand: "w",
		usage:     "Worker instance ID (default: hostname@PID)",
		bindViper: true,
	}

	// workerMaxActiveRunsFlag is the maximum number of active runs for the worker.
	workerMaxActiveRunsFlag = commandLineFlag{
		name:         "worker.max-active-runs",
		shorthand:    "m",
		defaultValue: "100",
		usage:        "Maximum number of active runs (default: 100)",
		bindViper:    true,
	}

	// workerLabelsFlag is the labels for worker capabilities.
	workerLabelsFlag = commandLineFlag{
		name:      "worker.labels",
		shorthand: "l",
		usage:     "Worker labels for capability matching (format: key1=value1,key2=value2)",
		bindViper: true,
	}

	// workerCoordinatorsFlag specifies coordinator addresses for static service discovery.
	// This enables shared-nothing deployment where workers don't need access to the service registry.
	workerCoordinatorsFlag = commandLineFlag{
		name:      "worker.coordinators",
		usage:     "Coordinator addresses for static discovery (format: host1:port1,host2:port2)",
		bindViper: true,
	}

	// peerInsecureFlag disables TLS for peer connections.
	peerInsecureFlag = commandLineFlag{
		name:      "peer.insecure",
		usage:     "Use insecure connection (h2c) instead of TLS",
		isBool:    true,
		bindViper: true,
	}

	// peerCertFileFlag is the TLS certificate for peer connections.
	peerCertFileFlag = commandLineFlag{
		name:      "peer.cert-file",
		usage:     "Path to TLS certificate file for mutual TLS",
		bindViper: true,
	}

	// peerKeyFileFlag is the TLS key for peer connections.
	peerKeyFileFlag = commandLineFlag{
		name:      "peer.key-file",
		usage:     "Path to TLS key file for mutual TLS",
		bindViper: true,
	}

	// peerClientCAFileFlag is the CA certificate for peer connections.
	peerClientCAFileFlag = commandLineFlag{
		name:      "peer.client-ca-file",
		usage:     "Path to CA certificate file for server verification",
		bindViper: true,
	}

	// peerSkipTLSVerifyFlag skips TLS certificate verification for peer connections.
	peerSkipTLSVerifyFlag = commandLineFlag{
		name:      "peer.skip-tls-verify",
		usage:     "Skip TLS certificate verification (insecure)",
		isBool:    true,
		bindViper: true,
	}

	// retentionDaysFlag specifies the number of days to retain history.
	// Records older than this will be deleted.
	// If set to 0, all records (except active) will be deleted.
	retentionDaysFlag = commandLineFlag{
		name:         "retention-days",
		defaultValue: "0",
		usage:        "Number of days to retain history (0 = delete all, except active runs)",
	}

	// dryRunFlag enables preview mode without actual deletion.
	dryRunFlag = commandLineFlag{
		name:   "dry-run",
		usage:  "Preview what would be deleted without actually deleting",
		isBool: true,
	}

	// yesFlag skips the confirmation prompt.
	yesFlag = commandLineFlag{
		name:      "yes",
		shorthand: "y",
		usage:     "Skip confirmation prompt",
		isBool:    true,
	}

	// tunnelFlag enables tunnel mode.
	tunnelFlag = commandLineFlag{
		name:      "tunnel",
		shorthand: "t",
		usage:     "Enable tunnel mode for remote access",
		isBool:    true,
		bindViper: true,
	}

	// tunnelProviderFlag specifies the tunnel provider.
	tunnelProviderFlag = commandLineFlag{
		name:      "tunnel-provider",
		usage:     "Tunnel provider: 'cloudflare' or 'tailscale' (default: tailscale)",
		bindViper: true,
	}

	// tunnelTokenFlag provides authentication token for the tunnel.
	tunnelTokenFlag = commandLineFlag{
		name:      "tunnel-token",
		usage:     "Tunnel auth token (Cloudflare tunnel token or Tailscale auth key)",
		bindViper: true,
	}
)

type commandLineFlag struct {
	name, shorthand, defaultValue, usage string
	required                             bool
	isBool                               bool
	bindViper                            bool
}

// initFlags registers a set of CLI flags on the provided Cobra command.
// It always includes the base flags (config, dagu-home, quiet, cpu-profile) and then the provided additionalFlags.
// Boolean flags are registered as boolean flags and others as string flags with their default values, and any flag marked required will be recorded as required.
func initFlags(cmd *cobra.Command, additionalFlags ...commandLineFlag) {
	flags := append([]commandLineFlag{configFlag, daguHomeFlag, quietFlag, cpuProfileFlag}, additionalFlags...)

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

// bindFlags binds command-line flags to the provided Viper instance for configuration lookup.
// It binds only flags whose `bindViper` field is true, using the camel-cased key produced
// from each flag's kebab-case name.
func bindFlags(viper *viper.Viper, cmd *cobra.Command, additionalFlags ...commandLineFlag) {
	flags := append([]commandLineFlag{configFlag}, additionalFlags...)

	for _, flag := range flags {
		if flag.bindViper {
			_ = viper.BindPFlag(stringutil.KebabToCamel(flag.name), cmd.Flags().Lookup(flag.name))
		}
	}
}
