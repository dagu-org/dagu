// Copyright (C) 2024 The Dagu Authors
// SPDX-License-Identifier: GPL-3.0-or-later

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dagu-org/dagu/internal/util"
)

// LogFileConfig holds the configuration for opening a log file
type LogFileConfig struct {
	Prefix    string
	LogDir    string
	DAGLogDir string
	DAGName   string
	RequestID string
}

// OpenLogFile opens a log file for the workflow.
func OpenLogFile(config LogFileConfig) (*os.File, error) {
	logDir, err := prepareLogDirectory(config)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare log directory: %w", err)
	}

	filename := generateLogFilename(config)
	return openFile(filepath.Join(logDir, filename))
}

func prepareLogDirectory(config LogFileConfig) (string, error) {
	validName := util.ValidFilename(config.DAGName)
	logDir := filepath.Join(config.LogDir, validName)
	if config.DAGLogDir != "" {
		logDir = filepath.Join(config.DAGLogDir, validName)
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}

	return logDir, nil
}

func generateLogFilename(config LogFileConfig) string {
	return fmt.Sprintf("%s%s.%s.%s.log",
		config.Prefix,
		util.ValidFilename(config.DAGName),
		time.Now().Format("20060102.15:04:05.000"),
		util.TruncString(config.RequestID, 8),
	)
}

func openFile(filepath string) (*os.File, error) {
	return os.OpenFile(
		filepath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC,
		0644,
	)
}
