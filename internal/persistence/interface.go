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

var (
	ErrRequestIDNotFound = fmt.Errorf("request id not found")
	ErrNoStatusDataToday = fmt.Errorf("no status data today")
	ErrNoStatusData      = fmt.Errorf("no status data")

	// HistoryStore errors
	ErrReqIDNotFound   = errors.New("request ID not found")
	ErrCreateDir       = errors.New("failed to create new directory")
	ErrConflict        = errors.New("conflict")
	ErrFileIsCompacted = errors.New("file is already compacted")
	ErrWriterOpen      = errors.New("writer is already open")
)

type DataStores interface {
	HistoryStore() HistoryStore
	DAGStore() DAGStore
	FlagStore() FlagStore
}

type HistoryStore interface {
	Open(dagFile string, t time.Time, requestID string) error
	Write(status *model.Status) error
	Close() error
	Update(dagFile, requestID string, st *model.Status) error
	ReadStatusRecent(dagFile string, n int) []*model.StatusFile
	ReadStatusToday(dagFile string) (*model.Status, error)
	ReadStatusForDate(dagID string, date time.Time) ([]*model.StatusFile, error)
	FindByRequestID(dagFile string, requestID string) (*model.StatusFile, error)
	RemoveAll(dagFile string) error
	RemoveOld(dagFile string, retentionDays int) error
	Rename(oldName, newName string) error
}

type DAGStore interface {
	Create(name string, spec []byte) (string, error)
	Delete(name string) error
	List() (ret []*dag.DAG, errs []string, err error)
	ListPagination(params DAGListPaginationArgs) (*DagListPaginationResult, error)
	GetMetadata(name string) (*dag.DAG, error)
	GetDetails(name string) (*dag.DAG, error)
	Grep(pattern string) (ret []*GrepResult, errs []string, err error)
	Rename(oldID, newID string) error
	GetSpec(name string) (string, error)
	UpdateSpec(name string, spec []byte) error
	Find(name string) (*dag.DAG, error)
	TagList() ([]string, []string, error)
}

type DAGListPaginationArgs struct {
	Page  int
	Limit int
	Name  *string
	Tag   *string
}

type DagListPaginationResult struct {
	DagList   []*dag.DAG
	Count     int
	ErrorList []string
}

type GrepResult struct {
	Name    string
	DAG     *dag.DAG
	Matches []*grep.Match
}

type FlagStore interface {
	ToggleSuspend(id string, suspend bool) error
	IsSuspended(id string) bool
}
