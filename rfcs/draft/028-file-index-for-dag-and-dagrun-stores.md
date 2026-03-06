# RFC: File Index for DAG and DAG Run Stores

## Goal

Introduce lightweight, disposable index files for DAG definitions and stable DAG-run history so list and search-style operations avoid repeatedly opening and parsing large numbers of YAML and `status.jsonl` files.

The filesystem remains the source of truth. Indexes are caches. If an index is missing, stale, corrupt, or version-mismatched, the system rebuilds it or falls back to the filesystem scan used today.

This RFC targets a design that is:

- Simple: no new coordination system, no database, no background watcher.
- Robust: no silent stale-data path for stable history; explicit eventual-consistency semantics on shared volumes.
- Efficient: read paths avoid repeated latest-attempt discovery and repeated `status.jsonl` parsing for stable historical days.

## Scope

| In Scope | Out of Scope |
|----------|-------------|
| Persistent index for DAG definition store | Full-text search / `Grep` |
| Persistent per-day-per-DAG index for stable DAG-run history | Database-backed stores |
| Lock-free index rebuild/write path | Replacing existing DAG-run write-side locks |
| Shared-volume compatibility with eventual convergence | Linearizable cross-node read-after-write consistency |
| Protobuf serialization for compact index files | Sub-DAG run indexing |

## Problem

### DAG Definitions

`DAGStore.List()` and `TagList()` currently scan the DAG directory and repeatedly load YAML metadata. On large workspaces, list operations pay the same parsing cost again and again even when nothing changed.

### DAG Runs

`ListStatuses`, `RecentAttempts`, and `LatestAttempt` currently walk day directories, discover runs, discover the latest attempt under each run, then open and parse `status.jsonl`.

That is expensive for historical queries because most old runs are already terminal and unchanged, but the read path still has to rediscover that fact from the filesystem every time.

## Design Principles

1. The index is never authoritative.
2. The read path must validate the exact source-of-truth state that can invalidate the cached summary.
3. Mutable history is not indexed. Stable history is indexed.
4. No new locks are introduced for index maintenance.
5. Shared-volume semantics are eventual, not linearizable.

---

## Part 1: DAG Definition Index

### Index Location

```
{dagDir}/.dag.index
```

### Source Write Requirement

The DAG definition index is only safe if Dagu's own DAG writes are atomic. This RFC therefore requires DAG definition create/update operations to write YAML via atomic temp-file + rename rather than plain `os.WriteFile`.

This requirement applies to Dagu-managed writes. External editors may still write non-atomically; in that case the index remains recoverable, but readers may temporarily observe partial or invalid files exactly as they can today.

### Index File Format

Protobuf-serialized binary file.

```protobuf
syntax = "proto3";
package dagu.index.v1;

message DAGIndex {
  uint32 version = 1;
  int64 built_at_unix = 2;
  repeated DAGIndexEntry entries = 3;
}

message DAGIndexEntry {
  string file_path = 1;
  int64 file_size = 2;
  int64 mod_time = 3;
  string name = 4;
  repeated string tags = 5;
  string group = 6;
  string schedule = 7;
  bool suspended = 8;
  string description = 9;
  string load_error = 10;
}
```

### Why `load_error` Is Stored

`DAGStore.List()` currently preserves DAGs with build errors instead of hiding them. The index must preserve that behavior.

If a DAG can be loaded with `WithAllowBuildErrors()`, its metadata is stored together with any resulting error string. If metadata cannot be produced at all, rebuild records a `load_error` entry for the file so list behavior still surfaces the failure without reopening the YAML on every read.

`TagList()` ignores entries with `load_error != ""`.

### Validation Model

On every read:

1. Read and parse `.dag.index`.
2. If missing, corrupt, or version-mismatched: rebuild.
3. `ReadDir(dagDir)` and build the set of root-level YAML files.
4. Compare file set and count to index entries. Mismatch: rebuild.
5. For each YAML file, `Stat` and compare `file_size` + `mod_time`. Mismatch: rebuild.
6. `ReadDir(flagsBaseDir)` and compare suspend flags to indexed `suspended` state. Mismatch: rebuild.
7. If all checks match, serve list/tag operations from the index.

Validation is metadata-only. It does not reopen YAML files on the common path.

### Rebuild Model

Rebuild scans the DAG directory and loads each file using the same metadata-loading semantics required to preserve current `DAGStore.List()` behavior:

- metadata-only load
- without evaluation
- without schema validation
- allow build errors

The rebuild path must not reuse any cache whose staleness check is weaker than the index's own stat validation. In particular, a cache that compares only second-granularity mtimes is not valid for index rebuild unless it is upgraded to nanosecond granularity.

### Notes on Tag Semantics

The index stores raw tags. `TagList()` continues to derive:

- the full tag string representation
- the key-only form for `key=value` tags

This preserves current filtering behavior.

---

## Part 2: DAG Run Index

### Key Change From the Original Draft

The DAG-run index is **not** a cache for "past days" in general. It is a cache for **stable historical days** only.

A day is stable only when every run in that day has a terminal latest attempt.

If any run's latest attempt is still active, that day is served directly from the filesystem and no `.dagrun.index` is written.

This matters because old run directories are still mutable:

- retries create new `attempt_*` directories under an existing `dag-run_*`
- the latest attempt can change
- the latest attempt's `status.jsonl` continues to change until the run is terminal

### Index Location

```
{dagPrefix}/dag-runs/{year}/{month}/{day}/.dagrun.index
```

One index per DAG per day, created only when:

- the day has 10 or more runs, and
- the day is stable

Days with fewer than 10 runs are read directly from the filesystem because the index overhead is not justified.

### Index File Format

Protobuf-serialized binary file.

```protobuf
message DAGRunIndex {
  uint32 version = 1;
  int64 built_at_unix = 2;
  repeated DAGRunIndexEntry entries = 3;
}

message DAGRunIndexEntry {
  string dag_run_dir = 1;
  string dag_run_id = 2;
  string latest_attempt_dir = 3;
  int64 latest_status_size = 4;
  int64 latest_status_mod_time = 5;
  int32 status = 6;
  int64 started_at = 7;
  int64 finished_at = 8;
  repeated string tags = 9;
}
```

### Why `latest_attempt_dir` Is Required

Directory existence alone is not enough to validate a run summary. A retry can add a newer attempt under the same run directory without changing the run-directory count at the day level.

The index must therefore validate that the indexed latest attempt is still the filesystem's latest attempt.

### Validation Model

When a read operation reaches a day directory:

1. `ReadDir(dayDir)` and build the set of visible `dag-run_*` directories.
2. If there are fewer than 10 runs: read directly from the filesystem. No index is used or written.
3. Otherwise, try to read `.dagrun.index`.
4. If the index is missing, corrupt, or version-mismatched: rebuild.
5. If present, validate:
   - compare indexed run set/count to `ReadDir(dayDir)`
   - for each indexed run, `ReadDir(runDir)` and determine the newest visible `attempt_*`
   - compare that directory name to `latest_attempt_dir`
   - `Stat(runDir/latest_attempt_dir/status.jsonl)` and compare `latest_status_size` + `latest_status_mod_time`
6. If any check fails: rebuild.
7. If rebuild finds any active latest attempt in that day: do not write an index; serve that day from the filesystem.
8. If all latest attempts are terminal: write the day index and use it.

This is the required validation model. The index must validate latest-attempt identity and latest-status-file metadata, not only directory existence.

### Rebuild Model

Rebuild scans all `dag-run_*` directories in the day, then for each run:

1. discover the newest visible `attempt_*`
2. read the latest attempt's `status.jsonl`
3. if the latest status is active, mark the day non-indexable and stop writing an index for that day
4. otherwise, store summary fields plus validation fields in the index entry

If rebuild fails to write the index, the operation falls back to direct filesystem results.

### What the Index Accelerates

For stable historical days, the index removes:

- repeated latest-attempt discovery for every request
- repeated `status.jsonl` open/parse for every run

It does **not** remove metadata validation. The common path remains:

- `ReadDir(dayDir)`
- `ReadDir(runDir)` for each indexed run
- one `Stat` of the latest attempt's `status.jsonl`

That is still much cheaper than parsing `status.jsonl` for every run.

### Filter-Then-Fetch

The day index stores only summary fields required for:

- filtering by tags/status
- sorting by run time
- pagination candidate selection

For full `DAGRunStatus` responses, `status.jsonl` is opened only for entries on the requested page.

If a candidate disappears or becomes unreadable between index validation and page fetch:

- log WARN
- skip that candidate
- continue fetching later candidates until the page is filled or candidates are exhausted

The system must not return a short page solely because stale indexed candidates were skipped while more candidates were still available.

### `LatestAttempt` and `RecentAttempts`

`LatestAttempt` and `RecentAttempts` walk days in reverse chronological order:

- if a stable-day index exists and validates, use it
- otherwise, read that day directly from the filesystem

This keeps the "hot" mutable path correct while accelerating stable historical days.

### Sub-DAG Runs

Sub-DAG runs remain out of scope.

They are discovered from the parent run under `children/` exactly as today and are not indexed by day.

---

## Shared Design Decisions

### Concurrency Model

Index maintenance is lock-free:

- readers may rebuild the same index concurrently
- last atomic rename wins
- no index-specific lock can block another reader

This RFC does **not** remove or replace existing DAG-run write-side locking. Existing store locks for run creation/removal remain unchanged.

### Shared-Volume Semantics

This design supports multiple servers using the same shared volume under an **eventual consistency** model.

It does **not** promise:

- linearizable cross-node reads
- immediate visibility of one node's write to every other node

It does promise:

- no silent trust of stale index data once filesystem metadata observed by the reader reflects the change
- automatic convergence via validation + rebuild
- safe fallback to direct filesystem reads if the index cannot be trusted

### Network Filesystem Notes

- Atomic rename within one directory is assumed.
- Metadata visibility may lag across nodes due to attribute or directory-entry caching.
- Coarse timestamp resolution can delay change detection.
- These behaviors are acceptable only because indexes are caches and the consistency target is eventual convergence.

The RFC must not describe this as "no NFS correctness concerns." The correct statement is that shared-volume correctness is eventual and bounded by the filesystem's metadata visibility model.

### Crash Safety

Atomic temp-file + rename prevents readers from seeing partially written index files.

This RFC uses "crash-safe" to mean:

- after a crash, the index may be missing, stale, or replaced by the previous complete version
- the next read can recover by rebuilding or falling back to the filesystem

This RFC does **not** claim durable commit semantics from rename alone. If the underlying atomic-write helper is later upgraded to fsync the temp file and parent directory, the implementation may document stronger durability.

### Temp File Cleanup

Orphaned `.tmp.*` files created during atomic writes may be cleaned up on store initialization using an age threshold.

Cleanup is a best-effort hygiene mechanism. It is not part of correctness.

### Retention / Data Cleanup

When the retention policy removes old DAG-run directories from a day, it must also delete `.dagrun.index` from that day directory. Otherwise the leftover index file prevents the empty day directory from being removed, leaving orphaned directories on disk.

The cleanup order is: delete all `dag-run_*` directories, then delete `.dagrun.index`, then remove the day directory.

### Error Handling

- Missing/corrupt/version-mismatched index: rebuild.
- Rebuild failure: fall back to direct filesystem scan, log WARN.
- Index write failure: serve direct filesystem results, log WARN.
- Missing `status.jsonl` during page fetch: skip, WARN, and backfill if more candidates exist.

---

## Tradeoffs

| Decision | Chosen | Rejected | Why |
|----------|--------|----------|-----|
| DAG definition index | Single `.dag.index` | Per-file sidecars | One small file is simpler and cheap to validate. |
| DAG definition rebuild | Preserve list semantics including build errors | Hiding broken DAGs | `List()` behavior must not regress. |
| DAG-run indexing scope | Stable days only | "All past days" | Old run directories remain mutable due to retries and active attempts crossing day boundaries. |
| DAG-run validation | Validate latest-attempt identity + latest status file metadata | Validate only day/run existence | Directory existence does not detect retries or status updates. |
| Mutation-side markers | Not in this RFC | `.latest_attempt` marker files | Read-only validation keeps the design simpler for v1. |
| Shared-volume guarantee | Eventual consistency | Immediate cross-node freshness | Shared filesystems do not reliably provide that guarantee. |
| Filter-then-fetch | Yes, with page backfill | Return short pages on stale entries | Prevents user-visible pagination gaps caused only by stale candidates. |

---

## Migration & Rollback

### Forward Compatibility

`.dag.index` and `.dagrun.index` are new cache artifacts. Older binaries ignore them.

### Rollback

Downgrading to a build that does not understand the indexes leaves harmless extra files on disk.

### Mixed-Version Deployments

During rolling upgrades, different binaries may rewrite indexes with different `version` values. This is correct but can cause repeated rebuilds until the rollout completes.

### Manual Cleanup

Deleting any index file is always safe. The next read rebuilds it if needed.

---

## Expected Performance

### DAG Definitions

For unchanged DAG directories, the common path is:

- one index read
- one `ReadDir(dagDir)`
- one `Stat` per YAML file
- one `ReadDir(flagsBaseDir)`

This avoids reopening/parsing every YAML file.

### DAG Runs

For stable historical days, the common path is:

- one index read
- one `ReadDir(dayDir)`
- one `ReadDir(runDir)` per run
- one `Stat(status.jsonl)` per run
- full `status.jsonl` read only for page results

This avoids repeated `status.jsonl` parsing for every run in the day and keeps the expensive work proportional to the requested page size rather than the full candidate set.

---

## Definition of Done

### DAG Definition Index

- [ ] `DAGStore.List()` serves from `.dag.index` without reopening YAML files on the common path.
- [ ] `DAGStore.TagList()` serves from `.dag.index` and preserves current tag semantics.
- [ ] Index format is protobuf with `version`, `built_at_unix`, and `entries`.
- [ ] Index entries preserve DAGs with build errors via stored metadata and/or `load_error`.
- [ ] DAG definition writes managed by Dagu are atomic.
- [ ] Validation compares root-level YAML file set, per-file `size + mtime`, and suspend flags.
- [ ] Rebuild uses the same metadata-loading behavior as current `List()`.
- [ ] Rebuild does not depend on a cache with weaker staleness checks than the index itself.

### DAG Run Index

- [ ] `.dagrun.index` is created only for stable days with 10 or more runs.
- [ ] Stable means every run's latest attempt is terminal.
- [ ] Validation compares day-level run set, per-run latest-attempt identity, and latest status file `size + mtime`.
- [ ] If a day contains any active latest attempt, the system serves that day from the filesystem and does not write an index.
- [ ] `ListStatuses`, `RecentAttempts`, and `LatestAttempt` use stable-day indexes when valid and fall back to the filesystem otherwise.
- [ ] Full `DAGRunStatus` reads are limited to page results.
- [ ] Missing/unreadable page candidates are skipped with WARN and backfilled if more candidates exist.

### Shared

- [ ] No new locks are introduced for index maintenance.
- [ ] Existing DAG-run write-side locking remains unchanged.
- [ ] Index failures never fail the read operation; they only disable the cache.
- [ ] Shared-volume behavior is documented as eventual consistency.
- [ ] Deleting any index file triggers safe rebuild on the next read.
- [ ] Retention cleanup deletes `.dagrun.index` before removing day directories.
- [ ] Structured logs exist for rebuild trigger, rebuild success, rebuild failure, and filesystem fallback.
- [ ] Benchmarks show a clear improvement for large DAG lists and wide historical DAG-run queries.
