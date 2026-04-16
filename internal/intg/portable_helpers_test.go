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

	quotedArgs := make([]string, len(args))
	for i, arg := range args {
		quotedArgs[i] = strconv.Quote(arg)
	}

	if len(quotedArgs) == 0 {
		return fmt.Sprintf("exec:\n      command: %s", strconv.Quote(commandPath))
	}

	return fmt.Sprintf(
		"exec:\n      command: %s\n      args: [%s]",
		strconv.Quote(commandPath),
		strings.Join(quotedArgs, ", "),
	)
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
