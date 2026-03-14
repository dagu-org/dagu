// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"path/filepath"
	"strings"

	"github.com/dagu-org/dagu/internal/core"
)

// dagSuspendFlagName returns the filename stem used by the file-based suspend
// flag system. This intentionally follows DAG file naming, not dag.Name.
func dagSuspendFlagName(dag *core.DAG) string {
	if dag == nil {
		return ""
	}
	base := strings.TrimSuffix(filepath.Base(dag.Location), filepath.Ext(dag.Location))
	if base != "" && base != "." {
		return base
	}
	return dag.Name
}
