//go:build !windows

package config

import (
	"os"
	"strings"

	"golang.org/x/text/encoding/ianaindex"
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
		if charset := normalizeEncoding(encoding); charset != "" {
			return charset
		}
	}

	// For locales without explicit encoding, modern systems use UTF-8
	return "utf-8"
}

// normalizeEncoding uses IANA index to get the canonical encoding name.
func normalizeEncoding(name string) string {
	enc, err := ianaindex.IANA.Encoding(name)
	if err != nil || enc == nil {
		return ""
	}
	canonical, err := ianaindex.IANA.Name(enc)
	if err != nil {
		return ""
	}
	return strings.ToLower(canonical)
}
