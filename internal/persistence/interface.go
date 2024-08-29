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

package persistence

import (
	"github.com/dagu-org/dagu/internal/dag"
	"github.com/dagu-org/dagu/internal/persistence/grep"
	"github.com/dagu-org/dagu/internal/persistence/history"
)

// DataStores is an interface that provides access to various data stores.
type DataStores interface {
	HistoryStore() history.Store // Returns the HistoryStore implementation
	DAGStore() DAGStore          // Returns the DAGStore implementation
	FlagStore() FlagStore        // Returns the FlagStore implementation
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
