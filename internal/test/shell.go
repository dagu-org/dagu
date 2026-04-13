// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package test

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func PosixQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func PowerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func ShellQuote(value string) string {
	if runtime.GOOS == "windows" {
		return PowerShellQuote(value)
	}
	return PosixQuote(value)
}

func PortableSuccessCommand() string {
	return "exit 0"
}

func PortableFailureCommand() string {
	return "exit 1"
}

func PortableOutputCommand(value string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Write-Output %s", PowerShellQuote(value))
	}
	return fmt.Sprintf("printf '%%s\\n' %s", PosixQuote(value))
}

func PortablePwdCommand() string {
	if runtime.GOOS == "windows" {
		return "(Get-Location).Path"
	}
	return "pwd"
}

func PortableShellPath(path string) string {
	if runtime.GOOS == "windows" {
		return filepath.ToSlash(path)
	}
	return path
}

func PortableSleepCommand(d time.Duration) string {
	if runtime.GOOS == "windows" {
		millis := d.Milliseconds()
		if millis <= 0 {
			millis = 1
		}
		return fmt.Sprintf("Start-Sleep -Milliseconds %d", millis)
	}
	return fmt.Sprintf("sleep %s", strconv.FormatFloat(d.Seconds(), 'f', -1, 64))
}

func PortableWaitForFileScript(path string, pollInterval time.Duration) string {
	if runtime.GOOS == "windows" {
		millis := pollInterval.Milliseconds()
		if millis <= 0 {
			millis = 1
		}
		return fmt.Sprintf(`
while (-not (Test-Path %s)) {
  Start-Sleep -Milliseconds %d
}
`, PowerShellQuote(path), millis)
	}
	return fmt.Sprintf(`
while [ ! -f %s ]; do
  %s
done
`, PosixQuote(path), PortableSleepCommand(pollInterval))
}

func PortableWriteFileCommand(path, content string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Set-Content -Path %s -Value %s -NoNewline", PowerShellQuote(path), PowerShellQuote(content))
	}
	return fmt.Sprintf("printf '%%s' %s > %s", PosixQuote(content), PosixQuote(path))
}

func PortableCreateEmptyFileCommand(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("New-Item -ItemType File -Path %s -Force | Out-Null", PowerShellQuote(path))
	}
	return fmt.Sprintf(": > %s", PosixQuote(path))
}

func PortableReadTrimmedFileCommand(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("(Get-Content -Raw -Path %s).TrimEnd(\"`r\", \"`n\")", PowerShellQuote(path))
	}
	return fmt.Sprintf("tr -d '\\r\\n' < %s", PosixQuote(path))
}

func PortableReadFileOrFallbackCommand(path, fallback string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(
			"if (Test-Path %s) { %s } else { Write-Output %s }",
			PowerShellQuote(path),
			PortableReadTrimmedFileCommand(path),
			PowerShellQuote(fallback),
		)
	}
	return fmt.Sprintf("cat %s 2>/dev/null || printf '%%s\\n' %s", PosixQuote(path), PosixQuote(fallback))
}

func PortableFileExistsCommand(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("if (Test-Path %s) { exit 0 } else { exit 1 }", PowerShellQuote(path))
	}
	return fmt.Sprintf("test -f %s", PosixQuote(path))
}

func PortableFileMissingCommand(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("if (-not (Test-Path %s)) { exit 0 } else { exit 1 }", PowerShellQuote(path))
	}
	return fmt.Sprintf("test ! -f %s", PosixQuote(path))
}

func PortableEnvOutputCommand(names ...string) string {
	if len(names) == 0 {
		if runtime.GOOS == "windows" {
			return "Write-Output ''"
		}
		return "printf ''"
	}

	if runtime.GOOS == "windows" {
		refs := make([]string, 0, len(names))
		for _, name := range names {
			refs = append(refs, "$env:"+name)
		}
		return fmt.Sprintf(
			"Write-Output ((@(%s) | ForEach-Object { if ($null -eq $_) { '' } else { [string]$_ } }) -join '|')",
			strings.Join(refs, ", "),
		)
	}

	placeholders := make([]string, 0, len(names))
	values := make([]string, 0, len(names))
	for _, name := range names {
		placeholders = append(placeholders, "%s")
		values = append(values, fmt.Sprintf("${%s:-}", name))
	}
	return fmt.Sprintf("printf '%s' %s", strings.Join(placeholders, "|"), strings.Join(values, " "))
}
