// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package cmdutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindExecutable resolves cmd from PATH first, then falls back to common
// Windows compatibility locations for Git-provided Unix tooling.
func FindExecutable(cmd string) (string, bool) {
	if cmd == "" {
		return "", false
	}
	if path, err := exec.LookPath(cmd); err == nil {
		return path, true
	}
	if runtime.GOOS != "windows" {
		return "", false
	}
	if path := findWindowsCompatExecutable(cmd); path != "" {
		return path, true
	}
	return "", false
}

// ResolveExecutable returns the best-effort resolved executable path for cmd.
// If no compatibility path is found, the original value is returned unchanged.
func ResolveExecutable(cmd string) string {
	if runtime.GOOS != "windows" {
		return cmd
	}
	if path, ok := FindExecutable(cmd); ok {
		return path
	}
	return cmd
}

func findWindowsCompatExecutable(cmd string) string {
	name := strings.ToLower(filepath.Base(strings.ReplaceAll(cmd, "\\", "/")))
	name = strings.TrimSuffix(name, ".exe")

	var candidates []string
	switch name {
	case "bash":
		candidates = windowsGitCandidates("bash.exe")
	case "sh":
		candidates = windowsGitCandidates("sh.exe")
	case "env":
		candidates = windowsGitCandidates("env.exe")
	default:
		return ""
	}

	for _, candidate := range candidates {
		if stat, err := os.Stat(candidate); err == nil && !stat.IsDir() {
			return candidate
		}
	}
	return ""
}

func windowsGitCandidates(exe string) []string {
	var roots []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
		if value := strings.TrimSpace(os.Getenv(env)); value != "" {
			roots = append(roots, value)
		}
	}

	var candidates []string
	for _, root := range roots {
		switch filepath.Base(root) {
		case "Programs":
			candidates = append(candidates,
				filepath.Join(root, "Git", "bin", exe),
				filepath.Join(root, "Git", "usr", "bin", exe),
			)
		default:
			candidates = append(candidates,
				filepath.Join(root, "Git", "bin", exe),
				filepath.Join(root, "Git", "usr", "bin", exe),
				filepath.Join(root, "Programs", "Git", "bin", exe),
				filepath.Join(root, "Programs", "Git", "usr", "bin", exe),
			)
		}
	}

	return candidates
}
