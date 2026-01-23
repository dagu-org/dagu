package ssh

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

const defaultSSHTimeout = 30 * time.Second

// Config represents SSH connection info
type Config struct {
	User          string
	Host          string
	Port          string
	Key           string
	Password      string
	StrictHostKey bool              // Enable strict host key checking (defaults to true)
	KnownHostFile string            // Path to known_hosts file (defaults to ~/.ssh/known_hosts)
	Shell         string            // Shell for remote command execution (e.g., "/bin/bash")
	ShellArgs     []string          // Additional shell arguments (e.g., -e, -o pipefail)
	Timeout       time.Duration     // Connection timeout (defaults to 30s)
	Env           map[string]string // Environment variables to set on remote before execution
	Bastion       *BastionConfig    // Optional bastion/jump host configuration
}

// BastionConfig represents bastion/jump host connection info
type BastionConfig struct {
	Host     string
	Port     string
	User     string
	Key      string
	Password string
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
		Timeout       string            // Duration string like "30s", "1m"
		Env           map[string]string // Environment variables for remote execution
		Bastion       *struct {
			Host     string
			Port     string
			User     string
			Key      string
			Password string
		}
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

	host := def.Host
	if def.IP != "" {
		host = def.IP
	}

	shell, shellArgs, err := parseShellConfig(def.Shell, def.ShellArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to parse shell config: %w", err)
	}

	timeout := defaultSSHTimeout
	if def.Timeout != "" {
		parsed, err := time.ParseDuration(def.Timeout)
		if err != nil {
			return nil, fmt.Errorf("invalid timeout duration %q: %w", def.Timeout, err)
		}
		timeout = parsed
	}

	var bastionCfg *BastionConfig
	if def.Bastion != nil {
		port := def.Bastion.Port
		if port == "" {
			port = "22"
		}
		bastionCfg = &BastionConfig{
			Host:     def.Bastion.Host,
			Port:     port,
			User:     def.Bastion.User,
			Key:      def.Bastion.Key,
			Password: def.Bastion.Password,
		}
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
		Timeout:       timeout,
		Env:           def.Env,
		Bastion:       bastionCfg,
	}

	return NewClient(cfg)
}

func parseShellConfig(shell string, args []string) (string, []string, error) {
	shell = strings.TrimSpace(shell)
	if shell == "" {
		return "", slices.Clone(args), nil
	}

	parsedShell, parsedArgs, err := cmdutil.SplitCommand(shell)
	if err != nil {
		return "", nil, err
	}

	// Return nil if no args to maintain nil vs empty slice distinction
	if len(parsedArgs) == 0 && len(args) == 0 {
		return strings.TrimSpace(parsedShell), nil, nil
	}

	return strings.TrimSpace(parsedShell), append(parsedArgs, args...), nil
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
		"timeout":       {Type: "string", Description: "Connection timeout (e.g., '30s', '1m')"},
		"env":           {Type: "object", Description: "Environment variables to set on remote host"},
		"bastion": {
			Type:        "object",
			Description: "Bastion/jump host configuration",
			Properties: map[string]*jsonschema.Schema{
				"host":     {Type: "string", Description: "Bastion host address"},
				"port":     {Type: "string", Description: "Bastion SSH port"},
				"user":     {Type: "string", Description: "Bastion SSH username"},
				"key":      {Type: "string", Description: "Path to bastion private key file"},
				"password": {Type: "string", Description: "Bastion SSH password"},
			},
		},
	},
}

var sftpConfigSchema = &jsonschema.Schema{
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
		"timeout":       {Type: "string", Description: "Connection timeout (e.g., '30s', '1m')"},
		"direction":     {Type: "string", Description: "Transfer direction: 'upload' or 'download'"},
		"source":        {Type: "string", Description: "Source path (local for upload, remote for download)"},
		"destination":   {Type: "string", Description: "Destination path (remote for upload, local for download)"},
		"bastion": {
			Type:        "object",
			Description: "Bastion/jump host configuration",
			Properties: map[string]*jsonschema.Schema{
				"host":     {Type: "string", Description: "Bastion host address"},
				"port":     {Type: "string", Description: "Bastion SSH port"},
				"user":     {Type: "string", Description: "Bastion SSH username"},
				"key":      {Type: "string", Description: "Path to bastion private key file"},
				"password": {Type: "string", Description: "Bastion SSH password"},
			},
		},
	},
}

func init() {
	core.RegisterExecutorConfigSchema("ssh", configSchema)
	core.RegisterExecutorConfigSchema("sftp", sftpConfigSchema)
}
