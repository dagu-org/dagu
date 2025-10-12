package ssh

import (
	"context"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
)

// Config represents SSH connection info
type Config struct {
	User          string
	Host          string
	Port          string
	Key           string
	Password      string
	StrictHostKey bool   // Enable strict host key checking (defaults to true)
	KnownHostFile string // Path to known_hosts file (defaults to ~/.ssh/known_hosts)
}

func FromMapConfig(ctx context.Context, mapCfg map[string]any) (*Client, error) {
	def := new(struct {
		User          string
		IP            string
		Host          string
		Port          string
		Key           string
		Password      string
		StrictHostKey bool
		KnownHostFile string
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

	cfg := &Config{
		User:          def.User,
		Host:          host,
		Port:          def.Port,
		Key:           def.Key,
		Password:      def.Password,
		StrictHostKey: def.StrictHostKey,
		KnownHostFile: def.KnownHostFile,
	}

	return NewClient(cfg)
}
