//go:build windows

package config

// init populates the package's defaultWhitelist with common Windows environment
// variable names so they are treated as whitelisted on Windows builds.
import "strings"

// normalizeEnvKey converts to uppercase on Windows for case-insensitive matching.
// Windows environment variables are case-insensitive, but Go maps are not.
func normalizeEnvKey(key string) string {
	return strings.ToUpper(key)
}

func init() {
	// Windows-specific environment variables (all uppercase for case-insensitive matching)
	for _, key := range []string{
		"USERPROFILE",  // Native Home
		"SYSTEMROOT",   // C:\Windows
		"WINDIR",       // Same as SystemRoot
		"SYSTEMDRIVE",  // C:
		"COMSPEC",      // cmd.exe
		"PATHEXT",      // .COM;.EXE;.BAT
		"TEMP",         // Temp dir
		"TMP",          // Temp dir
		"PATH",         // System path
		"PSMODULEPATH", // PowerShell specific
		"HOME",         // Used by Go, Git, and ported tools
	} {
		defaultWhitelist[key] = true
	}
}
