# Documentation Accuracy Report

This report summarizes the accuracy issues found when comparing docs-next with the original docs and actual codebase.

## Critical Issues Found

### 1. Installation Documentation

**Script Name Error:**
- ❌ New docs: `install.sh`
- ✅ Should be: `installer.sh` (verify actual script name)

**Homebrew Command:**
- ❌ New docs: `brew upgrade dagu`
- ✅ Old docs: `brew upgrade dagu-org/brew/dagu`

### 2. CLI Documentation

**Non-existent Commands:**
The new documentation includes several commands that **do not exist** in the codebase:
- ❌ `dagu logs` command
- ❌ `dagu validate` command  
- ❌ `dagu queue` command with subcommands

**Parameter Syntax Error:**
- ❌ New docs: `dagu start etl.yaml DATE=2024-01-01`
- ✅ Should be: `dagu start etl.yaml -- DATE=2024-01-01`

**Flag Errors:**
- ❌ `--dry` flag for start command (should be separate `dry` command)
- ❌ `--queue` flag for start command (should use `enqueue` command)
- ❌ `--run-id` in old docs should be `--run-id`

### 3. YAML Specification

**Field Name Errors:**
- ❌ `timeout` → ✅ `timeoutSec`
- ❌ `delay` → ✅ `delaySec`
- ❌ `maxCleanUpTime` → ✅ `maxCleanUpTimeSec`

**Type Errors:**
- ❌ `maxOutputSize: "1MB"` (string) → ✅ Should be integer (bytes)

**Structure Errors:**
- ❌ `exitCode` → ✅ `exitCodes` (plural) in retry policy
- ❌ `maxRetries` in repeat policy (doesn't exist)

**Missing Fields:**
- `DefaultParams`
- `InfoMail`
- `id` (for step ID references)
- `packages` (for nix-shell)

### 4. Configuration Documentation

**Field Name Errors:**
- ❌ `dags` → ✅ `paths.dagsDir`
- ❌ `histRetentionDays` as server config (it's DAG-level)
- ❌ `maxActiveRuns` as server config (it's DAG-level)

**Non-existent Configuration:**
- ❌ `logLevel` as server config
- ❌ `DAGU_AUTH_BASIC_ENABLED` environment variable

**Missing Configuration:**
- `server.strictValidation`
- `paths.dagRunsDir`, `paths.queueDir`, `paths.procDir`
- `permissions.writeDAGs`, `permissions.runDAGs`
- Queue configuration structure

## Recommendations

### Immediate Actions

1. **Verify Script Names:**
   ```bash
   # Check actual installation script name
   ls scripts/install*.sh
   ```

2. **Fix CLI Examples:**
   - Remove non-existent commands
   - Fix parameter syntax to use `--`
   - Update flags to match implementation

3. **Correct YAML Fields:**
   - Use correct field names with `Sec` suffix for time values
   - Fix type specifications
   - Add missing fields

4. **Update Configuration:**
   - Use correct nested structure (`paths.dagsDir`)
   - Remove non-existent options
   - Add missing configuration fields

### Documentation Strategy

1. **Create Validation Script:**
   Build a script that validates documentation examples against the actual codebase

2. **Reference the Code:**
   Always check the actual struct definitions:
   - `internal/digraph/definition.go` for DAG/Step fields
   - `internal/config/config.go` for server configuration
   - `cmd/` directory for available commands

3. **Test Examples:**
   Run all examples in the documentation to ensure they work

4. **Version Awareness:**
   Note which version of Dagu the documentation refers to

## Files to Update

Priority files that need immediate correction:

1. `/docs-next/getting-started/installation.md` - Fix script names
2. `/docs-next/reference/cli.md` - Remove non-existent commands, fix syntax
3. `/docs-next/reference/yaml.md` - Fix field names and types
4. `/docs-next/configurations/index.md` - Fix configuration structure
5. `/docs-next/overview/cli.md` - Fix command examples

## Verification Checklist

- [ ] All installation commands tested and working
- [ ] All CLI examples run without error
- [ ] All YAML examples validate correctly
- [ ] All configuration examples are valid
- [ ] No references to non-existent features
- [ ] Field names match code exactly
- [ ] Environment variables verified against code

## Notes

The new documentation structure and presentation is excellent, but accuracy is critical. Users will lose trust if commands don't work as documented. Consider:

1. Adding automated tests for documentation examples
2. Creating a documentation review process
3. Linking documentation updates to code changes
4. Adding "Verified for version X.Y.Z" badges
