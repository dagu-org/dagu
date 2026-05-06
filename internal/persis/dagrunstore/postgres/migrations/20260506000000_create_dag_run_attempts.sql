-- +goose Up
CREATE DOMAIN dagu_uuid_v7 AS uuid
    CHECK (VALUE::text ~* '^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$');

CREATE DOMAIN dagu_dag_name AS text
    CHECK (
        VALUE <> ''
        AND VALUE <> '.'
        AND VALUE <> '..'
        AND char_length(VALUE) <= 40
        AND VALUE ~ '^[a-zA-Z0-9_.-]+$'
    );

CREATE DOMAIN dagu_dag_run_id AS text
    CHECK (
        VALUE <> ''
        AND char_length(VALUE) <= 64
        AND VALUE ~ '^[-a-zA-Z0-9_]+$'
    );

CREATE DOMAIN dagu_workspace_name AS text
    CHECK (
        VALUE <> ''
        AND lower(VALUE) NOT IN ('all', 'default')
        AND VALUE ~ '^[A-Za-z0-9_-]+$'
    );

CREATE TABLE dagu_dag_run_attempts (
    id dagu_uuid_v7 PRIMARY KEY,
    dag_name dagu_dag_name NOT NULL,
    dag_run_id dagu_dag_run_id NOT NULL,
    root_dag_name dagu_dag_name NOT NULL,
    root_dag_run_id dagu_dag_run_id NOT NULL,
    is_root boolean NOT NULL,
    attempt_id text NOT NULL CHECK (
        attempt_id <> ''
        AND char_length(attempt_id) <= 64
        AND attempt_id ~ '^[-a-zA-Z0-9_]+$'
    ),
    run_created_at timestamptz NOT NULL,
    attempt_created_at timestamptz NOT NULL,
    workspace dagu_workspace_name,
    workspace_valid boolean NOT NULL DEFAULT true,
    status integer,
    started_at timestamptz,
    finished_at timestamptz,
    status_data jsonb,
    dag_data jsonb,
    outputs_data jsonb,
    messages_data jsonb NOT NULL DEFAULT '{}'::jsonb,
    cancel_requested boolean NOT NULL DEFAULT false,
    hidden boolean NOT NULL DEFAULT false,
    local_work_dir text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (workspace IS NULL OR workspace_valid),
    CHECK (status_data IS NULL OR jsonb_typeof(status_data) = 'object'),
    CHECK (dag_data IS NULL OR jsonb_typeof(dag_data) = 'object'),
    CHECK (outputs_data IS NULL OR jsonb_typeof(outputs_data) = 'object'),
    CHECK (jsonb_typeof(messages_data) = 'object')
);

CREATE INDEX dagu_dag_run_attempts_latest_root_idx
    ON dagu_dag_run_attempts (dag_name, dag_run_id, hidden, attempt_created_at DESC, id DESC)
    WHERE is_root AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_latest_sub_idx
    ON dagu_dag_run_attempts (root_dag_name, root_dag_run_id, dag_run_id, hidden, attempt_created_at DESC, id DESC)
    WHERE NOT is_root AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_list_order_idx
    ON dagu_dag_run_attempts (run_created_at DESC, dag_name ASC, dag_run_id ASC)
    WHERE is_root AND NOT hidden AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_workspace_idx
    ON dagu_dag_run_attempts (workspace, run_created_at DESC)
    WHERE is_root AND NOT hidden AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_status_idx
    ON dagu_dag_run_attempts (status, run_created_at DESC)
    WHERE is_root AND NOT hidden AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_started_at_idx
    ON dagu_dag_run_attempts (started_at DESC)
    WHERE is_root AND NOT hidden AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_finished_at_idx
    ON dagu_dag_run_attempts (finished_at DESC)
    WHERE is_root AND NOT hidden AND status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_status_data_gin_idx
    ON dagu_dag_run_attempts USING gin (status_data jsonb_path_ops)
    WHERE status_data IS NOT NULL;

CREATE INDEX dagu_dag_run_attempts_labels_gin_idx
    ON dagu_dag_run_attempts USING gin ((status_data -> 'labels'))
    WHERE status_data IS NOT NULL;

-- +goose Down
DROP TABLE IF EXISTS dagu_dag_run_attempts;
DROP DOMAIN IF EXISTS dagu_workspace_name;
DROP DOMAIN IF EXISTS dagu_dag_run_id;
DROP DOMAIN IF EXISTS dagu_dag_name;
DROP DOMAIN IF EXISTS dagu_uuid_v7;
