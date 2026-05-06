-- +goose Up
CREATE INDEX dagu_dag_run_attempts_cleanup_latest_idx
    ON dagu_dag_run_attempts (dag_name, dag_run_id, attempt_created_at DESC, id DESC)
    WHERE is_root AND NOT hidden;

-- +goose Down
DROP INDEX IF EXISTS dagu_dag_run_attempts_cleanup_latest_idx;
