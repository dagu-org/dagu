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

package scheduler

import (
	"errors"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/dag"
	dagscheduler "github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/util"
)

var (
	errJobRunning      = errors.New("job already running")
	errJobIsNotRunning = errors.New("job is not running")
	errJobFinished     = errors.New("job already finished")
)

var _ jobCreator = (*jobCreatorImpl)(nil)

type jobCreatorImpl struct {
	Executable string
	WorkDir    string
	Client     client.Client
}

func (jf jobCreatorImpl) CreateJob(workflow *dag.DAG, next time.Time) job {
	return &jobImpl{
		DAG:        workflow,
		Executable: jf.Executable,
		WorkDir:    jf.WorkDir,
		Next:       next,
		Client:     jf.Client,
	}
}

var _ job = (*jobImpl)(nil)

type jobImpl struct {
	DAG        *dag.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Client     client.Client
}

func (j *jobImpl) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *jobImpl) Start() error {
	latestStatus, err := j.Client.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}

	if latestStatus.Status == dagscheduler.StatusRunning {
		// already running
		return errJobRunning
	}

	// check the last execution time
	lastExecTime, err := util.ParseTime(latestStatus.StartedAt)
	if err == nil {
		lastExecTime = lastExecTime.Truncate(time.Second * 60)
		if lastExecTime.After(j.Next) || j.Next.Equal(lastExecTime) {
			return errJobFinished
		}
	}
	return j.Client.Start(j.DAG, client.StartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) Stop() error {
	latestStatus, err := j.Client.GetLatestStatus(j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != dagscheduler.StatusRunning {
		return errJobIsNotRunning
	}
	return j.Client.Stop(j.DAG)
}

func (j *jobImpl) Restart() error {
	return j.Client.Restart(j.DAG, client.RestartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) String() string {
	return j.DAG.Name
}
