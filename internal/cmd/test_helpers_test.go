// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmd_test

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dagucloud/dagu/internal/test"
	"github.com/stretchr/testify/require"
)

func cancelWhenLogContains(t *testing.T, th test.Command, want ...string) {
	t.Helper()

	done := make(chan bool, 1)
	go func() {
		deadline := time.Now().Add(commandLogWaitTimeout())
		for time.Now().Before(deadline) {
			out := th.LoggingOutput.String()
			matched := true
			for _, token := range want {
				if !strings.Contains(out, token) {
					matched = false
					break
				}
			}
			if matched {
				th.Cancel()
				done <- true
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
		th.Cancel()
		done <- false
	}()

	t.Cleanup(func() {
		require.True(t, <-done, "startup log never appeared: %v", want)
	})
}

func commandLogWaitTimeout() time.Duration {
	if runtime.GOOS == "windows" {
		return 30 * time.Second
	}
	return 10 * time.Second
}
