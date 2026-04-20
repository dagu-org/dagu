// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package exec

import (
	"github.com/dagucloud/dagu/internal/core"
	"github.com/dagucloud/dagu/internal/workspace"
)

const WorkspaceLabelKey = "workspace"

// WorkspaceNameFromLabels returns a valid single workspace label value.
//
// Missing, invalid, or conflicting workspace labels are treated as unscoped so
// legacy and malformed DAGs remain visible to all users.
func WorkspaceNameFromLabels(labels core.Labels) (string, bool) {
	var workspaceName string
	for _, value := range labels.Get(WorkspaceLabelKey) {
		if value == "" {
			continue
		}
		if err := workspace.ValidateName(value); err != nil {
			return "", false
		}
		if workspaceName != "" && workspaceName != value {
			return "", false
		}
		workspaceName = value
	}
	return workspaceName, workspaceName != ""
}

// WorkspaceFilter restricts list/search results to allowed workspaces.
type WorkspaceFilter struct {
	Enabled           bool
	Workspaces        []string
	IncludeUnlabelled bool
}

// MatchesLabels reports whether labels are visible under the filter.
func (f *WorkspaceFilter) MatchesLabels(labels core.Labels) bool {
	if f == nil || !f.Enabled {
		return true
	}
	workspaceName, ok := WorkspaceNameFromLabels(labels)
	if !ok {
		return f.IncludeUnlabelled
	}
	for _, allowed := range f.Workspaces {
		if workspaceName == allowed {
			return true
		}
	}
	return false
}
