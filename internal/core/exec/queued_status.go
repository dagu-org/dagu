// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import "github.com/dagu-org/dagu/internal/core"

// IsQueuedCatchup reports whether the queued status belongs to a catchup run.
func IsQueuedCatchup(status *DAGRunStatus) bool {
	return status != nil &&
		status.Status == core.Queued &&
		status.TriggerType == core.TriggerTypeCatchUp
}

// PreservedQueueTriggerType returns the trigger type that must be preserved
// when consuming a queued item. Only catchup is preserved for now; all other
// queued execution paths keep their existing behavior.
func PreservedQueueTriggerType(status *DAGRunStatus) core.TriggerType {
	if IsQueuedCatchup(status) {
		return core.TriggerTypeCatchUp
	}
	return core.TriggerTypeUnknown
}
