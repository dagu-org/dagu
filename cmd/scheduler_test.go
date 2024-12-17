// Copyright (C) 2024 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"testing"
	"time"

	"github.com/dagu-org/dagu/internal/test"
)

func TestSchedulerCommand(t *testing.T) {
	t.Run("StartScheduler", func(t *testing.T) {
		setup := test.SetupTest(t)
		defer setup.Cleanup()

		go func() {
			testRunCommand(t, schedulerCmd(), cmdTest{
				args:        []string{"scheduler"},
				expectedOut: []string{"starting dagu scheduler"},
			})
		}()

		time.Sleep(time.Millisecond * 500)
	})
}
