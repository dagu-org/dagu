# Dequeue State Preservation Analysis

## Current Behavior

### Problem Statement
When a DAG is enqueued, it creates a new DAG run attempt with status `Queued`. This becomes the latest attempt shown in the UI. When dequeued, the current implementation changes this queued attempt's status to `Cancel`, making it remain as the latest attempt. Users see "Canceled" in the DAG list instead of the previous status (e.g., Success) from before enqueueing.

### Current Implementation Flow

#### Enqueue Process (`/internal/cmd/enqueue.go`)
1. Creates a **new DAG run attempt** (line 76)
2. Sets status to `Queued` (line 94)
3. Adds to queue store
4. This new attempt becomes the latest for the DAG

#### Dequeue Process (`/internal/cmd/dequeue.go`)
1. Finds the queued attempt (line 44)
2. Verifies status is `Queued`
3. **Sets status to `Cancel`** (line 74)
4. Removes from queue store
5. The canceled attempt remains as the latest

## Code Analysis

### Key Components

1. **Attempt Storage Structure**:
   - Attempts stored in directories: `attempt_YYYYMMDD_HHMMSS_MSSZ_<attemptID>`
   - Each attempt has its own directory with status file
   - DAG runs can have multiple attempts

2. **DAGRunAttempt Interface** (`/internal/models/dagrun.go`):
   - No `Delete()` or `Remove()` method
   - Only provides: Open, Write, Close, ReadStatus, ReadDAG

3. **DAGRunStore Interface**:
   - No method to delete attempts
   - `RemoveOldDAGRuns` exists but only for retention cleanup

## The Real Issue

The problem is architectural:
1. When enqueueing creates a new attempt, this becomes the "latest" attempt
2. When dequeuing marks it as canceled, it remains the latest
3. The UI shows the latest attempt's status, showing "Canceled"
4. Users expect to see the previous status (before enqueueing)

## Solution Requirements

When dequeuing a DAG run:
1. The queued attempt should be removed entirely
2. The previous attempt should become the latest again
3. UI should show the status from before enqueueing

## Design Challenges

### Challenge 1: No Delete Method
The current `DAGRunAttempt` interface has no delete capability. We need to:
- Add a `Delete()` method to the interface
- Implement deletion in the file-based storage

### Challenge 2: Finding Previous Attempt
After deleting the queued attempt, we need the previous attempt to be recognized as the latest.

### Challenge 3: Data Consistency
Must ensure atomic operations to avoid partial states.

## Recommended Approach

### Option 1: Mark as "Hidden" (Recommended - Preserves History)
1. Add a new status `Dequeued` to represent hidden/canceled queue items
2. Modify `LatestAttempt()` and other queries to skip `Dequeued` status
3. Update dequeue to set status to `Dequeued` instead of `Cancel`
4. Preserves full audit trail while hiding from UI

### Option 2: Add Delete Capability (Alternative - Destructive)
1. Add `Delete()` method to `DAGRunAttempt` interface
2. Implement file deletion in `filedagrun.Attempt`
3. Modify dequeue to delete the attempt instead of updating status
4. Let the store naturally find the previous attempt as latest

### Option 3: Avoid Creating Attempt (Complex)
1. Store queued items differently (not as attempts)
2. Only create attempt when actually running
3. Requires significant refactoring

## Implementation Approaches

### Approach 1: Rename Attempt Directory (Recommended - Preserves History)

After deep analysis of the filedagrun package, I found that:
- Attempts are stored as directories: `attempt_YYYYMMDD_HHMMSS_MSSZ_<attemptID>/`
- `LatestAttempt()` uses `listDirsSorted()` with `reAttemptDir` regex pattern
- The regex only matches directories starting with `attempt_`
- We can effectively "hide" attempts by renaming them

**Implementation**:
1. **Add Hide Method to Attempt**:
   ```go
   // In attempt.go
   func (att *Attempt) Hide(ctx context.Context) error {
       att.mu.Lock()
       defer att.mu.Unlock()
       
       if att.writer != nil {
           return fmt.Errorf("cannot hide: attempt is open for writing")
       }
       
       // Remove from cache if present
       if att.cache != nil {
           att.cache.Remove(att.file)
       }
       
       // Rename the attempt directory to hide it
       attemptDir := filepath.Dir(att.file)
       hiddenDir := strings.Replace(attemptDir, "attempt_", ".dequeued_attempt_", 1)
       
       if err := os.Rename(attemptDir, hiddenDir); err != nil {
           return fmt.Errorf("failed to hide attempt directory: %w", err)
       }
       
       return nil
   }
   ```

2. **Update Dequeue Logic**:
   ```go
   // In dequeue.go, replace lines 73-84 with:
   if err := attempt.Hide(ctx); err != nil {
       return fmt.Errorf("failed to hide queued attempt: %w", err)
   }
   ```

**Advantages**:
- Preserves full history for auditing
- No interface changes needed (Hide is internal)
- Simple file system operation (rename)
- Automatically filtered by existing regex
- Can be reversed if needed
- No risk of data loss

### Approach 2: Add Hidden Flag File (Alternative)

Instead of renaming, add a marker file:
1. Create `.hidden` file in attempt directory
2. Modify `LatestAttempt()` to check for this file
3. Skip attempts with `.hidden` marker

**Implementation sketch**:
```go
// In LatestAttempt()
for _, attDir := range attDirs {
    hiddenPath := filepath.Join(dr.baseDir, attDir, ".hidden")
    if _, err := os.Stat(hiddenPath); err == nil {
        continue // Skip hidden attempts
    }
    // ... existing logic
}
```

### Approach 3: Delete Method (Not Recommended)

Add Delete method to interface and remove attempt directory completely.
- Pros: Clean, simple
- Cons: Destructive, loses audit trail

## Recommended Implementation Plan

1. **Use Approach 1 (Rename)**:
   - Minimal code changes
   - Preserves history
   - Works with existing regex filtering
   - No interface changes

2. **Testing Plan**:
   - Test rename operation
   - Verify hidden attempts are skipped
   - Test concurrent access
   - Ensure UI shows previous status
   - Add test case to verify dequeue behavior

3. **Performance Considerations**:
   - Rename is atomic on same filesystem
   - No performance impact on queries
   - Hidden attempts automatically filtered by regex

4. **Future Enhancement**:
   - Could add cleanup job to remove old hidden attempts
   - Could add UI feature to show hidden/dequeued attempts

## Directory Structure Example

Before dequeue:
```
dag-runs/2024/01/15/dag-run_20240115_100000Z_abc123/
├── attempt_20240115_100000_000Z_att1/
│   └── status.jsonl (Success)
└── attempt_20240115_110000_000Z_att2/
    └── status.jsonl (Queued)
```

After dequeue:
```
dag-runs/2024/01/15/dag-run_20240115_100000Z_abc123/
├── attempt_20240115_100000_000Z_att1/
│   └── status.jsonl (Success)
└── .dequeued_attempt_20240115_110000_000Z_att2/
    └── status.jsonl (Queued)
```

`LatestAttempt()` will now return att1 (Success) since att2 is hidden.