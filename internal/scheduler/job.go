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
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dagu-org/dagu/internal/client"
	"github.com/dagu-org/dagu/internal/dag"
	dagscheduler "github.com/dagu-org/dagu/internal/dag/scheduler"
	"github.com/dagu-org/dagu/internal/util"
	"github.com/robfig/cron/v3"
)

var (
	errJobRunning      = errors.New("job already running")
	errJobIsNotRunning = errors.New("job is not running")
	errJobFinished     = errors.New("job already finished")
	errJobSkipped      = errors.New("job skipped")
)

var _ jobCreator = (*jobCreatorImpl)(nil)

type jobCreatorImpl struct {
	Executable string
	WorkDir    string
	Client     client.Client
}

func (jf jobCreatorImpl) CreateJob(dAG *dag.DAG, next time.Time, schedule cron.Schedule) job {
	return &jobImpl{
		DAG:        dAG,
		Executable: jf.Executable,
		WorkDir:    jf.WorkDir,
		Next:       next,
		Schedule:   schedule,
		Client:     jf.Client,
	}
}

var _ job = (*jobImpl)(nil)

type jobImpl struct {
	DAG        *dag.DAG
	Executable string
	WorkDir    string
	Next       time.Time
	Schedule   cron.Schedule
	Client     client.Client
}

func (j *jobImpl) GetDAG() *dag.DAG {
	return j.DAG
}

func (j *jobImpl) Start(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
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

		// Check the `skipIfSuccessful` is set to true in the DAG configuration.
		// When set to true, Dagu will automatically check the last successful run
		// time against the defined schedule. If the DAG has already run successfully
		// since the last scheduled time, the current run will be skipped.
		if j.DAG.SkipIfSuccessful {
			prev := j.Prev()
			if lastExecTime.After(prev) || lastExecTime.Equal(prev) {
				// Calculate the previous scheduled time
				lastStartedAt, _ := util.ParseTime(latestStatus.StartedAt)
				return fmt.Errorf("%w: last successful run time: %s is after the previous scheduled time: %s", errJobSkipped, lastStartedAt, prev)
			}
		}
	}

	return j.Client.Start(ctx, j.DAG, client.StartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) Prev() time.Time {
	// Since robfig/cron does not provide a way to get the previous schedule time,
	// we need to do it manually.
	// The idea is to get the next schedule time and subtract the duration of the schedule.
	// This will give us the previous schedule time.
	t := j.Schedule.Next(j.Next.Add(time.Second))
	return j.Next.Add(-t.Sub(j.Next))
}

func (j *jobImpl) Stop(ctx context.Context) error {
	latestStatus, err := j.Client.GetLatestStatus(ctx, j.DAG)
	if err != nil {
		return err
	}
	if latestStatus.Status != dagscheduler.StatusRunning {
		return errJobIsNotRunning
	}
	return j.Client.Stop(ctx, j.DAG)
}

func (j *jobImpl) Restart(ctx context.Context) error {
	return j.Client.Restart(ctx, j.DAG, client.RestartOptions{
		Quiet: true,
	})
}

func (j *jobImpl) String() string {
	return j.DAG.Name
}
