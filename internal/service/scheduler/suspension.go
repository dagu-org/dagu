// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/exec"
)

func isSchedulerManagedTriggerType(triggerType core.TriggerType) bool {
	switch triggerType {
	case core.TriggerTypeScheduler, core.TriggerTypeCatchUp, core.TriggerTypeRetry:
		return true
	default:
		return false
	}
}

func suspendFlagName(status *exec.DAGRunStatus, dag *core.DAG) string {
	if status != nil && status.SuspendFlagName != "" {
		return status.SuspendFlagName
	}
	if dag != nil {
		if name := dagSuspendFlagName(dag); name != "" {
			return name
		}
	}
	if status != nil {
		return status.Name
	}
	return ""
}

func isSuspendedDAG(
	ctx context.Context,
	isSuspended IsSuspendedFunc,
	status *exec.DAGRunStatus,
	dag *core.DAG,
) bool {
	if isSuspended == nil {
		return false
	}
	name := suspendFlagName(status, dag)
	if name == "" {
		return false
	}
	return isSuspended(ctx, name)
}
