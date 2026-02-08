package spec

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildSchedulerFromEntries(t *testing.T) {
	t.Parallel()

	t.Run("CatchupAllWithExplicitWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "all", CatchupWindow: "6h"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyAll, schedules[0].Catchup)
		assert.Equal(t, 6*time.Hour, schedules[0].CatchupWindow)
	})

	t.Run("CatchupAllDefaultWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "all"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyAll, schedules[0].Catchup)
		assert.Equal(t, 24*time.Hour, schedules[0].CatchupWindow, "should default to 24h")
	})

	t.Run("CatchupLatestDefaultWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "latest"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyLatest, schedules[0].Catchup)
		assert.Equal(t, 24*time.Hour, schedules[0].CatchupWindow, "should default to 24h for latest too")
	})

	t.Run("CatchupOffNoWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "off"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyOff, schedules[0].Catchup)
		assert.Equal(t, time.Duration(0), schedules[0].CatchupWindow, "off policy should have zero window")
	})

	t.Run("NoCatchupNoWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyOff, schedules[0].Catchup)
		assert.Equal(t, time.Duration(0), schedules[0].CatchupWindow)
	})

	t.Run("InvalidCron", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "not-a-cron"},
		}
		_, err := buildSchedulerFromEntries(entries)
		require.Error(t, err)
	})

	t.Run("InvalidCatchupPolicy", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "invalid"},
		}
		_, err := buildSchedulerFromEntries(entries)
		require.Error(t, err)
	})

	t.Run("InvalidCatchupWindow", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "all", CatchupWindow: "not-a-duration"},
		}
		_, err := buildSchedulerFromEntries(entries)
		require.Error(t, err)
	})

	t.Run("CaseInsensitiveCatchup", func(t *testing.T) {
		t.Parallel()

		entries := []types.ScheduleEntry{
			{Cron: "0 * * * *", Catchup: "ALL"},
		}
		schedules, err := buildSchedulerFromEntries(entries)
		require.NoError(t, err)
		require.Len(t, schedules, 1)

		assert.Equal(t, core.CatchupPolicyAll, schedules[0].Catchup)
	})
}
