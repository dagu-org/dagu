-- name: LockDAGRunKey :exec
SELECT pg_advisory_xact_lock(
    hashtext(sqlc.arg(lock_key)::text),
    hashtext('dagu-dag-run:' || sqlc.arg(lock_key)::text)
);

-- name: CreateAttempt :one
INSERT INTO dagu_dag_run_attempts (
    id,
    dag_name,
    dag_run_id,
    root_dag_name,
    root_dag_run_id,
    is_root,
    attempt_id,
    run_created_at,
    attempt_created_at,
    workspace,
    workspace_valid,
    dag_data,
    local_work_dir
) VALUES (
    sqlc.arg(id),
    sqlc.arg(dag_name),
    sqlc.arg(dag_run_id),
    sqlc.arg(root_dag_name),
    sqlc.arg(root_dag_run_id),
    sqlc.arg(is_root),
    sqlc.arg(attempt_id),
    sqlc.arg(run_created_at),
    sqlc.arg(attempt_created_at),
    sqlc.narg(workspace),
    sqlc.arg(workspace_valid),
    sqlc.narg(dag_data),
    sqlc.arg(local_work_dir)
)
RETURNING *;

-- name: FindAnyRootAttempt :one
SELECT *
FROM dagu_dag_run_attempts
WHERE is_root
  AND dag_name = sqlc.arg(dag_name)
  AND dag_run_id = sqlc.arg(dag_run_id)
ORDER BY run_created_at ASC, attempt_created_at ASC, id ASC
LIMIT 1;

-- name: LatestRootAttempt :one
SELECT *
FROM dagu_dag_run_attempts
WHERE is_root
  AND dag_name = sqlc.arg(dag_name)
  AND dag_run_id = sqlc.arg(dag_run_id)
  AND NOT hidden
  AND status_data IS NOT NULL
ORDER BY attempt_created_at DESC, id DESC
LIMIT 1;

-- name: LatestRootAttemptForUpdate :one
SELECT *
FROM dagu_dag_run_attempts
WHERE is_root
  AND dag_name = sqlc.arg(dag_name)
  AND dag_run_id = sqlc.arg(dag_run_id)
  AND NOT hidden
  AND status_data IS NOT NULL
ORDER BY attempt_created_at DESC, id DESC
LIMIT 1
FOR UPDATE;

-- name: LatestSubAttempt :one
SELECT *
FROM dagu_dag_run_attempts
WHERE NOT is_root
  AND root_dag_name = sqlc.arg(root_dag_name)
  AND root_dag_run_id = sqlc.arg(root_dag_run_id)
  AND dag_run_id = sqlc.arg(dag_run_id)
  AND NOT hidden
  AND status_data IS NOT NULL
ORDER BY attempt_created_at DESC, id DESC
LIMIT 1;

-- name: LatestAttemptByName :one
SELECT *
FROM dagu_dag_run_attempts
WHERE is_root
  AND dag_name = sqlc.arg(dag_name)
  AND NOT hidden
  AND status_data IS NOT NULL
  AND (NOT sqlc.arg(has_from)::boolean OR run_created_at >= sqlc.arg(from_at)::timestamptz)
ORDER BY run_created_at DESC, dag_run_id ASC, attempt_created_at DESC, id DESC
LIMIT 1;

-- name: RecentAttemptsByName :many
WITH latest AS (
    SELECT DISTINCT ON (dag_run_id) *
    FROM dagu_dag_run_attempts
    WHERE is_root
      AND dag_name = sqlc.arg(dag_name)
      AND NOT hidden
      AND status_data IS NOT NULL
    ORDER BY dag_run_id, attempt_created_at DESC, id DESC
)
SELECT *
FROM latest
ORDER BY run_created_at DESC, dag_run_id ASC
LIMIT sqlc.arg(item_limit)::integer;

-- name: GetAttempt :one
SELECT *
FROM dagu_dag_run_attempts
WHERE id = sqlc.arg(id);

-- name: UpdateAttemptDAG :exec
UPDATE dagu_dag_run_attempts
SET dag_data = sqlc.arg(dag_data),
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: UpdateAttemptStatus :exec
UPDATE dagu_dag_run_attempts
SET status_data = sqlc.arg(status_data),
    status = sqlc.arg(status),
    workspace = sqlc.narg(workspace),
    workspace_valid = sqlc.arg(workspace_valid),
    started_at = sqlc.narg(started_at),
    finished_at = sqlc.narg(finished_at),
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: UpdateAttemptOutputs :exec
UPDATE dagu_dag_run_attempts
SET outputs_data = sqlc.narg(outputs_data),
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: UpdateAttemptMessages :exec
UPDATE dagu_dag_run_attempts
SET messages_data = sqlc.arg(messages_data),
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: MergeAttemptStepMessages :exec
UPDATE dagu_dag_run_attempts
SET messages_data = jsonb_set(
        coalesce(messages_data, '{}'::jsonb),
        ARRAY[sqlc.arg(step_name)::text],
        sqlc.arg(messages)::jsonb,
        true
    ),
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: SetAttemptCancelRequested :exec
UPDATE dagu_dag_run_attempts
SET cancel_requested = true,
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: SetAttemptHidden :exec
UPDATE dagu_dag_run_attempts
SET hidden = true,
    updated_at = now()
WHERE id = sqlc.arg(id);

-- name: ListRootStatusRows :many
WITH latest AS (
    SELECT DISTINCT ON (dag_name, dag_run_id) *
    FROM dagu_dag_run_attempts
    WHERE is_root
      AND NOT hidden
      AND status_data IS NOT NULL
    ORDER BY dag_name, dag_run_id, attempt_created_at DESC, id DESC
)
SELECT *
FROM latest
WHERE (sqlc.arg(exact_name)::text = '' OR dag_name::text = sqlc.arg(exact_name)::text)
  AND (sqlc.arg(name_contains)::text = '' OR status_data ->> 'name' ILIKE '%' || sqlc.arg(name_contains)::text || '%')
  AND (sqlc.arg(dag_run_id_contains)::text = '' OR dag_run_id::text LIKE '%' || sqlc.arg(dag_run_id_contains)::text || '%')
  AND (NOT sqlc.arg(has_from)::boolean OR run_created_at >= sqlc.arg(from_at)::timestamptz)
  AND (NOT sqlc.arg(has_to)::boolean OR run_created_at <= sqlc.arg(to_at)::timestamptz)
  AND (cardinality(sqlc.arg(statuses)::integer[]) = 0 OR status = ANY(sqlc.arg(statuses)::integer[]))
  AND (
      NOT sqlc.arg(workspace_filter_enabled)::boolean
      OR (
          workspace_valid
          AND (
              (workspace IS NULL AND sqlc.arg(include_unlabelled)::boolean)
              OR workspace::text = ANY(sqlc.arg(workspaces)::text[])
          )
      )
  )
  AND (
      NOT sqlc.arg(cursor_set)::boolean
      OR run_created_at < sqlc.arg(cursor_timestamp)::timestamptz
      OR (
          run_created_at = sqlc.arg(cursor_timestamp)::timestamptz
          AND dag_name::text > sqlc.arg(cursor_name)::text
      )
      OR (
          run_created_at = sqlc.arg(cursor_timestamp)::timestamptz
          AND dag_name::text = sqlc.arg(cursor_name)::text
          AND dag_run_id::text > sqlc.arg(cursor_dag_run_id)::text
      )
  )
ORDER BY run_created_at DESC, dag_name ASC, dag_run_id ASC
LIMIT sqlc.arg(page_limit)::integer;

-- name: ListRemovableRunsByDays :many
WITH latest AS (
    SELECT DISTINCT ON (dag_run_id) dag_run_id, status, run_created_at, updated_at, status_data
    FROM dagu_dag_run_attempts
    WHERE is_root
      AND dag_name = sqlc.arg(dag_name)
      AND NOT hidden
    ORDER BY dag_run_id, attempt_created_at DESC, id DESC
)
SELECT dag_run_id
FROM latest
WHERE run_created_at < sqlc.arg(cutoff)::timestamptz
  AND updated_at < sqlc.arg(cutoff)::timestamptz
  AND status_data IS NOT NULL
  AND status <> ALL(sqlc.arg(active_statuses)::integer[])
ORDER BY run_created_at ASC, dag_run_id ASC;

-- name: ListRemovableRunsByCount :many
WITH latest AS (
    SELECT DISTINCT ON (dag_run_id) dag_run_id, status, run_created_at, status_data
    FROM dagu_dag_run_attempts
    WHERE is_root
      AND dag_name = sqlc.arg(dag_name)
      AND NOT hidden
    ORDER BY dag_run_id, attempt_created_at DESC, id DESC
),
terminal AS (
    SELECT dag_run_id, run_created_at
    FROM latest
    WHERE status_data IS NOT NULL
      AND status <> ALL(sqlc.arg(active_statuses)::integer[])
),
ranked AS (
    SELECT dag_run_id, run_created_at
    FROM latest
    ORDER BY run_created_at DESC, dag_run_id ASC
    OFFSET sqlc.arg(retention_runs)::integer
),
removable AS (
    SELECT ranked.dag_run_id, ranked.run_created_at
    FROM ranked
    JOIN terminal USING (dag_run_id)
)
SELECT dag_run_id
FROM removable
ORDER BY run_created_at DESC, dag_run_id ASC;

-- name: DeleteDAGRunRows :many
WITH deleted AS (
    DELETE FROM dagu_dag_run_attempts
    WHERE root_dag_name = sqlc.arg(root_dag_name)
      AND root_dag_run_id = sqlc.arg(root_dag_run_id)
    RETURNING dag_run_id
)
SELECT DISTINCT dag_run_id
FROM deleted
ORDER BY dag_run_id;

-- name: RenameDAGRuns :exec
UPDATE dagu_dag_run_attempts
SET dag_name = CASE WHEN is_root AND dag_name::text = sqlc.arg(old_name)::text THEN sqlc.arg(new_name) ELSE dag_name END,
    root_dag_name = CASE WHEN root_dag_name::text = sqlc.arg(old_name)::text THEN sqlc.arg(new_name) ELSE root_dag_name END,
    status_data = CASE
        WHEN is_root
         AND dag_name::text = sqlc.arg(old_name)::text
         AND status_data IS NOT NULL
        THEN jsonb_set(status_data, '{name}', to_jsonb(sqlc.arg(new_name)::text), true)
        ELSE status_data
    END,
    updated_at = now()
WHERE root_dag_name::text = sqlc.arg(old_name)::text
   OR (is_root AND dag_name::text = sqlc.arg(old_name)::text);
