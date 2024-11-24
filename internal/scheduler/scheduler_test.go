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
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"

	"github.com/stretchr/testify/require"
)

func TestScheduler(t *testing.T) {
	t.Parallel()
	t.Run("Start", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		setFixedTime(now)

		entryReader := &mockEntryReader{
			Entries: []*entry{
				{
					Job:    &mockJob{},
					Next:   now,
					Logger: test.NewLogger(),
				},
				{
					Job:    &mockJob{},
					Next:   now.Add(time.Minute),
					Logger: test.NewLogger(),
				},
			},
		}

		schedulerInstance := newScheduler(entryReader, test.NewLogger(), testHomeDir, time.Local)

		go func() {
			_ = schedulerInstance.Start(context.Background())
		}()

		time.Sleep(time.Second + time.Millisecond*100)
		schedulerInstance.Stop()

		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RunCount.Load())
		require.Equal(t, int32(0), entryReader.Entries[1].Job.(*mockJob).RunCount.Load())
	})
	t.Run("Restart", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		setFixedTime(now)

		entryReader := &mockEntryReader{
			Entries: []*entry{
				{
					EntryType: entryTypeRestart,
					Job:       &mockJob{},
					Next:      now,
					Logger:    test.NewLogger(),
				},
			},
		}

		schedulerInstance := newScheduler(entryReader, test.NewLogger(), testHomeDir, time.Local)

		go func() {
			_ = schedulerInstance.Start(context.Background())
		}()
		defer schedulerInstance.Stop()

		time.Sleep(time.Second + time.Millisecond*100)
		require.Equal(t, int32(1), entryReader.Entries[0].Job.(*mockJob).RestartCount.Load())
	})
	t.Run("NextTick", func(t *testing.T) {
		now := time.Date(2020, 1, 1, 1, 0, 50, 0, time.UTC)
		setFixedTime(now)
		schedulerInstance := newScheduler(&mockEntryReader{}, test.NewLogger(), testHomeDir, time.Local)
		next := schedulerInstance.nextTick(now)
		require.Equal(t, time.Date(2020, 1, 1, 1, 1, 0, 0, time.UTC), next)
	})
	t.Run("FixedTime", func(t *testing.T) {
		fixedTime := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

		setFixedTime(fixedTime)
		require.Equal(t, fixedTime, now())

		// Reset
		setFixedTime(time.Time{})
		require.NotEqual(t, fixedTime, now())
	})
}
