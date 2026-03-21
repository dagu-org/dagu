// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileproc

import (
	"time"

	"github.com/dagu-org/dagu/internal/core/exec"
)

func testProcMetaFromRun(ref exec.DAGRunRef) exec.ProcMeta {
	return exec.ProcMeta{
		StartedAt:    time.Now().Unix(),
		Name:         ref.Name,
		DAGRunID:     ref.ID,
		AttemptID:    "attempt_" + ref.ID,
		RootName:     ref.Name,
		RootDAGRunID: ref.ID,
	}
}
