// Copyright (C) 2024 The Daguflow/Dagu Authors
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daguflow/dagu/internal/util"
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
	logDir := config.LogDir
	if config.DAGLogDir != "" {
		logDir = filepath.Join(config.DAGLogDir, util.ValidFilename(config.DAGName))
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
