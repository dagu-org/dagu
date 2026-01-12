//go:build !windows

package config

func normalizeEnvKey(key string) string {
	return key
}

// init populates the package-level defaultWhitelist map with common Unix-like environment variable names.
// It marks typical variables (paths, user, shell, temp dir, terminal, editor preferences, locale, timezone,
// shared library path, and XDG config/data/cache locations) as allowed.
// normalizeEnvKey returns the key as-is on Unix (case-sensitive).
func init() {
	// Unix/Linux/macOS environment variables
	for _, key := range []string{
		"PATH",   // Where to find binaries
		"HOME",   // User's home directory
		"USER",   // Current username (equivalent to USERNAME on Windows)
		"SHELL",  // Path to the current shell (e.g., /bin/bash)
		"TMPDIR", // Standard location for temporary files

		"TERM",   // Crucial: Defines terminal capabilities (colors, cursor)
		"EDITOR", // User's preferred text editor (vim, nano)
		"VISUAL", // User's preferred visual editor

		"LANG",     // System language/encoding
		"LC_ALL",   // Force override for all locale settings
		"LC_CTYPE", // Character classification (fixes encoding issues)
		"TZ",       // TimeZone override

		"LD_LIBRARY_PATH", // Path to shared libraries (.so files)

		"XDG_CONFIG_HOME", // User config (usually ~/.config)
		"XDG_DATA_HOME",   // User data (usually ~/.local/share)
		"XDG_CACHE_HOME",  // User cache (usually ~/.cache)
	} {
		defaultWhitelist[key] = true
	}
}
