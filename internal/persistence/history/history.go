// Copyright (C) 2024 The Dagu Authors
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

package history

import (
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/persistence/model"
)

var (
	ErrReqIDNotFound     = errors.New("request ID not found")
	ErrCreateDir         = errors.New("failed to create new directory")
	ErrConflict          = errors.New("conflict")
	ErrFileIsCompacted   = errors.New("file is already compacted")
	ErrWriterOpen        = errors.New("writer is already open")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
)

// Store interface defines methods for managing DAG execution history.
type Store interface {
	// Open initializes a new history entry for a DAG execution
	Open(dagFile string, t time.Time, requestID string) error
	// Write records a new status for the current DAG execution
	Write(status *model.Status) error
	// Close finalizes the current history entry
	Close() error
	// UpdateStatus modifies an existing history entry
	UpdateStatus(dagFile, requestID string, st *model.Status) error
	// GetLatest retrieves the latest status entry for today
	GetLatest(dagFile string) (*model.Status, error)
	// GetByRequestID locates a specific history entry by its request ID
	GetByRequestID(dagFile string, requestID string) (*model.History, error)
	// ListRecent retrieves the n most recent status entries for a DAG
	ListRecent(dagFile string, n int) []*model.History
	// ListStatusesByDate retrieves all status entries for a specific date
	ListByLocalDate(date time.Time) ([]*model.History, error)
	// ListRecentAll retrieves the n most recent status files across all DAGs.
	ListRecentAll(n int) ([]*model.History, error)
	// DeleteAll deletes all history entries for a DAG
	DeleteAll(dagFile string) error
	// DeleteOld deletes history entries older than the specified retention period
	DeleteOld(dagFile string, retentionDays int) error
	// RenameDAG updates the DAG identifier in history entries
	RenameDAG(oldID, newID string) error
}
