package ssh

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dagu-org/dagu/internal/common/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
)

// Config represents SSH connection info
type Config struct {
	User          string
	Host          string
	Port          string
	Key           string
	Password      string
	StrictHostKey bool     // Enable strict host key checking (defaults to true)
	KnownHostFile string   // Path to known_hosts file (defaults to ~/.ssh/known_hosts)
	Shell         string   // Shell for remote command execution (e.g., "/bin/bash")
	ShellArgs     []string // Additional shell arguments (e.g., -e, -o pipefail)
}

func FromMapConfig(_ context.Context, mapCfg map[string]any) (*Client, error) {
	def := new(struct {
		User          string
		IP            string
		Host          string
		Port          string
		Key           string
		Password      string
		StrictHostKey bool
		KnownHostFile string
		Shell         string
		ShellArgs     []string
	})
	md, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{Result: def, WeaklyTypedInput: true},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create decoder: %w", err)
	}

	if err := md.Decode(mapCfg); err != nil {
		return nil, fmt.Errorf("failed to decode ssh config: %w", err)
	}

	var host string
	if def.Host != "" {
		host = def.Host
	}
	if def.IP != "" {
		host = def.IP
	}

	shell, shellArgs, err := parseShellConfig(def.Shell, def.ShellArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shell config: %w", err)
	}

	cfg := &Config{
		User:          def.User,
		Host:          host,
		Port:          def.Port,
		Key:           def.Key,
		Password:      def.Password,
		StrictHostKey: def.StrictHostKey,
		KnownHostFile: def.KnownHostFile,
		Shell:         shell,
		ShellArgs:     shellArgs,
	}

	return NewClient(cfg)
}

func parseShellConfig(shell string, args []string) (string, []string, error) {
	shell = strings.TrimSpace(shell)
	resultArgs := slices.Clone(args)
	if shell == "" {
		return "", resultArgs, nil
	}

	parsedShell, parsedArgs, err := cmdutil.SplitCommand(shell)
	if err != nil {
		return "", nil, err
	}
	parsedShell = strings.TrimSpace(parsedShell)
	if len(parsedArgs) > 0 {
		resultArgs = append(parsedArgs, resultArgs...)
	}
	return parsedShell, resultArgs, nil
}

// ConfigSchema defines the schema for ssh executor config.
// This struct is ONLY for generating JSON Schema - not used at runtime.
type ConfigSchema struct {
	User          string   `json:"user,omitempty" jsonschema:"SSH username"`
	Host          string   `json:"host,omitempty" jsonschema:"SSH hostname"`
	IP            string   `json:"ip,omitempty" jsonschema:"SSH host IP (alias for host)"`
	Port          string   `json:"port,omitempty" jsonschema:"SSH port"`
	Key           string   `json:"key,omitempty" jsonschema:"Path to private key file"`
	Password      string   `json:"password,omitempty" jsonschema:"SSH password"`
	StrictHostKey *bool    `json:"strictHostKey,omitempty" jsonschema:"Enable strict host key checking"`
	KnownHostFile string   `json:"knownHostFile,omitempty" jsonschema:"Path to known_hosts file"`
	Shell         string   `json:"shell,omitempty" jsonschema:"Shell for remote execution"`
	ShellArgs     []string `json:"shellArgs,omitempty" jsonschema:"Additional shell arguments"`
}

func init() {
	core.RegisterExecutorConfigType[ConfigSchema]("ssh")
}
