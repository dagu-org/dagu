//go:build windows

package config

import (
	"fmt"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/ianaindex"
)

// getDefaultLogEncodingCharset returns the default log encoding charset for Windows
// by detecting the system's ANSI code page.
func getDefaultLogEncodingCharset() string {
	acp := windows.GetACP()
	return codePageToCharset(acp)
}

// codePageToCharset converts a Windows code page number to a canonical charset name
// using the IANA index for normalization.
func codePageToCharset(codePage uint32) string {
	// Map code page number to a charset name that ianaindex can recognize
	var name string
	switch codePage {
	case 65001:
		return "utf-8"
	case 932:
		name = "shift_jis"
	case 936:
		name = "gbk"
	case 949:
		name = "euc-kr"
	case 950:
		name = "big5"
	default:
		// For Windows code pages (874, 1250-1258), use "windows-XXX" format
		name = fmt.Sprintf("windows-%d", codePage)
	}

	// Normalize the name using IANA index
	enc, err := ianaindex.IANA.Encoding(name)
	if err != nil || enc == nil {
		return "utf-8"
	}
	canonical, err := ianaindex.IANA.Name(enc)
	if err != nil {
		return "utf-8"
	}
	return strings.ToLower(canonical)
}
