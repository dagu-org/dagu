//go:build windows

package config

import (
	"golang.org/x/sys/windows"
)

// getDefaultLogEncodingCharset returns the default log encoding charset for Windows
// by detecting the system's ANSI code page.
func getDefaultLogEncodingCharset() string {
	acp := windows.GetACP()
	return codePageToCharset(acp)
}

// codePageToCharset maps Windows code page numbers to charset names.
func codePageToCharset(codePage uint32) string {
	switch codePage {
	// Japanese
	case 932:
		return "shift_jis"

	// Simplified Chinese
	case 936:
		return "gbk"

	// Korean
	case 949:
		return "euc-kr"

	// Traditional Chinese
	case 950:
		return "big5"

	// Central European (Czech, Hungarian, Polish, etc.)
	case 1250:
		return "windows-1250"

	// Cyrillic (Russian, Bulgarian, etc.)
	case 1251:
		return "windows-1251"

	// Western European (English, German, French, Spanish, etc.)
	case 1252:
		return "windows-1252"

	// Greek
	case 1253:
		return "windows-1253"

	// Turkish
	case 1254:
		return "windows-1254"

	// Hebrew
	case 1255:
		return "windows-1255"

	// Arabic
	case 1256:
		return "windows-1256"

	// Baltic (Estonian, Latvian, Lithuanian)
	case 1257:
		return "windows-1257"

	// Vietnamese
	case 1258:
		return "windows-1258"

	// Thai
	case 874:
		return "windows-874"

	// UTF-8 (Windows 10 1903+ can use UTF-8 as system locale)
	case 65001:
		return "utf-8"

	// Default to UTF-8 for unknown code pages
	default:
		return "utf-8"
	}
}
