// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package fileutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrPathEscapesBase indicates that a relative path resolves outside the expected base directory.
var ErrPathEscapesBase = errors.New("path escapes base directory")

// ResolvePathWithinBase resolves relPath under baseDir while preventing lexical path traversal.
func ResolvePathWithinBase(baseDir, relPath string) (string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		return "", fmt.Errorf("base directory is empty")
	}

	cleanRel := filepath.Clean(filepath.FromSlash(relPath))
	if cleanRel == "." || cleanRel == "" || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) {
		return "", ErrPathEscapesBase
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	resolvedAbs, err := filepath.Abs(filepath.Join(baseAbs, cleanRel))
	if err != nil {
		return "", err
	}
	if !pathWithinBase(resolvedAbs, baseAbs) {
		return "", ErrPathEscapesBase
	}
	return resolvedAbs, nil
}

// ResolveExistingPathWithinBase resolves relPath under baseDir and rejects symlinks that escape the base directory.
func ResolveExistingPathWithinBase(baseDir, relPath string) (string, error) {
	resolvedAbs, err := ResolvePathWithinBase(baseDir, relPath)
	if err != nil {
		return "", err
	}

	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	realBase, err := filepath.EvalSymlinks(baseAbs)
	if err != nil {
		return "", err
	}
	realBase, err = filepath.Abs(realBase)
	if err != nil {
		return "", err
	}

	realPath, err := filepath.EvalSymlinks(resolvedAbs)
	if err != nil {
		return "", err
	}
	realPath, err = filepath.Abs(realPath)
	if err != nil {
		return "", err
	}
	if !pathWithinBase(realPath, realBase) {
		return "", ErrPathEscapesBase
	}
	return realPath, nil
}

func pathWithinBase(path, base string) bool {
	return path == base || strings.HasPrefix(path, base+string(filepath.Separator))
}

// IsSymlinkDirEntry reports whether the entry itself is a symbolic link.
func IsSymlinkDirEntry(entry os.DirEntry) bool {
	if entry == nil {
		return false
	}
	return entry.Type()&os.ModeSymlink != 0
}
