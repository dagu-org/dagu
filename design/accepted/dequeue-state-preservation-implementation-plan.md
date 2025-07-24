# Dequeue State Preservation Implementation Plan

## Overview

This document outlines the implementation plan for issue #1117: "Dequeue should restore previous DAG state instead of showing canceled". The goal is to modify the dequeue behavior so that when a queued DAG run is dequeued, the UI shows the previous successful state rather than "Canceled".

## Problem Statement

### Current Behavior
- When a DAG run is dequeued, the system sets the status to "Canceled"
- This overwrites the previous state (e.g., "Success")
- Users lose visibility into the actual outcome of the previous run

### Desired Behavior
- When dequeued, the queued attempt should be hidden
- The UI should display the previous state (e.g., "Success")
- The dequeued attempt should be preserved for audit purposes

## Technical Approach

The solution involves renaming the queued attempt directory with a dot prefix (`.`) to hide it from normal operations while preserving the data.

### Example Directory Structure

**Before Dequeue:**
```
dag-run_20240115_100000Z_abc123/
├── attempt_20240115_100000_000Z_att1/ (Status: Success)
└── attempt_20240115_110000_000Z_att2/ (Status: Queued)
```

**After Dequeue:**
```
dag-run_20240115_100000Z_abc123/
├── attempt_20240115_100000_000Z_att1/ (Status: Success)
└── .attempt_20240115_110000_000Z_att2/ (Status: Queued - hidden)
```

## Implementation Steps

### Phase 1: Core Infrastructure (Priority: High)

#### 1.1 Analyze Current Implementation
- **Files to examine:**
  - `/internal/cmd/dequeue.go` - Dequeue command logic
  - `/internal/persistence/filedagrun/` - File-based persistence layer
  - `/internal/models/dagrun.go` - DAG run data structures
  - `/internal/persistence/interface.go` - Persistence interfaces

- **Key areas to understand:**
  - How dequeue currently sets status to Cancel
  - How attempts are stored and retrieved
  - How the UI determines which attempt to display
  - Directory naming conventions

#### 1.2 Design the Hide() Method
- **Location:** Add to the `Attempt` type in `/internal/persistence/filedagrun/attempt.go`
- **Interface Update:** Add method to `DAGRunAttempt` interface in `/internal/models/dagrun.go`
- **Functionality:**
  - Rename attempt directory by adding `.` prefix
  - Preserve all data within the directory
  - Return error if operation fails
  - Support idempotency (handle already hidden attempts)

**Interface Addition:**
```go
// In /internal/models/dagrun.go - DAGRunAttempt interface
type DAGRunAttempt interface {
    // ... existing methods ...
    
    // Hide marks the attempt as dequeued by renaming its directory
    Hide(ctx context.Context) error
}
```

**Method Signatures:**
```go
// Hide renames the attempt directory to hide it from normal operations
func (a *Attempt) Hide(ctx context.Context) error {
    // Implementation details
}

// Path returns the directory path of the attempt
func (a *Attempt) Path() string {
    // Extract directory from status file path
}
```

#### 1.3 Implement the Hide() Method
- **File:** `/internal/persistence/filedagrun/attempt.go`
- **Constants to add:** Define the dequeued prefix constant
- **Implementation considerations:**
  - Use atomic file operations to prevent data loss
  - Handle concurrent access with existing mutex
  - Validate directory exists before renaming
  - Log the operation for debugging
  - Handle case where attempt is currently open

**Constants Addition:**
```go
// In /internal/persistence/filedagrun/dagrun.go
const (
    AttemptDirPrefix   = "attempt_"
    // No need for a special dequeued prefix - just use dot prefix
)
```

**Implementation:**
```go
// Path returns the directory path of the attempt
func (a *Attempt) Path() string {
    return filepath.Dir(a.file)
}

// Hide renames the attempt directory to hide it from normal operations
func (a *Attempt) Hide(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    // Check if attempt is currently open
    if a.writer != nil {
        return errors.New("cannot hide an open attempt")
    }
    
    // Get current and new paths
    currentDir := a.Path()
    baseName := filepath.Base(currentDir)
    
    // Check if already hidden (idempotent)
    if strings.HasPrefix(baseName, ".") {
        return nil
    }
    
    // Simply add a dot prefix to hide the directory
    newBaseName := "." + baseName
    
    newDir := filepath.Join(filepath.Dir(currentDir), newBaseName)
    
    // Check if target already exists
    if _, err := os.Stat(newDir); err == nil {
        return fmt.Errorf("target directory already exists: %s", newDir)
    }
    
    // Perform atomic rename
    if err := os.Rename(currentDir, newDir); err != nil {
        return fmt.Errorf("failed to hide attempt: %w", err)
    }
    
    // Update internal file path
    a.file = filepath.Join(newDir, filepath.Base(a.file))
    
    // Log the operation
    logger.Info(ctx, "Hidden attempt",
        "oldPath", currentDir,
        "newPath", newDir,
        "attemptID", a.id,
    )
    
    return nil
}
```

### Phase 2: Command Modification (Priority: High)

#### 2.1 Modify Dequeue Command
- **File:** `/internal/cmd/dequeue.go`
- **Function:** `dequeueDAGRun`
- **Changes:**
  - Remove the status change to Cancel (lines 73-84)
  - Replace with Hide() call
  - Remove the Open/Close operations (no longer needed)
  - Update error messages

**Current Implementation (lines 73-84):**
```go
// Make the status as canceled
dagStatus.Status = status.Cancel

if err := attempt.Open(ctx.Context); err != nil {
    return fmt.Errorf("failed to open run: %w", err)
}
defer func() {
    _ = attempt.Close(ctx.Context)
}()
if err := attempt.Write(ctx.Context, *dagStatus); err != nil {
    return fmt.Errorf("failed to save status: %w", err)
}
```

**New Implementation:**
```go
// Hide the queued attempt instead of canceling it
if err := attempt.Hide(ctx.Context); err != nil {
    return fmt.Errorf("failed to hide queued attempt: %w", err)
}
```

**Complete Updated Function:**
```go
func dequeueDAGRun(ctx *Context, dagRun digraph.DAGRunRef) error {
    // Check if queues are enabled
    if !ctx.Config.Queues.Enabled {
        return fmt.Errorf("queues are disabled in configuration")
    }
    
    attempt, err := ctx.DAGRunStore.FindAttempt(ctx, dagRun)
    if err != nil {
        return fmt.Errorf("failed to find the record for dag-run ID %s: %w", dagRun.ID, err)
    }

    dagStatus, err := attempt.ReadStatus(ctx)
    if err != nil {
        return fmt.Errorf("failed to read status: %w", err)
    }

    if dagStatus.Status != status.Queued {
        // If the status is not queued, return an error
        return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, dagStatus.Status)
    }

    dag, err := attempt.ReadDAG(ctx)
    if err != nil {
        return fmt.Errorf("failed to read dag: %w", err)
    }

    // Make sure the dag-run is not running at least locally
    latestStatus, err := ctx.DAGRunMgr.GetCurrentStatus(ctx, dag, dagRun.ID)
    if err != nil {
        return fmt.Errorf("failed to get latest status: %w", err)
    }
    if latestStatus.Status != status.Queued {
        return fmt.Errorf("dag-run %s is not in queued status but %s", dagRun.ID, latestStatus.Status)
    }

    // Hide the queued attempt instead of canceling it
    if err := attempt.Hide(ctx.Context); err != nil {
        return fmt.Errorf("failed to hide queued attempt: %w", err)
    }

    // Dequeue the dag-run from the queue
    if _, err = ctx.QueueStore.DequeueByDAGRunID(ctx.Context, dagRun.Name, dagRun.ID); err != nil {
        return fmt.Errorf("failed to dequeue dag-run %s: %w", dagRun.ID, err)
    }

    logger.Info(ctx.Context, "Dequeued dag-run",
        "dag", dagRun.Name,
        "runId", dagRun.ID,
    )

    return nil
}
```

#### 2.2 Update Queue Processing
- Ensure queue processor skips hidden attempts
- Verify queue listing doesn't include hidden attempts

### Phase 3: Filtering and Display Logic (Priority: Medium)

#### 3.1 Update Directory Listing Functions
- **File:** `/internal/persistence/filedagrun/dagrun.go`
- **Functions to modify:**
  - `listDirsSorted` - Add logic to skip hidden directories
  - `ListAttempts` - No changes needed (relies on listDirsSorted)
  - `LatestAttempt` - No changes needed (relies on listDirsSorted)

**Current `listDirsSorted` Implementation:**
```go
func listDirsSorted(dir string, reverse bool, re *regexp.Regexp) ([]string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    var dirs []string
    for _, e := range entries {
        if e.IsDir() && re.MatchString(e.Name()) {
            dirs = append(dirs, e.Name())
        }
    }
    // Sort logic...
}
```

**Updated `listDirsSorted` Implementation:**
```go
func listDirsSorted(dir string, reverse bool, re *regexp.Regexp) ([]string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    var dirs []string
    for _, e := range entries {
        name := e.Name()
        // Skip hidden directories (starting with .)
        if strings.HasPrefix(name, ".") {
            continue
        }
        if e.IsDir() && re.MatchString(name) {
            dirs = append(dirs, name)
        }
    }
    
    if reverse {
        sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
    } else {
        sort.Strings(dirs)
    }
    return dirs, nil
}
```

**Clean Approach - Options Struct:**
```go
// ListDirsOptions configures directory listing behavior
type ListDirsOptions struct {
    Reverse       bool
    IncludeHidden bool
}

// Keep existing function for backward compatibility
func listDirsSorted(dir string, reverse bool, re *regexp.Regexp) ([]string, error) {
    return listDirsWithOptions(dir, &ListDirsOptions{Reverse: reverse}, re)
}

// New function with options
func listDirsWithOptions(dir string, opts *ListDirsOptions, re *regexp.Regexp) ([]string, error) {
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    
    var dirs []string
    for _, e := range entries {
        name := e.Name()
        // Skip hidden directories unless included
        if !opts.IncludeHidden && strings.HasPrefix(name, ".") {
            continue
        }
        if e.IsDir() && re.MatchString(name) {
            dirs = append(dirs, name)
        }
    }
    
    if opts.Reverse {
        sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
    } else {
        sort.Strings(dirs)
    }
    return dirs, nil
}
```

**Update removeLogFiles to include hidden attempts:**
```go
func (dr DAGRun) removeLogFiles(ctx context.Context) error {
    // Get ALL attempts including hidden ones for log cleanup
    attDirs, err := listDirsWithOptions(dr.baseDir, &ListDirsOptions{
        Reverse:       true,
        IncludeHidden: true,
    }, reAttemptDir)
    if err != nil {
        return fmt.Errorf("failed to list attempt directories: %w", err)
    }
    
    var deleteFiles []string
    for _, attDir := range attDirs {
        attempt, err := NewAttempt(filepath.Join(dr.baseDir, attDir, JSONLStatusFile), nil)
        if err != nil {
            logger.Error(ctx, "failed to read attempt data", "err", err)
            continue
        }
        if !attempt.Exists() {
            continue
        }
        status, err := attempt.ReadStatus(ctx)
        if err != nil {
            logger.Error(ctx, "failed to read status", "err", err)
            continue
        }
        deleteFiles = append(deleteFiles, status.Log)
        for _, n := range status.Nodes {
            deleteFiles = append(deleteFiles, n.Stdout, n.Stderr)
        }
    }
    
    // Also handle child DAG runs...
    // (rest of the existing logic)
}
```

#### 3.2 Verify Behavior
- **Key Points:**
  - Since both `ListAttempts` and `LatestAttempt` use `listDirsSorted`, updating this single function will fix both
  - Hidden attempts (starting with `.`) will be automatically excluded
  - The sorting logic (newest first with `reverse=true`) remains unchanged
  - This ensures `LatestAttempt` returns the most recent non-hidden attempt

**Expected Behavior After Implementation:**
1. When a DAG has attempts: `[attempt_1 (Success), attempt_2 (Queued)]`
2. After dequeuing: `[attempt_1 (Success), .attempt_2 (hidden)]`
3. `LatestAttempt()` returns: `attempt_1` (showing Success status)
4. `ListAttempts()` returns: `[attempt_1]` only

### Phase 4: Testing (Priority: Medium)

#### 4.1 Unit Tests

**Test File: `/internal/persistence/filedagrun/attempt_test.go`**

```go
func TestAttempt_Hide(t *testing.T) {
    tests := []struct {
        name    string
        setup   func(t *testing.T) (*Attempt, string)
        wantErr bool
        verify  func(t *testing.T, att *Attempt, oldPath string)
    }{
        {
            name: "successfully hide normal attempt",
            setup: func(t *testing.T) (*Attempt, string) {
                dir := t.TempDir()
                attemptDir := filepath.Join(dir, "attempt_20240122_100000_000Z_abc123")
                os.MkdirAll(attemptDir, 0755)
                statusFile := filepath.Join(attemptDir, JSONLStatusFile)
                att, _ := NewAttempt(statusFile, nil)
                return att, attemptDir
            },
            verify: func(t *testing.T, att *Attempt, oldPath string) {
                newPath := att.Path()
                assert.True(t, strings.HasPrefix(filepath.Base(newPath), "."))
                assert.NoFileExists(t, oldPath)
                assert.DirExists(t, newPath)
            },
        },
        {
            name: "idempotent - already hidden",
            setup: func(t *testing.T) (*Attempt, string) {
                dir := t.TempDir()
                attemptDir := filepath.Join(dir, ".attempt_20240122_100000_000Z_abc123")
                os.MkdirAll(attemptDir, 0755)
                statusFile := filepath.Join(attemptDir, JSONLStatusFile)
                att, _ := NewAttempt(statusFile, nil)
                return att, attemptDir
            },
            wantErr: false,
        },
        {
            name: "error when attempt is open",
            setup: func(t *testing.T) (*Attempt, string) {
                // Create attempt and open it
                // Test should fail
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            att, oldPath := tt.setup(t)
            err := att.Hide(context.Background())
            
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
                if tt.verify != nil {
                    tt.verify(t, att, oldPath)
                }
            }
        })
    }
}
```

**Test File: `/internal/cmd/dequeue_test.go`**

```go
func TestDequeueCommand_HidesAttempt(t *testing.T) {
    // Setup test environment
    ctx := setupTestContext(t)
    
    // Create a DAG with successful run
    dag := testutil.CreateTestDAG(t, "test-dag")
    
    // Create first attempt (successful)
    run1 := dag.CreateRun(t, models.TimeInUTC{Time: time.Now().Add(-1 * time.Hour)})
    run1.SetStatus(t, status.Success)
    
    // Create second attempt (queued)
    run2 := dag.CreateRun(t, models.TimeInUTC{Time: time.Now()})
    run2.SetStatus(t, status.Queued)
    
    // Enqueue the second run
    ctx.QueueStore.Enqueue(ctx.Context, dag.Name, run2.ID)
    
    // Execute dequeue command
    err := dequeueDAGRun(ctx, digraph.DAGRunRef{
        Name: dag.Name,
        ID:   run2.ID,
    })
    assert.NoError(t, err)
    
    // Verify the queued attempt is hidden
    attempts, err := dag.ListAttempts(ctx.Context)
    assert.NoError(t, err)
    assert.Len(t, attempts, 1) // Only the successful attempt should be visible
    
    // Verify latest attempt shows success
    latest, err := dag.LatestAttempt(ctx.Context, nil)
    assert.NoError(t, err)
    
    status, err := latest.ReadStatus(ctx.Context)
    assert.NoError(t, err)
    assert.Equal(t, status.Status, status.Success)
    
    // Verify the hidden directory exists
    hiddenPath := filepath.Join(dag.Dir, ".attempt_*")
    matches, _ := filepath.Glob(hiddenPath)
    assert.Len(t, matches, 1)
}
```

**Test File: `/internal/persistence/filedagrun/dagrun_test.go`**

```go
func TestListDirsSorted_ExcludesHidden(t *testing.T) {
    dir := t.TempDir()
    
    // Create test directories
    dirs := []string{
        "attempt_20240122_100000_000Z_aaa",
        "attempt_20240122_110000_000Z_bbb",
        ".attempt_20240122_120000_000Z_ccc",
        "attempt_20240122_130000_000Z_ddd",
        ".hidden_other_dir",
    }
    
    for _, d := range dirs {
        os.MkdirAll(filepath.Join(dir, d), 0755)
    }
    
    // Test with reverse=true (newest first)
    result, err := listDirsSorted(dir, true, reAttemptDir)
    assert.NoError(t, err)
    assert.Equal(t, []string{
        "attempt_20240122_130000_000Z_ddd",
        "attempt_20240122_110000_000Z_bbb", 
        "attempt_20240122_100000_000Z_aaa",
    }, result)
    
    // Verify hidden directories are excluded
    assert.NotContains(t, result, ".attempt_20240122_120000_000Z_ccc")
    assert.NotContains(t, result, ".hidden_other_dir")
}

func TestLatestAttempt_SkipsHidden(t *testing.T) {
    // Create DAG run with multiple attempts
    dagRun := setupTestDAGRun(t)
    
    // Create attempts
    att1 := dagRun.CreateAttempt(t, time.Now().Add(-2*time.Hour))
    att1.SetStatus(t, status.Success)
    
    att2 := dagRun.CreateAttempt(t, time.Now().Add(-1*time.Hour))
    att2.SetStatus(t, status.Failed)
    
    att3 := dagRun.CreateAttempt(t, time.Now())
    att3.SetStatus(t, status.Queued)
    
    // Hide the latest attempt
    err := att3.Hide(context.Background())
    assert.NoError(t, err)
    
    // Get latest attempt
    latest, err := dagRun.LatestAttempt(context.Background(), nil)
    assert.NoError(t, err)
    
    // Should return the failed attempt, not the hidden queued one
    status, err := latest.ReadStatus(context.Background())
    assert.NoError(t, err)
    assert.Equal(t, status.Status, status.Failed)
}
```

#### 4.2 Integration Tests
- **Scenarios:**
  1. DAG with successful run → queue new run → dequeue → verify shows "Success"
  2. DAG with failed run → queue new run → dequeue → verify shows "Failed"
  3. Multiple queued attempts → dequeue all → verify correct state
  4. Dequeue non-existent run → verify error handling

#### 4.3 Edge Cases
- Dequeue while DAG is running
- Dequeue already dequeued attempt
- File system permissions issues
- Disk space constraints
- Concurrent dequeue operations

### Phase 5: Recovery and Rollback (Priority: Low)

#### 5.1 Restore Functionality (Optional)
- Implement `Restore()` method for recovery scenarios
- Add admin command to restore hidden attempts

```go
// Restore reverses the Hide operation, making the attempt visible again
func (a *Attempt) Restore(ctx context.Context) error {
    a.mu.Lock()
    defer a.mu.Unlock()
    
    currentDir := a.Path()
    baseName := filepath.Base(currentDir)
    
    // Check if it's actually hidden
    if !strings.HasPrefix(baseName, ".") {
        return fmt.Errorf("attempt is not hidden")
    }
    
    // Remove the dot prefix to restore visibility
    newBaseName := strings.TrimPrefix(baseName, ".")
    newDir := filepath.Join(filepath.Dir(currentDir), newBaseName)
    
    // Check if target already exists
    if _, err := os.Stat(newDir); err == nil {
        return fmt.Errorf("cannot restore: target directory already exists: %s", newDir)
    }
    
    // Perform atomic rename
    if err := os.Rename(currentDir, newDir); err != nil {
        return fmt.Errorf("failed to restore attempt: %w", err)
    }
    
    // Update internal file path
    a.file = filepath.Join(newDir, filepath.Base(a.file))
    
    logger.Info(ctx, "Restored attempt",
        "oldPath", currentDir,
        "newPath", newDir,
        "attemptID", a.id,
    )
    
    return nil
}
```

#### 5.2 Cleanup Utilities
- Tool to list all hidden attempts
- Option to permanently delete old hidden attempts
- Configurable retention policy

### Phase 6: Documentation (Priority: Low)

#### 6.1 Code Documentation
- Add comprehensive comments to new methods
- Document the hiding mechanism
- Explain the directory naming convention

#### 6.2 User Documentation
- Update dequeue command documentation
- Explain new behavior in user guide
- Add troubleshooting section

#### 6.3 Migration Guide
- Instructions for users with existing dequeued runs
- Explanation of behavior change

## Risk Assessment

### Potential Risks
1. **File System Operations:** Rename operations might fail due to permissions or disk issues
2. **Backwards Compatibility:** Existing dequeued runs will still show as "Canceled"
3. **Performance:** Directory listing might be slower with hidden files
4. **Concurrent Access:** Multiple processes accessing the same attempt
5. **Log File Cleanup:** Hidden attempts' log files must be properly cleaned up

### Mitigation Strategies
1. **Atomic Operations:** Use atomic rename to prevent partial states
2. **Graceful Degradation:** Fall back to cancel if hide fails
3. **Caching:** Cache attempt listings to minimize file system calls
4. **File Locking:** Implement appropriate locking mechanisms
5. **Log Cleanup:** Ensure `removeLogFiles` includes hidden attempts to prevent resource leaks

## Rollout Plan

### Phase 1: Development (Week 1-2)
- Implement core Hide() functionality
- Modify dequeue command
- Basic unit tests

### Phase 2: Testing (Week 3)
- Comprehensive testing
- Edge case validation
- Performance testing

### Phase 3: Documentation (Week 4)
- Code documentation
- User guides
- Release notes

### Phase 4: Release
- Include in next minor version
- Monitor for issues
- Gather user feedback

## Success Criteria

1. Dequeued DAG runs show previous state instead of "Canceled"
2. Hidden attempts are preserved for audit purposes
3. No performance degradation
4. All tests pass
5. Documentation is complete

## Alternative Approaches Considered

1. **Database Flag:** Add a "hidden" flag to database
   - Rejected: Requires schema changes

2. **Separate Hidden Directory:** Move hidden attempts to different location
   - Rejected: Complicates file management

3. **Metadata File:** Store visibility in separate metadata
   - Rejected: Adds complexity and potential sync issues

## Summary of Required File Changes

### 1. **Interface Update**
- **File:** `/internal/models/dagrun.go`
- **Change:** Add `Hide(ctx context.Context) error` to `DAGRunAttempt` interface

### 2. **Core Implementation**
- **File:** `/internal/persistence/filedagrun/attempt.go`
- **Changes:**
  - Add `Path() string` method
  - Add `Hide(ctx context.Context) error` method
  - Optional: Add `Restore(ctx context.Context) error` method

### 3. **Constants and Helper Functions**
- **File:** `/internal/persistence/filedagrun/dagrun.go`
- **Changes:**
  - No new constants needed (just use dot prefix)
  - Update `listDirsSorted` to skip directories starting with "." by default
  - Add optional parameter to include hidden directories when needed
  - Modify `removeLogFiles` to include hidden attempts for proper cleanup

### 4. **Dequeue Command**
- **File:** `/internal/cmd/dequeue.go`
- **Changes:**
  - Replace status change logic (lines 73-84) with `attempt.Hide(ctx.Context)`
  - Remove Open/Close operations

### 5. **Tests**
- **New File:** `/internal/persistence/filedagrun/attempt_test.go` - Add `TestAttempt_Hide`
- **Update:** `/internal/cmd/dequeue_test.go` - Add `TestDequeueCommand_HidesAttempt`
- **Update:** `/internal/persistence/filedagrun/dagrun_test.go` - Add filtering tests

## Implementation Checklist

- [ ] Add Hide() method to DAGRunAttempt interface
- [ ] Implement Hide() method in Attempt struct
- [ ] No new constants needed
- [ ] Update listDirsSorted to filter hidden directories
- [ ] Modify dequeue command to use Hide()
- [ ] Write unit tests for Hide() method
- [ ] Write integration tests for dequeue behavior
- [ ] Test edge cases (permissions, concurrent access)
- [ ] Optional: Implement Restore() method
- [ ] Update documentation

## Conclusion

This implementation plan provides a comprehensive approach to preserving DAG state when dequeuing. The solution is simple, maintains backward compatibility, and preserves all historical data while improving the user experience.

### Key Benefits Recap:
1. **Preserves State**: Previous DAG run status remains visible after dequeue
2. **Maintains History**: All attempt data is preserved for audit purposes
3. **Simple Implementation**: Uses file system operations, no schema changes
4. **Reversible**: Hidden attempts can be restored if needed
5. **Minimal Risk**: Changes are isolated and well-tested

The implementation focuses on reliability and simplicity, using Dagu's existing file-based architecture to deliver a better user experience without adding complexity.