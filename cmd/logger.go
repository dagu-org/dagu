// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/config"
	"github.com/dagu-org/dagu/internal/fileutil"
	"github.com/dagu-org/dagu/internal/logger"
	"github.com/dagu-org/dagu/internal/util"
)

func buildLogger(cfg *config.Config, quiet bool) logger.Logger {
	return buildLoggerWithFile(cfg, quiet, nil)
}

func buildLoggerWithFile(cfg *config.Config, quiet bool, f *os.File) logger.Logger {
	var opts []logger.Option
	if cfg.Debug {
		opts = append(opts, logger.WithDebug())
	}
	if cfg.LogFormat != "" {
		opts = append(opts, logger.WithFormat(cfg.LogFormat))
	}
	if quiet {
		opts = append(opts, logger.WithQuiet())
	}
	if f != nil {
		opts = append(opts, logger.WithLogFile(f))
	}
	return logger.NewLogger(opts...)
}

// logFileSettings contains the settings for the log file.
type logFileSettings struct {
	Prefix    string
	LogDir    string
	DAGLogDir string
	DAGName   string
	RequestID string
}

// openLogFile creates and opens a log file based on the provided settings.
// It creates the necessary directory structure and returns the file handle.
func openLogFile(config logFileSettings) (*os.File, error) {
	if err := validateSettings(config); err != nil {
		return nil, fmt.Errorf("invalid log settings: %w", err)
	}

	logDir, err := setupLogDirectory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to setup log directory: %w", err)
	}

	filename := buildLogFilename(config)
	return createLogFile(filepath.Join(logDir, filename))
}

// validateSettings ensures all required fields are properly set
func validateSettings(config logFileSettings) error {
	if config.DAGName == "" {
		return fmt.Errorf("DAGName cannot be empty")
	}
	if config.LogDir == "" && config.DAGLogDir == "" {
		return fmt.Errorf("either LogDir or DAGLogDir must be specified")
	}
	return nil
}

// setupLogDirectory creates and returns the appropriate log directory
func setupLogDirectory(config logFileSettings) (string, error) {
	safeName := fileutil.SafeName(config.DAGName)

	// Determine the base directory
	baseDir := config.LogDir
	if config.DAGLogDir != "" {
		baseDir = config.DAGLogDir
	}

	logDir := filepath.Join(baseDir, safeName)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory %s: %w", logDir, err)
	}

	return logDir, nil
}

// buildLogFilename generates the log filename using the configured format
func buildLogFilename(config logFileSettings) string {
	timestamp := time.Now().Format("20060102.15:04:05.000")
	truncatedRequestID := util.TruncString(config.RequestID, 8)
	safeDagName := fileutil.SafeName(config.DAGName)

	return fmt.Sprintf("%s%s.%s.%s.log",
		config.Prefix,
		safeDagName,
		timestamp,
		truncatedRequestID,
	)
}

// createLogFile opens or creates a log file with appropriate permissions
func createLogFile(filepath string) (*os.File, error) {
	flags := os.O_CREATE | os.O_WRONLY | os.O_APPEND | os.O_SYNC
	permissions := os.FileMode(0644)

	file, err := os.OpenFile(filepath, flags, permissions)
	if err != nil {
		return nil, fmt.Errorf("failed to create/open log file %s: %w", filepath, err)
	}

	return file, nil
}
