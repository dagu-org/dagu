package sql

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/go-viper/mapstructure/v2"
	"github.com/google/jsonschema-go/jsonschema"
)

// Config represents the SQL executor configuration.
type Config struct {
	// DSN is the data source name for database connection.
	// Format depends on the driver:
	// - PostgreSQL: "postgres://user:pass@host:port/dbname?sslmode=disable"
	// - SQLite: "file:./data.db?mode=rw" or ":memory:"
	DSN string `mapstructure:"dsn"`

	// Connection pool settings
	MaxOpenConns    int `mapstructure:"maxOpenConns"`    // Maximum open connections (default: 5)
	MaxIdleConns    int `mapstructure:"maxIdleConns"`    // Maximum idle connections (default: 2)
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"` // Connection max lifetime in seconds (default: 300)

	// Parameterized queries (SQL injection prevention)
	// Can be map[string]any for named params or []any for positional params
	Params any `mapstructure:"params"`

	// Execution settings
	Timeout        int    `mapstructure:"timeout"`        // Query timeout in seconds (default: 60)
	Transaction    bool   `mapstructure:"transaction"`    // Wrap execution in transaction
	IsolationLevel string `mapstructure:"isolationLevel"` // Transaction isolation level

	// Locking
	AdvisoryLock string `mapstructure:"advisoryLock"` // Named advisory lock (PostgreSQL)
	FileLock     bool   `mapstructure:"fileLock"`     // Use file locking (SQLite)

	// Output settings
	OutputFormat string `mapstructure:"outputFormat"` // jsonl (default), json, csv
	Headers      bool   `mapstructure:"headers"`      // Include headers in CSV output
	NullString   string `mapstructure:"nullString"`   // String representation for NULL values

	// Large result handling
	MaxRows    int    `mapstructure:"maxRows"`    // Maximum rows to return (0 = unlimited)
	Streaming  bool   `mapstructure:"streaming"`  // Stream results to file
	OutputFile string `mapstructure:"outputFile"` // File path for streaming output
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 300,
		Timeout:         60,
		OutputFormat:    "jsonl",
		NullString:      "null",
	}
}

// ParseConfig parses the executor configuration from a map.
func ParseConfig(_ context.Context, mapCfg map[string]any) (*Config, error) {
	cfg := DefaultConfig()

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		Result:           cfg,
		WeaklyTypedInput: true,
		TagName:          "mapstructure",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create config decoder: %w", err)
	}

	if err := decoder.Decode(mapCfg); err != nil {
		return nil, fmt.Errorf("failed to decode sql config: %w", err)
	}

	// Validate required fields
	if cfg.DSN == "" {
		return nil, fmt.Errorf("dsn is required")
	}

	// Validate output format
	switch cfg.OutputFormat {
	case "jsonl", "json", "csv":
		// Valid
	default:
		return nil, fmt.Errorf("invalid outputFormat: %s (must be jsonl, json, or csv)", cfg.OutputFormat)
	}

	// Validate isolation level if specified
	if cfg.IsolationLevel != "" {
		switch cfg.IsolationLevel {
		case "read_committed", "repeatable_read", "serializable":
			// Valid
		default:
			return nil, fmt.Errorf("invalid isolationLevel: %s", cfg.IsolationLevel)
		}
	}

	return cfg, nil
}

// GetNamedParams returns params as a map if they are named parameters.
func (c *Config) GetNamedParams() (map[string]any, bool) {
	if c.Params == nil {
		return nil, false
	}
	params, ok := c.Params.(map[string]any)
	return params, ok
}

// GetPositionalParams returns params as a slice if they are positional parameters.
func (c *Config) GetPositionalParams() ([]any, bool) {
	if c.Params == nil {
		return nil, false
	}
	params, ok := c.Params.([]any)
	return params, ok
}

// JSON Schema for SQL executor configurations
var postgresConfigSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"dsn":             {Type: "string", Description: "PostgreSQL connection string (DSN)"},
		"maxOpenConns":    {Type: "integer", Description: "Maximum open connections"},
		"maxIdleConns":    {Type: "integer", Description: "Maximum idle connections"},
		"connMaxLifetime": {Type: "integer", Description: "Connection max lifetime in seconds"},
		"params": {
			Description: "Query parameters (map for named, array for positional)",
			OneOf: []*jsonschema.Schema{
				{Type: "object", AdditionalProperties: &jsonschema.Schema{}},
				{Type: "array"},
			},
		},
		"timeout":        {Type: "integer", Description: "Query timeout in seconds"},
		"transaction":    {Type: "boolean", Description: "Wrap execution in transaction"},
		"isolationLevel": {Type: "string", Enum: []any{"read_committed", "repeatable_read", "serializable"}, Description: "Transaction isolation level"},
		"advisoryLock":   {Type: "string", Description: "Named advisory lock"},
		"outputFormat":   {Type: "string", Enum: []any{"jsonl", "json", "csv"}, Description: "Output format"},
		"headers":        {Type: "boolean", Description: "Include headers in CSV output"},
		"nullString":     {Type: "string", Description: "String representation for NULL values"},
		"maxRows":        {Type: "integer", Description: "Maximum rows to return"},
		"streaming":      {Type: "boolean", Description: "Stream results to file"},
		"outputFile":     {Type: "string", Description: "File path for streaming output"},
	},
	Required: []string{"dsn"},
}

var sqliteConfigSchema = &jsonschema.Schema{
	Type: "object",
	Properties: map[string]*jsonschema.Schema{
		"dsn":          {Type: "string", Description: "SQLite connection string (file path or :memory:)"},
		"maxOpenConns": {Type: "integer", Description: "Maximum open connections"},
		"maxIdleConns": {Type: "integer", Description: "Maximum idle connections"},
		"params": {
			Description: "Query parameters (map for named, array for positional)",
			OneOf: []*jsonschema.Schema{
				{Type: "object", AdditionalProperties: &jsonschema.Schema{}},
				{Type: "array"},
			},
		},
		"timeout":      {Type: "integer", Description: "Query timeout in seconds"},
		"transaction":  {Type: "boolean", Description: "Wrap execution in transaction"},
		"fileLock":     {Type: "boolean", Description: "Use file locking for exclusive access"},
		"outputFormat": {Type: "string", Enum: []any{"jsonl", "json", "csv"}, Description: "Output format"},
		"headers":      {Type: "boolean", Description: "Include headers in CSV output"},
		"nullString":   {Type: "string", Description: "String representation for NULL values"},
		"maxRows":      {Type: "integer", Description: "Maximum rows to return"},
		"streaming":    {Type: "boolean", Description: "Stream results to file"},
		"outputFile":   {Type: "string", Description: "File path for streaming output"},
	},
	Required: []string{"dsn"},
}

func init() {
	core.RegisterExecutorConfigSchema("postgres", postgresConfigSchema)
	core.RegisterExecutorConfigSchema("sqlite", sqliteConfigSchema)
}
