//go:build !windows

package config

import (
	"os"
	"strings"
)

// unixEncodingCharsets maps normalized Unix encoding names to standard charset names.
// Keys should be lowercase with hyphens and underscores removed.
var unixEncodingCharsets = map[string]string{
	// UTF-8 variants
	"utf8": "utf-8",

	// Japanese
	"eucjp":     "euc-jp",
	"ujis":      "euc-jp",
	"shiftjis":  "shift_jis",
	"sjis":      "shift_jis",
	"cp932":     "shift_jis",
	"ms932":     "shift_jis",
	"iso2022jp": "iso-2022-jp",

	// Chinese
	"eucch":     "gbk",
	"euccn":     "gbk",
	"gb2312":    "gbk",
	"gb18030":   "gbk",
	"gbk":       "gbk",
	"euctw":     "big5",
	"big5":      "big5",
	"big5hkscs": "big5",

	// Korean
	"euckr": "euc-kr",

	// ISO-8859 variants
	"iso88591":  "iso-8859-1",
	"latin1":    "iso-8859-1",
	"iso88592":  "iso-8859-2",
	"latin2":    "iso-8859-2",
	"iso88595":  "iso-8859-5",
	"iso88596":  "iso-8859-6",
	"iso88597":  "iso-8859-7",
	"iso88598":  "iso-8859-8",
	"iso88599":  "iso-8859-9",
	"latin5":    "iso-8859-9",
	"iso885915": "iso-8859-15",
	"latin9":    "iso-8859-15",

	// Cyrillic
	"koi8r": "koi8-r",
	"koi8u": "koi8-u",
}

// getDefaultLogEncodingCharset returns the default log encoding charset for Unix systems
// by detecting the system locale from environment variables.
func getDefaultLogEncodingCharset() string {
	// Check LC_ALL first, then LANG (standard precedence)
	locale := os.Getenv("LC_ALL")
	if locale == "" {
		locale = os.Getenv("LANG")
	}
	if locale == "" {
		return "utf-8"
	}

	locale = strings.ToLower(locale)

	// Check for explicit encoding suffix (e.g., "ja_JP.UTF-8", "ja_JP.eucJP")
	if idx := strings.LastIndex(locale, "."); idx != -1 {
		encoding := locale[idx+1:]
		// Remove any modifier (e.g., "@euro")
		if modIdx := strings.Index(encoding, "@"); modIdx != -1 {
			encoding = encoding[:modIdx]
		}
		if charset := normalizeUnixEncoding(encoding); charset != "" {
			return charset
		}
	}

	// For locales without explicit encoding, modern systems use UTF-8
	return "utf-8"
}

// normalizeUnixEncoding maps Unix encoding names to standard charset names.
func normalizeUnixEncoding(encoding string) string {
	encoding = strings.ToLower(encoding)
	encoding = strings.ReplaceAll(encoding, "-", "")
	encoding = strings.ReplaceAll(encoding, "_", "")

	if charset, ok := unixEncodingCharsets[encoding]; ok {
		return charset
	}
	return ""
}
