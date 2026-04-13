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

func PortableStderrCommand(value string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("[Console]::Error.WriteLine(%s)", PowerShellQuote(value))
	}
	return fmt.Sprintf("printf '%%s\\n' %s 1>&2", PosixQuote(value))
}

func PortableCommandSequence(commands ...string) string {
	nonEmpty := make([]string, 0, len(commands))
	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}
		nonEmpty = append(nonEmpty, command)
	}
	return strings.Join(nonEmpty, "\n")
}

func PortableCommandSubstitution(command string) string {
	return "`" + command + "`"
}

func portableEnvNameFromRef(ref string) (string, bool) {
	var name string
	switch {
	case strings.HasPrefix(ref, "${") && strings.HasSuffix(ref, "}"):
		name = ref[2 : len(ref)-1]
	case strings.HasPrefix(ref, "$"):
		name = ref[1:]
	default:
		return "", false
	}

	if name == "" {
		return "", false
	}

	for i, r := range name {
		switch {
		case r == '_':
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return "", false
		}
	}

	return name, true
}

func portablePowerShellEnvExpr(ref string) (string, bool) {
	name, ok := portableEnvNameFromRef(ref)
	if !ok {
		return "", false
	}
	return "$env:" + name, true
}

// PortableExpandedOutputCommand emits a Dagu-resolved ${...} value while keeping
// shell quoting valid on each platform. The input should be a Dagu reference,
// not an arbitrary literal string.
func PortableExpandedOutputCommand(ref string) string {
	if runtime.GOOS == "windows" {
		if expr, ok := portablePowerShellEnvExpr(ref); ok {
			return "Write-Output " + expr
		}
		return fmt.Sprintf("Write-Output %s", PowerShellQuote(ref))
	}
	return fmt.Sprintf("printf '%%s\\n' \"%s\"", ref)
}

func PortableLabeledExpandedOutputCommand(prefix, ref string) string {
	if runtime.GOOS == "windows" {
		if expr, ok := portablePowerShellEnvExpr(ref); ok {
			return fmt.Sprintf("Write-Output (%s + [string](%s))", PowerShellQuote(prefix), expr)
		}
		return fmt.Sprintf("Write-Output (%s + %s)", PowerShellQuote(prefix), PowerShellQuote(ref))
	}
	return fmt.Sprintf("printf '%%s\\n' \"%s%s\"", prefix, ref)
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
		return fmt.Sprintf("([string](Get-Content -Raw -Path %s)).TrimEnd([char]13, [char]10)", PowerShellQuote(path))
	}
	return fmt.Sprintf("tr -d '\\r\\n' < %s", PosixQuote(path))
}

func PortableReadFileCommand(path string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("Get-Content -Raw -Path %s", PowerShellQuote(path))
	}
	return fmt.Sprintf("cat %s", PosixQuote(path))
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

func PortableEnvOutputCommandWithSeparator(separator string, names ...string) string {
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
			"Write-Output ((@(%s) | ForEach-Object { if ($null -eq $_) { '' } else { [string]$_ } }) -join %s)",
			strings.Join(refs, ", "),
			PowerShellQuote(separator),
		)
	}

	placeholders := make([]string, 0, len(names))
	values := make([]string, 0, len(names))
	for _, name := range names {
		placeholders = append(placeholders, "%s")
		values = append(values, fmt.Sprintf("${%s:-}", name))
	}
	return fmt.Sprintf("printf '%s' %s", strings.Join(placeholders, separator), strings.Join(values, " "))
}

func PortableEnvOutputCommand(names ...string) string {
	return PortableEnvOutputCommandWithSeparator("|", names...)
}
