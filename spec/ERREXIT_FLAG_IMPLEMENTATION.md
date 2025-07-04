# Enable errexit (-e) Flag by Default for Shell Executors

## Overview

This specification describes the implementation of automatic errexit (`-e`) flag for shell executors in Dagu. The feature aims to prevent multi-line scripts from continuing execution after a command fails, improving reliability and aligning with modern CI/CD best practices.

## Problem Statement

Currently, when executing multi-line shell commands in Dagu, if one command fails, subsequent commands continue to execute:

```yaml
steps:
  - name: dangerous-example
    command: |
      false  # This fails with exit code 1
      rm -rf /important/data  # This still executes!
```

This behavior can lead to:
- Silent failures in workflows
- Data corruption or loss
- Difficult debugging of failed workflows
- Inconsistent state in systems

## Solution

Enable the errexit flag (`-e`) by default for shell executors, but **only when the user has not explicitly specified a shell**.

### Core Principle

1. **User specifies `shell` field**: Use exactly what they specified, no modifications
2. **User doesn't specify `shell` field**: Add `-e` flag to the default shell

### Examples

```yaml
# Case 1: No shell specified - errexit enabled automatically
steps:
  - name: safe-by-default
    command: |
      false
      echo "This won't execute"  # Script stops at false
# Executes as: /bin/sh -e -c "..."

# Case 2: User specifies shell - respect their choice
steps:
  - name: user-controlled
    shell: bash
    command: |
      false
      echo "This will execute"  # No errexit, continues
# Executes as: bash -c "..."

# Case 3: User explicitly wants errexit
steps:
  - name: explicit-safety
    shell: bash -e
    command: |
      false
      echo "This won't execute"
# Executes as: bash -e -c "..."
```

## Implementation Details

### 1. Affected Files

- `/internal/digraph/executor/command.go` - Main implementation
- `/internal/digraph/executor/command_test.go` - Tests
- `/docs/features/executors/shell.md` - Documentation updates

### 2. Code Changes

#### 2.1 Modify `createCommandConfig` function

```go
func createCommandConfig(ctx context.Context, step digraph.Step) (*commandConfig, error) {
    var shellCommand string
    
    if step.Shell != "" {
        // User explicitly set shell - respect their choice exactly
        shellCommand = cmdutil.GetShellCommand(step.Shell)
    } else {
        // No shell specified - use default with errexit
        defaultShell := cmdutil.GetShellCommand("")
        
        // Add errexit flag for Unix-like shells
        if isUnixLikeShell(defaultShell) {
            shellCommand = defaultShell + " -e"
        } else {
            shellCommand = defaultShell
        }
    }
    
    shellCmdArgs := step.ShellCmdArgs
    
    return &commandConfig{
        Ctx:              ctx,
        Dir:              GetEnv(ctx).WorkingDir,
        Command:          step.Command,
        Args:             step.Args,
        Script:           step.Script,
        ShellCommand:     shellCommand,
        ShellCommandArgs: shellCmdArgs,
        ShellPackages:    step.ShellPackages,
    }, nil
}
```

#### 2.2 Add helper function

```go
// isUnixLikeShell returns true if the shell supports -e flag
func isUnixLikeShell(shell string) bool {
    if shell == "" {
        return false
    }
    
    // Extract just the executable name (handle full paths)
    shellName := filepath.Base(shell)
    
    switch shellName {
    case "sh", "bash", "zsh", "ksh", "ash", "dash":
        return true
    case "fish":
        // Fish shell doesn't support -e flag
        return false
    default:
        return false
    }
}
```

#### 2.3 Script file execution

When executing script files, also consider errexit:

```go
case cfg.ShellCommand != "" && scriptFile != "":
    // Determine if we should add -e based on whether user specified shell
    if needsErrexitForScript(ctx, step) {
        cmd = exec.CommandContext(cfg.Ctx, cfg.ShellCommand, "-e", scriptFile)
    } else {
        cmd = exec.CommandContext(cfg.Ctx, cfg.ShellCommand, scriptFile)
    }
```

### 3. Special Cases

#### 3.1 Nix-shell

For nix-shell, errexit must be applied to the inner shell:

```go
case "nix-shell":
    // ... existing package handling ...
    
    if userDidNotSpecifyShell(ctx) {
        // For inline commands, prepend set -e
        b.ShellCommandArgs = "set -e; " + b.ShellCommandArgs
    }
```

#### 3.2 Environment Variables

The implementation respects these environment variables in order:
1. Step-level `shell` field (if specified)
2. `DAGU_DEFAULT_SHELL` environment variable
3. System `$SHELL` environment variable
4. Platform defaults (sh on Unix, PowerShell on Windows)

### 4. Shell Support Matrix

| Shell | Supports -e | Implementation |
|-------|-------------|----------------|
| sh | Yes | Add -e flag |
| bash | Yes | Add -e flag |
| zsh | Yes | Add -e flag |
| ksh | Yes | Add -e flag |
| dash | Yes | Add -e flag |
| ash | Yes | Add -e flag |
| fish | No | No modification |
| PowerShell | No | No modification |
| cmd.exe | No | No modification |

## Testing Strategy

### 1. Unit Tests

```go
func TestErrexitDefaultBehavior(t *testing.T) {
    tests := []struct {
        name        string
        step        digraph.Step
        shouldFail  bool
        shouldAddE  bool
    }{
        {
            name: "no shell specified - adds errexit",
            step: digraph.Step{
                Command: "false\necho 'should not print'",
            },
            shouldFail: true,
            shouldAddE: true,
        },
        {
            name: "shell specified - no errexit",
            step: digraph.Step{
                Shell:   "bash",
                Command: "false\necho 'should print'",
            },
            shouldFail: false,
            shouldAddE: false,
        },
        {
            name: "explicit errexit",
            step: digraph.Step{
                Shell:   "bash -e",
                Command: "false\necho 'should not print'",
            },
            shouldFail: true,
            shouldAddE: false, // Already has -e
        },
    }
    
    // Test implementation...
}
```

### 2. Integration Tests

- Test with real multi-line scripts
- Test with various shell types
- Test with environment variables
- Test script file execution
- Test nix-shell integration

### 3. Backward Compatibility Tests

Ensure existing DAGs continue to work:
- DAGs with explicit `shell:` field
- DAGs relying on error continuation
- DAGs with custom error handling

## Migration Guide

### For Users

#### No Action Required If:
- You explicitly set the `shell` field in your steps
- You want the new safer behavior for steps without `shell` field

#### Action Required If:
- You have steps without `shell` field that rely on continuing after errors
- Solution: Add `shell: bash` (or your preferred shell) to those steps

#### Examples:

```yaml
# Before: This might break with the update
steps:
  - name: cleanup
    command: |
      rm -f /tmp/file1  # Might fail if doesn't exist
      rm -f /tmp/file2  # Previously would continue
      
# After: Explicitly disable errexit
steps:
  - name: cleanup
    shell: bash  # Or use: shell: bash +e
    command: |
      rm -f /tmp/file1
      rm -f /tmp/file2
      
# Alternative: Use proper error handling
steps:
  - name: cleanup
    command: |
      rm -f /tmp/file1 || true
      rm -f /tmp/file2 || true
```

### Documentation Updates

1. Remove `set -e` from examples (no longer needed)
2. Add section explaining errexit is default
3. Show how to disable with explicit `shell:` field
4. Update best practices guide

## Security Considerations

This change improves security by:
- Preventing unintended command execution after failures
- Reducing risk of partial state changes
- Making failures more visible and debuggable

## Performance Impact

Minimal - only adds one flag to shell invocation.

## Rollout Plan

1. **v1.XX.0**: Feature released with clear documentation
2. **Migration period**: Users update DAGs if needed
3. **Monitoring**: Track any issues or unexpected behaviors

## Future Considerations

1. **Configuration option**: Consider adding global config to disable this behavior
2. **Per-DAG override**: Allow DAGs to specify default behavior
3. **Enhanced error messages**: Clearly indicate when errexit stops execution

## Decision Log

- **Why not modify user-specified shells?** Respects user intent and prevents breaking changes
- **Why add to default shells only?** Provides safety while maintaining flexibility
- **Why not make it configurable?** Follows principle of safe defaults; users can opt-out via `shell:` field

## References

- GitHub Issue: #1081
- Shell errexit documentation: `man sh` (see `-e` flag)
- POSIX specification: IEEE Std 1003.1-2017