// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package scheduler

import (
	"context"
	"fmt"

	"github.com/dagu-org/dagu/internal/core"
	"github.com/dagu-org/dagu/internal/core/spec"
)

// rehydrateExecutionDAG reloads a full DAG from source before scheduler-owned
// execution or persistence paths use it as an execution snapshot.
func rehydrateExecutionDAG(ctx context.Context, dag *core.DAG, params any, baseConfig string) (*core.DAG, error) {
	fresh, err := spec.ResolveRuntimeParams(ctx, dag, params, spec.ResolveRuntimeParamsOptions{
		BaseConfig: baseConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("rehydrate execution DAG: %w", err)
	}
	return fresh, nil
}
