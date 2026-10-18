// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package wait

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dagucloud/dagu/v2/internal/cmn/value"
	"github.com/dagucloud/dagu/v2/internal/ir"
	"github.com/dagucloud/dagu/v2/internal/runtime"
)

// dryRunCheck reports a wait.file target that already fails on this host:
// a file that must exist but does not. A with field still holding a
// reference, such as a step output, is skipped.
func dryRunCheck(ctx context.Context, step ir.Step) error {
	cfg := defaultConfig()
	if err := decodeConfig(step.ExecutorConfig.Config, &cfg); err != nil {
		return nil
	}
	if value.HasValueReference(cfg.Path) || value.HasValueReference(cfg.State) {
		return nil
	}
	if stepOperation(step) != opFile || cfg.State != stateExists {
		return nil
	}
	path := cfg.Path
	if !filepath.IsAbs(path) {
		if workDir := runtime.GetEnv(ctx).WorkingDir; workDir != "" {
			path = filepath.Join(workDir, path)
		}
	}
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("field 'with.path': %s: file not found", cfg.Path)
	}
	return nil
}
