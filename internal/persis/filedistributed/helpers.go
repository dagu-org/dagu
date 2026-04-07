// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package filedistributed

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/dagucloud/dagu/internal/cmn/fileutil"
)

const filePermissions = 0600

func ensureDir(dir string) error {
	return os.MkdirAll(dir, 0750)
}

func writeJSONAtomic(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal json: %w", err)
	}
	if err := ensureDir(filepath.Dir(path)); err != nil {
		return fmt.Errorf("create parent dir: %w", err)
	}
	if err := fileutil.WriteFileAtomic(path, data, filePermissions); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path) //nolint:gosec // internal controlled file path
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func sortedFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func encodeKey(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
