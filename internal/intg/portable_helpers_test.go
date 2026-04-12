// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package intg_test

import (
	"fmt"
	osexec "os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func directCommandYAML(t *testing.T, commandName string, args ...string) string {
	t.Helper()

	commandPath, err := osexec.LookPath(commandName)
	require.NoError(t, err)

	elements := append([]string{commandPath}, args...)
	quoted := make([]string, len(elements))
	for i, element := range elements {
		quoted[i] = strconv.Quote(element)
	}

	return fmt.Sprintf("shell: direct\n    command: [%s]", strings.Join(quoted, ", "))
}

func portableDirectSuccessStepYAML(t *testing.T) string {
	t.Helper()
	return directCommandYAML(t, "whoami")
}

func portableDirectFailureStepYAML(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		return directCommandYAML(t, "cmd", "/c", "exit", "1")
	}
	return directCommandYAML(t, "false")
}
