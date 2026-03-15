// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

//go:build windows

package terminal

import (
	"os/exec"
	"syscall"

	"github.com/dagu-org/dagu/internal/cmn/cmdutil"
)

func requestHangup(_ *exec.Cmd) error {
	return nil
}

func forceKillProcess(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}
	return cmdutil.KillProcessGroup(cmd, syscall.SIGKILL)
}
