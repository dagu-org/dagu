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

package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/util"
)

// openLogFile opens a log file for the workflow.
func openLogFile(
	prefix string,
	logDir string,
	workflow *dag.DAG,
	requestID string,
) (*os.File, error) {
	name := util.ValidFilename(workflow.Name)
	if workflow.LogDir != "" {
		logDir = filepath.Join(workflow.LogDir, name)
	}
	// Check if the log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		// Create the log directory
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, err
		}
	}
	file := filepath.Join(logDir, fmt.Sprintf("%s%s.%s.%s.log",
		prefix,
		name,
		time.Now().Format("20060102.15:04:05.000"),
		util.TruncString(requestID, 8),
	))
	// Open or create the log file
	return os.OpenFile(
		file, os.O_CREATE|os.O_WRONLY|os.O_APPEND|os.O_SYNC, 0644,
	)
}
