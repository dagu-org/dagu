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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/persistence/model"
)

var (
	ErrReqIDNotFound     = errors.New("request ID not found")
	ErrCreateDir         = errors.New("failed to create new directory")
	ErrFileIsCompacted   = errors.New("file is already compacted")
	ErrWriterOpen        = errors.New("writer is already open")
	ErrWriterIsClosed    = errors.New("writer is already closed")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")
	ErrConflict          = errors.New("conflict")
)

// Store interface defines methods for managing DAG execution history.
type Store interface {
	// Open initializes a new history entry for a DAG execution
	Open(ctx context.Context, dagID string, startTime time.Time, requestID string) error
	// Write records a new status for the current DAG execution
	Write(ctx context.Context, status *model.Status) error
	// Close finalizes the current history entry
	Close(ctx context.Context) error
	// UpdateStatus modifies an existing history entry
	UpdateStatus(ctx context.Context, dagID, requestID string, status *model.Status) error
	// GetLatest retrieves the latest status entry for today
	GetLatestStatus(ctx context.Context, dagID string) (*model.Status, error)
	// GetByRequestID locates a specific history entry by its request ID
	GetStatusByRequestID(ctx context.Context, dagID string, requestID string) (*model.History, error)
	// ListRecent retrieves the n most recent status entries for a DAG
	ListRecentStatuses(ctx context.Context, dagID string, limit int) []*model.History
	// ListStatusesByDate retrieves all status entries for a specific date
	ListStatusesByDate(ctx context.Context, date time.Time) ([]*model.History, error)
	// ListRecentAll retrieves the n most recent status files across all DAGs.
	ListRecentStatusesAllDAGs(ctx context.Context, limit int) ([]*model.History, error)
	// DeleteAll deletes all history entries for a DAG
	DeleteAllStatuses(ctx context.Context, dagID string) error
	// DeleteOld deletes history entries older than the specified retention period
	DeleteOldStatuses(ctx context.Context, dagID string, retentionDays int) error
	// RenameDAG updates the DAG identifier in history entries
	RenameDAG(ctx context.Context, oldID, newID string) error
}
