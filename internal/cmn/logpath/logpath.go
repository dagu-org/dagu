// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package logpath

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagucloud/dagu/internal/cmn/eval"
	"github.com/dagucloud/dagu/internal/cmn/fileutil"
	"github.com/dagucloud/dagu/internal/cmn/stringutil"
)

// Config defines configuration for DAG run log file creation.
type Config struct {
	BaseDir   string
	DAGLogDir string
	Name      string
	DAGRunID  string
}

// Generate expands the configured log directories, creates the DAG-run log
// directory if needed, and returns the full log file path.
func Generate(ctx context.Context, baseLogDir, dagLogDir, dagName, dagRunID string) (string, error) {
	baseDir, err := eval.String(ctx, baseLogDir, eval.WithOSExpansion())
	if err != nil {
		return "", fmt.Errorf("failed to expand log directory: %w", err)
	}

	dagDir, err := eval.String(ctx, dagLogDir, eval.WithOSExpansion())
	if err != nil {
		return "", fmt.Errorf("failed to expand DAG log directory: %w", err)
	}

	cfg := Config{
		BaseDir:   baseDir,
		DAGLogDir: dagDir,
		Name:      dagName,
		DAGRunID:  dagRunID,
	}

	if err := cfg.Validate(); err != nil {
		return "", fmt.Errorf("invalid log settings: %w", err)
	}

	dir, err := cfg.LogDir()
	if err != nil {
		return "", fmt.Errorf("failed to setup log directory: %w", err)
	}

	return filepath.Join(dir, cfg.LogFile()), nil
}

// Validate checks that essential fields are provided.
func (cfg Config) Validate() error {
	if cfg.Name == "" {
		return fmt.Errorf("DAGName cannot be empty")
	}
	if cfg.BaseDir == "" && cfg.DAGLogDir == "" {
		return fmt.Errorf("either LogDir or DAGLogDir must be specified")
	}
	return nil
}

// LogDir creates and returns the log directory based on the log configuration.
func (cfg Config) LogDir() (string, error) {
	baseDir := cfg.BaseDir
	if cfg.DAGLogDir != "" {
		baseDir = cfg.DAGLogDir
	}
	if baseDir == "" {
		return "", fmt.Errorf("base log directory is not set")
	}

	utcTimestamp := time.Now().UTC().Format("20060102_150405Z")

	safeName := fileutil.SafeName(cfg.Name)
	logDir := filepath.Join(baseDir, safeName, "dag-run_"+utcTimestamp+"_"+cfg.DAGRunID)
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return "", fmt.Errorf("failed to initialize directory %s: %w", logDir, err)
	}

	return logDir, nil
}

// LogFile constructs the scheduler log filename using the current timestamp and
// a truncated DAG-run ID.
func (cfg Config) LogFile() string {
	timestamp := time.Now().Format("20060102.150405.000")
	truncDAGRunID := stringutil.TruncString(cfg.DAGRunID, 8)

	return fmt.Sprintf("dag-run_%s.%s.log",
		timestamp,
		truncDAGRunID,
	)
}
