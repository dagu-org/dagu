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

	// Connection pool settings (1:1 is sufficient for workflow steps)
	MaxOpenConns    int `mapstructure:"maxOpenConns"`    // Maximum open connections (default: 1)
	MaxIdleConns    int `mapstructure:"maxIdleConns"`    // Maximum idle connections (default: 1)
	ConnMaxLifetime int `mapstructure:"connMaxLifetime"` // Connection max lifetime in seconds (default: 0, no limit)

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

	// SQLite-specific options
	SharedMemory bool `mapstructure:"sharedMemory"` // Enable shared cache for :memory: databases (SQLite)

	// Output settings
	OutputFormat string `mapstructure:"outputFormat"` // jsonl (default), json, csv
	Headers      bool   `mapstructure:"headers"`      // Include headers in CSV output
	NullString   string `mapstructure:"nullString"`   // String representation for NULL values

	// Large result handling
	MaxRows    int    `mapstructure:"maxRows"`    // Maximum rows to return (0 = unlimited)
	Streaming  bool   `mapstructure:"streaming"`  // Stream results to file
	OutputFile string `mapstructure:"outputFile"` // File path for streaming output

	// Import settings
	Import *ImportConfig `mapstructure:"import"` // Import data from file
}

// ImportConfig configures data import from CSV/TSV/JSONL files.
type ImportConfig struct {
	// Required fields
	InputFile string `mapstructure:"inputFile"` // Path to input file
	Table     string `mapstructure:"table"`     // Target table name

	// Format options
	Format    string `mapstructure:"format"`    // csv, tsv, jsonl (auto-detect if empty)
	HasHeader *bool  `mapstructure:"hasHeader"` // Whether first row is header (default: true for csv/tsv)
	Delimiter string `mapstructure:"delimiter"` // Field delimiter (default: "," for csv, "\t" for tsv)

	// Column mapping
	Columns []string `mapstructure:"columns"` // Explicit column names (overrides header)

	// NULL handling
	NullValues []string `mapstructure:"nullValues"` // Values to treat as NULL

	// Batch settings
	BatchSize int `mapstructure:"batchSize"` // Rows per INSERT statement (default: 1000)

	// Conflict handling
	OnConflict     string   `mapstructure:"onConflict"`     // error (default), ignore, replace
	ConflictTarget string   `mapstructure:"conflictTarget"` // Column(s) for conflict detection (required for PostgreSQL UPSERT with "replace")
	UpdateColumns  []string `mapstructure:"updateColumns"`  // Columns to update on conflict (if empty, updates all non-key columns)

	// Row limits
	SkipRows int `mapstructure:"skipRows"` // Skip first N data rows
	MaxRows  int `mapstructure:"maxRows"`  // Limit import (0 = unlimited)

	// Validation
	DryRun bool `mapstructure:"dryRun"` // Validate without importing
}

// DefaultConfig returns a Config with default values.
// These defaults match the JSON schema documentation.
func DefaultConfig() *Config {
	return &Config{
		MaxOpenConns:    5,   // Match schema default
		MaxIdleConns:    2,   // Match schema default
		ConnMaxLifetime: 300, // Match schema default (seconds)
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
		case "default", "read_committed", "repeatable_read", "serializable":
			// Valid - "default" uses database's default isolation level
		default:
			return nil, fmt.Errorf("invalid isolationLevel: %s (valid: default, read_committed, repeatable_read, serializable)", cfg.IsolationLevel)
		}
	}

	// Validate connection pool settings
	if cfg.MaxOpenConns < 0 {
		return nil, fmt.Errorf("maxOpenConns must be non-negative")
	}
	if cfg.MaxIdleConns < 0 {
		return nil, fmt.Errorf("maxIdleConns must be non-negative")
	}
	if cfg.MaxIdleConns > cfg.MaxOpenConns && cfg.MaxOpenConns > 0 {
		return nil, fmt.Errorf("maxIdleConns (%d) cannot exceed maxOpenConns (%d)", cfg.MaxIdleConns, cfg.MaxOpenConns)
	}
	if cfg.ConnMaxLifetime < 0 {
		return nil, fmt.Errorf("connMaxLifetime must be non-negative")
	}
	if cfg.Timeout < 0 {
		return nil, fmt.Errorf("timeout must be non-negative")
	}

	// Validate and set defaults for import config
	if cfg.Import != nil {
		if err := cfg.Import.validate(); err != nil {
			return nil, fmt.Errorf("invalid import config: %w", err)
		}
		cfg.Import.setDefaults()
	}

	return cfg, nil
}

// validate checks required fields and valid values for ImportConfig.
func (c *ImportConfig) validate() error {
	if c.InputFile == "" {
		return fmt.Errorf("inputFile is required")
	}
	if c.Table == "" {
		return fmt.Errorf("table is required")
	}

	// Validate format if specified
	if c.Format != "" {
		switch c.Format {
		case "csv", "tsv", "jsonl":
			// Valid
		default:
			return fmt.Errorf("invalid format: %s (must be csv, tsv, or jsonl)", c.Format)
		}
	}

	// Validate onConflict if specified
	if c.OnConflict != "" {
		switch c.OnConflict {
		case "error", "ignore", "replace":
			// Valid
		default:
			return fmt.Errorf("invalid onConflict: %s (must be error, ignore, or replace)", c.OnConflict)
		}
	}

	// Validate batch size
	if c.BatchSize < 0 {
		return fmt.Errorf("batchSize must be non-negative")
	}

	// Validate row limits
	if c.SkipRows < 0 {
		return fmt.Errorf("skipRows must be non-negative")
	}
	if c.MaxRows < 0 {
		return fmt.Errorf("maxRows must be non-negative")
	}

	return nil
}

// setDefaults applies default values to ImportConfig.
func (c *ImportConfig) setDefaults() {
	if c.Format == "" {
		c.Format = DetectFormat(c.InputFile)
	}
	if c.BatchSize == 0 {
		c.BatchSize = 1000
	}
	if c.OnConflict == "" {
		c.OnConflict = "error"
	}
	if len(c.NullValues) == 0 {
		c.NullValues = []string{"", "NULL", "null", "\\N"}
	}
	// HasHeader defaults to true for CSV/TSV (most files have headers)
	if c.HasHeader == nil {
		t := true
		c.HasHeader = &t
	}
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

// importConfigSchema defines the JSON schema for import configuration.
var importConfigSchema = &jsonschema.Schema{
	Type:        "object",
	Description: "Import data from CSV/TSV/JSONL file",
	Properties: map[string]*jsonschema.Schema{
		"inputFile":      {Type: "string", Description: "Path to input file"},
		"table":          {Type: "string", Description: "Target table name"},
		"format":         {Type: "string", Enum: []any{"csv", "tsv", "jsonl"}, Description: "Input format (auto-detected from file extension if not specified)"},
		"hasHeader":      {Type: "boolean", Description: "Whether first row is header (default: true)"},
		"delimiter":      {Type: "string", Description: "Field delimiter (default: ',' for csv, '\\t' for tsv)"},
		"columns":        {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Explicit column names"},
		"nullValues":     {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Values to treat as NULL"},
		"batchSize":      {Type: "integer", Description: "Rows per INSERT statement (default: 1000)"},
		"onConflict":     {Type: "string", Enum: []any{"error", "ignore", "replace"}, Description: "Conflict handling (default: error)"},
		"conflictTarget": {Type: "string", Description: "Column(s) for conflict detection (required for PostgreSQL UPSERT with 'replace')"},
		"updateColumns":  {Type: "array", Items: &jsonschema.Schema{Type: "string"}, Description: "Columns to update on conflict (if empty, updates all non-key columns)"},
		"skipRows":       {Type: "integer", Description: "Skip first N data rows"},
		"maxRows":        {Type: "integer", Description: "Limit import rows (0 = unlimited)"},
		"dryRun":         {Type: "boolean", Description: "Validate without importing"},
	},
	Required: []string{"inputFile", "table"},
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
		"isolationLevel": {Type: "string", Enum: []any{"default", "read_committed", "repeatable_read", "serializable"}, Description: "Transaction isolation level"},
		"advisoryLock":   {Type: "string", Description: "Named advisory lock"},
		"outputFormat":   {Type: "string", Enum: []any{"jsonl", "json", "csv"}, Description: "Output format"},
		"headers":        {Type: "boolean", Description: "Include headers in CSV output"},
		"nullString":     {Type: "string", Description: "String representation for NULL values"},
		"maxRows":        {Type: "integer", Description: "Maximum rows to return"},
		"streaming":      {Type: "boolean", Description: "Stream results to file"},
		"outputFile":     {Type: "string", Description: "File path for streaming output"},
		"import":         importConfigSchema,
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
		"sharedMemory": {Type: "boolean", Description: "Enable shared cache for :memory: databases to share data across DAG steps"},
		"outputFormat": {Type: "string", Enum: []any{"jsonl", "json", "csv"}, Description: "Output format"},
		"headers":      {Type: "boolean", Description: "Include headers in CSV output"},
		"nullString":   {Type: "string", Description: "String representation for NULL values"},
		"maxRows":      {Type: "integer", Description: "Maximum rows to return"},
		"streaming":    {Type: "boolean", Description: "Stream results to file"},
		"outputFile":   {Type: "string", Description: "File path for streaming output"},
		"import":       importConfigSchema,
	},
	Required: []string{"dsn"},
}

func init() {
	core.RegisterExecutorConfigSchema("postgres", postgresConfigSchema)
	core.RegisterExecutorConfigSchema("sqlite", sqliteConfigSchema)
}
