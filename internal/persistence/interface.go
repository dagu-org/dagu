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

package persistence

import (
	"errors"
	"fmt"
	"time"

	"github.com/daguflow/dagu/internal/dag"
	"github.com/daguflow/dagu/internal/persistence/grep"
	"github.com/daguflow/dagu/internal/persistence/model"
)

// Error variables for common persistence operations
var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")

	// HistoryStore specific errors
	ErrReqIDNotFound   = errors.New("request ID not found")
	ErrCreateDir       = errors.New("failed to create new directory")
	ErrConflict        = errors.New("conflict")
	ErrFileIsCompacted = errors.New("file is already compacted")
	ErrWriterOpen      = errors.New("writer is already open")
)

// DataStores is an interface that provides access to various data stores.
type DataStores interface {
	HistoryStore() HistoryStore // Returns the HistoryStore implementation
	DAGStore() DAGStore         // Returns the DAGStore implementation
	FlagStore() FlagStore       // Returns the FlagStore implementation
}

// HistoryStore interface defines methods for managing DAG execution history.
type HistoryStore interface {
	// Open initializes a new history entry for a DAG execution
	Open(dagFile string, t time.Time, requestID string) error
	// Write records a new status for the current DAG execution
	Write(status *model.Status) error
	// Close finalizes the current history entry
	Close() error
	// Update modifies an existing history entry
	Update(dagFile, requestID string, st *model.Status) error
	// ReadStatusRecent retrieves the n most recent status entries for a DAG
	ReadStatusRecent(dagFile string, n int) []*model.StatusFile
	// ReadStatusToday retrieves the latest status entry for today
	ReadStatusToday(dagFile string) (*model.Status, error)
	// ReadStatusForDate retrieves all status entries for a specific date
	ReadStatusForDate(dagID string, date time.Time) ([]*model.StatusFile, error)
	// FindByRequestID locates a specific history entry by its request ID
	FindByRequestID(dagFile string, requestID string) (*model.StatusFile, error)
	// RemoveAll deletes all history entries for a DAG
	RemoveAll(dagFile string) error
	// RemoveOld deletes history entries older than the specified retention period
	RemoveOld(dagFile string, retentionDays int) error
	// Rename updates the DAG identifier in history entries
	Rename(oldName, newName string) error
}

// DAGStore interface defines methods for managing DAG definitions and metadata.
type DAGStore interface {
	// Create adds a new DAG definition
	Create(name string, spec []byte) (string, error)
	// Delete removes a DAG definition
	Delete(name string) error
	// List retrieves all DAG definitions
	List() (ret []*dag.DAG, errs []string, err error)
	// ListPagination retrieves DAG definitions with pagination
	ListPagination(params DAGListPaginationArgs) (*DagListPaginationResult, error)
	// GetMetadata retrieves basic metadata for a DAG
	GetMetadata(name string) (*dag.DAG, error)
	// GetDetails retrieves detailed information for a DAG
	GetDetails(name string) (*dag.DAG, error)
	// Grep searches for a pattern across all DAG definitions
	Grep(pattern string) (ret []*GrepResult, errs []string, err error)
	// Rename changes the identifier of a DAG
	Rename(oldID, newID string) error
	// GetSpec retrieves the specification of a DAG
	GetSpec(name string) (string, error)
	// UpdateSpec modifies the specification of a DAG
	UpdateSpec(name string, spec []byte) error
	// Find locates a DAG by its name
	Find(name string) (*dag.DAG, error)
	// TagList retrieves all tags used across DAGs
	TagList() ([]string, []string, error)
}

// DAGListPaginationArgs defines parameters for paginated DAG listing.
type DAGListPaginationArgs struct {
	Page  int     // Page number
	Limit int     // Number of items per page
	Name  *string // Optional name filter
	Tag   *string // Optional tag filter
}

// DagListPaginationResult contains the result of a paginated DAG list query.
type DagListPaginationResult struct {
	DagList   []*dag.DAG // List of DAGs for the current page
	Count     int        // Total number of DAGs matching the query
	ErrorList []string   // Any errors encountered during the query
}

// GrepResult represents the result of a grep operation on DAG definitions.
type GrepResult struct {
	Name    string        // Name of the DAG
	DAG     *dag.DAG      // DAG object
	Matches []*grep.Match // List of matches found in the DAG definition
}

// FlagStore interface defines methods for managing DAG flags.
type FlagStore interface {
	// ToggleSuspend changes the suspended state of a DAG
	ToggleSuspend(id string, suspend bool) error
	// IsSuspended checks if a DAG is currently suspended
	IsSuspended(id string) bool
}
