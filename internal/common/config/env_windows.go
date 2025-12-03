//go:build windows

package config

func init() {
	// Windows-specific environment variables
	for _, key := range []string{
		"USERPROFILE", // Native Home
		"SystemRoot",  // C:\Windows
		"WINDIR",      // Same as SystemRoot
		"SystemDrive", // C:
		"COMSPEC",     // cmd.exe
		"PATHEXT",     // .COM;.EXE;.BAT
		"TEMP",        // Temp dir
		"TMP",         // Temp dir

		"Path",         // Native Windows casing
		"PATH",         // Uppercase compatibility
		"PSModulePath", // PowerShell specific

		"HOME", // Used by Go, Git, and ported tools
	} {
		defaultWhitelist[key] = true
	}
}
