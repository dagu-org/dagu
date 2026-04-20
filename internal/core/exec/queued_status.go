// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import "github.com/dagucloud/dagu/internal/core"

// IsQueuedCatchup reports whether the queued status belongs to a catchup run.
func IsQueuedCatchup(status *DAGRunStatus) bool {
	return status != nil &&
		status.Status == core.Queued &&
		status.TriggerType == core.TriggerTypeCatchUp
}

// IsQueuedAutomata reports whether the queued status belongs to an Automata run.
func IsQueuedAutomata(status *DAGRunStatus) bool {
	return status != nil &&
		status.Status == core.Queued &&
		status.TriggerType == core.TriggerTypeAutomata
}

// PreservedQueueTriggerType returns the trigger type that must be preserved
// when consuming a queued item. All other queued execution paths keep their
// existing behavior and execute as retries.
func PreservedQueueTriggerType(status *DAGRunStatus) core.TriggerType {
	if IsQueuedCatchup(status) {
		return core.TriggerTypeCatchUp
	}
	if IsQueuedAutomata(status) {
		return core.TriggerTypeAutomata
	}
	return core.TriggerTypeUnknown
}
