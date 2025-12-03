//go:build !windows

package command

// normalizeScriptPath returns the shell command args unchanged on Unix systems.
//
// On Unix, shells don't automatically search the current directory for executables,
// and users are expected to use "./" explicitly. This is the expected Unix behavior
// and doesn't need correction (unlike Windows where this can be surprising).
func (b *shellCommandBuilder) normalizeScriptPath() string {
	return b.ShellCommandArgs
}
