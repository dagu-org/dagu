#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
STATE_DIR="${DAGU_E2E_STATE_DIR:-$ROOT_DIR/ui/test-results/e2e-stack}"
BIN_PATH="${DAGU_E2E_BIN:-$ROOT_DIR/.local/bin/dagu}"
CONFIG_FILE="$STATE_DIR/config.yaml"
DAGS_DIR="$STATE_DIR/dags"
SERVICE_LOG_DIR="$STATE_DIR/service-logs"
SERVER_PORT="${DAGU_E2E_SERVER_PORT:-32180}"
COORDINATOR_PORT="${DAGU_E2E_COORDINATOR_PORT:-32181}"
WORKER_ID="${DAGU_E2E_WORKER_ID:-worker-1}"
BASE_URL="http://127.0.0.1:${SERVER_PORT}"

if [[ ! -x "$BIN_PATH" ]]; then
  echo "expected Dagu binary at $BIN_PATH; run 'make test-e2e' or 'make bin' after building UI assets" >&2
  exit 1
fi

rm -rf "$STATE_DIR"
mkdir -p "$DAGS_DIR" "$SERVICE_LOG_DIR"

cp "$ROOT_DIR/ui/e2e/fixtures/dags/e2e-distributed-queue.yaml" "$DAGS_DIR/"

cat >"$CONFIG_FILE" <<EOF
debug: true
log_format: text
access_log_mode: none
skip_examples: true
host: 127.0.0.1
port: ${SERVER_PORT}
auth:
  mode: none
permissions:
  write_dags: true
  run_dags: true
paths:
  dags_dir: ${DAGS_DIR}
  log_dir: ${STATE_DIR}/logs
  data_dir: ${STATE_DIR}/data
  suspend_flags_dir: ${STATE_DIR}/suspend-flags
  admin_logs_dir: ${STATE_DIR}/admin-logs
  event_store_dir: ${STATE_DIR}/admin-logs/events
  users_dir: ${STATE_DIR}/users
coordinator:
  enabled: true
  host: 127.0.0.1
  advertise: 127.0.0.1
  port: ${COORDINATOR_PORT}
  health_port: 0
worker:
  id: ${WORKER_ID}
  max_active_runs: 1
  health_port: 0
  labels:
    role: e2e
  # Static coordinator discovery forces the worker down the shared-nothing path.
  coordinators:
    - 127.0.0.1:${COORDINATOR_PORT}
scheduler:
  port: 0
  lock_retry_interval: 250ms
queues:
  enabled: true
  config:
    - name: e2e-shared
      max_concurrency: 1
EOF

service_names=()
service_pids=()

cleanup() {
  local exit_code=$?
  trap - EXIT INT TERM
  for pid in "${service_pids[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill "$pid" 2>/dev/null || true
    fi
  done
  for pid in "${service_pids[@]:-}"; do
    wait "$pid" 2>/dev/null || true
  done
  exit "$exit_code"
}

trap cleanup EXIT INT TERM

start_service() {
  local name="$1"
  shift
  local log_file="$SERVICE_LOG_DIR/${name}.log"

  (
    cd "$ROOT_DIR"
    exec "$BIN_PATH" "$@" --config "$CONFIG_FILE"
  ) >"$log_file" 2>&1 &

  service_names+=("$name")
  service_pids+=("$!")
}

check_services() {
  local index
  for index in "${!service_pids[@]}"; do
    local pid="${service_pids[$index]}"
    local name="${service_names[$index]}"
    if ! kill -0 "$pid" 2>/dev/null; then
      echo "${name} exited unexpectedly" >&2
      if [[ -f "$SERVICE_LOG_DIR/${name}.log" ]]; then
        tail -n 200 "$SERVICE_LOG_DIR/${name}.log" >&2 || true
      fi
      exit 1
    fi
  done
}

wait_for_url() {
  local url="$1"
  local attempts="${2:-300}"
  local sleep_interval="${3:-0.2}"
  local i

  for ((i = 0; i < attempts; i++)); do
    check_services
    if curl --fail --silent --show-error "$url" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$sleep_interval"
  done

  echo "timed out waiting for ${url}" >&2
  exit 1
}

wait_for_worker_registration() {
  local attempts="${1:-300}"
  local sleep_interval="${2:-0.2}"
  local i

  for ((i = 0; i < attempts; i++)); do
    check_services
    if curl --fail --silent --show-error "$BASE_URL/api/v1/workers" | grep -q "\"id\":\"${WORKER_ID}\""; then
      return 0
    fi
    sleep "$sleep_interval"
  done

  echo "timed out waiting for worker registration" >&2
  exit 1
}

start_service coordinator coordinator
start_service worker worker
start_service scheduler scheduler
start_service server server

wait_for_url "$BASE_URL/api/v1/health"
wait_for_worker_registration

echo "Dagu e2e stack is ready at $BASE_URL" >&2

wait
