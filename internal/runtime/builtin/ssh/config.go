package ssh

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
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

var configSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"user":          {Type: "string", Description: "SSH username"},
		"host":          {Type: "string", Description: "SSH hostname"},
		"ip":            {Type: "string", Description: "SSH host IP (alias for host)"},
		"port":          {Type: "string", Description: "SSH port"},
		"key":           {Type: "string", Description: "Path to private key file"},
		"password":      {Type: "string", Description: "SSH password"},
		"strictHostKey": {Type: "boolean", Description: "Enable strict host key checking"},
		"knownHostFile": {Type: "string", Description: "Path to known_hosts file"},
		"shell":         {Type: "string", Description: "Shell for remote execution"},
		"shellArgs":     {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Additional shell arguments"},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("ssh", configSchema)
}
