//go:build !windows

package config

import (
	"os"
	"strings"
)

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

	// If no explicit encoding, infer from locale prefix
	return inferEncodingFromLocale(locale)
}

// normalizeUnixEncoding maps Unix encoding names to standard charset names.
func normalizeUnixEncoding(encoding string) string {
	encoding = strings.ToLower(encoding)
	encoding = strings.ReplaceAll(encoding, "-", "")
	encoding = strings.ReplaceAll(encoding, "_", "")

	switch encoding {
	// UTF-8 variants
	case "utf8":
		return "utf-8"

	// Japanese
	case "eucjp", "ujis":
		return "euc-jp"
	case "shiftjis", "sjis":
		return "shift_jis"
	case "iso2022jp":
		return "iso-2022-jp"

	// Chinese
	case "eucch", "euccn", "gb2312", "gb18030", "gbk":
		return "gbk"
	case "euctw", "big5", "big5hkscs":
		return "big5"

	// Korean
	case "euckr":
		return "euc-kr"

	// ISO-8859 variants
	case "iso88591", "latin1":
		return "iso-8859-1"
	case "iso88592", "latin2":
		return "iso-8859-2"
	case "iso88595":
		return "iso-8859-5"
	case "iso88596":
		return "iso-8859-6"
	case "iso88597":
		return "iso-8859-7"
	case "iso88598":
		return "iso-8859-8"
	case "iso88599", "latin5":
		return "iso-8859-9"
	case "iso885915", "latin9":
		return "iso-8859-15"

	// Cyrillic
	case "koi8r":
		return "koi8-r"
	case "koi8u":
		return "koi8-u"

	default:
		return ""
	}
}

// inferEncodingFromLocale attempts to infer encoding from locale prefix
// when no explicit encoding is specified.
func inferEncodingFromLocale(locale string) string {
	// Extract language/country part (e.g., "ja_jp" from "ja_jp.utf-8")
	if idx := strings.Index(locale, "."); idx != -1 {
		locale = locale[:idx]
	}
	if idx := strings.Index(locale, "@"); idx != -1 {
		locale = locale[:idx]
	}

	// For locales without explicit encoding, modern systems typically use UTF-8
	// But for historical compatibility, we can check specific locales
	switch {
	case strings.HasPrefix(locale, "ja_"):
		// Japanese: historically EUC-JP on Unix, but modern systems use UTF-8
		return "utf-8"
	case strings.HasPrefix(locale, "ko_"):
		// Korean: historically EUC-KR on Unix
		return "utf-8"
	case strings.HasPrefix(locale, "zh_cn"), strings.HasPrefix(locale, "zh_sg"):
		// Simplified Chinese
		return "utf-8"
	case strings.HasPrefix(locale, "zh_tw"), strings.HasPrefix(locale, "zh_hk"):
		// Traditional Chinese
		return "utf-8"
	case strings.HasPrefix(locale, "ru_"):
		// Russian
		return "utf-8"
	default:
		// Default to UTF-8 for modern systems
		return "utf-8"
	}
}
