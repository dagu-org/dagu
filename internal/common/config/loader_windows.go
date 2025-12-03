//go:build windows

package config

import (
	"golang.org/x/sys/windows"
)

// codePageCharsets maps Windows code page numbers to charset names.
var codePageCharsets = map[uint32]string{
	932:   "shift_jis",    // Japanese
	936:   "gbk",          // Simplified Chinese
	949:   "euc-kr",       // Korean
	950:   "big5",         // Traditional Chinese
	874:   "windows-874",  // Thai
	1250:  "windows-1250", // Central European (Czech, Hungarian, Polish, etc.)
	1251:  "windows-1251", // Cyrillic (Russian, Bulgarian, etc.)
	1252:  "windows-1252", // Western European (English, German, French, Spanish, etc.)
	1253:  "windows-1253", // Greek
	1254:  "windows-1254", // Turkish
	1255:  "windows-1255", // Hebrew
	1256:  "windows-1256", // Arabic
	1257:  "windows-1257", // Baltic (Estonian, Latvian, Lithuanian)
	1258:  "windows-1258", // Vietnamese
	65001: "utf-8",        // UTF-8 (Windows 10 1903+ can use UTF-8 as system locale)
}

// getDefaultLogEncodingCharset returns the default log encoding charset for Windows
// by detecting the system's ANSI code page.
func getDefaultLogEncodingCharset() string {
	acp := windows.GetACP()
	return codePageToCharset(acp)
}

// codePageToCharset maps Windows code page numbers to charset names.
func codePageToCharset(codePage uint32) string {
	if charset, ok := codePageCharsets[codePage]; ok {
		return charset
	}
	return "utf-8"
}
