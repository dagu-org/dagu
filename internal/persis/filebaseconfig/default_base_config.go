package filebaseconfig

// defaultBaseConfig is the content written to base.yaml on first run.
// All fields are commented out — users uncomment what they need.
// This serves as a discoverable reference for all inheritable DAG settings.
const defaultBaseConfig = `# Base DAG Configuration
# =====================
# Values defined here are inherited by ALL DAGs.
# Individual DAGs can override any setting below.
# Environment variables (env:) are additive — DAG env vars append to these.
#
# Everything is commented out by default. Uncomment the settings you need.

# -- Execution Type --
# Default execution model for all DAGs.
# "graph" = dependency-based parallel execution (default)
# "chain" = sequential execution in definition order
# type: graph

# -- Default Shell --
# Shell used for command steps. Can be a string or array.
# shell: "bash -e"
# shell: ["/bin/bash", "-e"]

# -- Working Directory --
# Default working directory for all DAGs.
# working_dir: /home/user/workflows

# -- Dotenv --
# Load environment variables from .env files before DAG execution.
# dotenv: .env
# dotenv:
#   - .env
#   - .env.local

# -- Catchup --
# Automatically run missed scheduled intervals (e.g., after downtime).
# Set a lookback window to enable catchup for all scheduled DAGs.
# catchup_window: "6h"

# -- Overlap Policy --
# How to handle runs triggered while one is already active.
# "skip" (default) | "all" | "latest"
# overlap_policy: skip

# -- Timeout & Delays --
# timeout_sec: 3600
# delay_sec: 0
# restart_wait_sec: 0

# -- History & Cleanup --
# hist_retention_days: 30
# max_clean_up_time_sec: 60

# -- Concurrency --
# max_active_steps: 10

# -- Output --
# max_output_size: 1048576

# -- Logging --
# "separate" = separate .out/.err files (default)
# "merged"   = single combined .log file
# log_output: merged
# log_dir: /var/log/dagu

# -- Queue --
# Assign all DAGs to a named queue for concurrency control.
# queue: default

# -- Environment Variables --
# env:
#   - LOG_LEVEL: "info"
#   - ENVIRONMENT: "production"

# -- Step Defaults --
# Default configuration applied to all steps. Steps can override individually.
# defaults:
#   retry_policy:
#     limit: 2
#     interval_sec: 5
#     backoff: true
#     max_interval_sec: 60
#   continue_on:
#     failure: false
#     skipped: false
#     exit_code: []
#     output: []
#     mark_success: false
#   timeout_sec: 300
#   signal_on_stop: "SIGTERM"
#   mail_on_error: true

# -- Lifecycle Handlers --
# Steps executed on specific DAG events.
# handler_on:
#   init:
#     command: echo "DAG starting"
#   success:
#     command: echo "DAG succeeded"
#   failure:
#     command: echo "DAG failed"
#   abort:
#     command: echo "DAG aborted"
#   exit:
#     command: echo "DAG finished"
#   wait:
#     command: echo "DAG waiting for approval"

# -- Email Notifications --
# smtp:
#   host: smtp.example.com
#   port: 587
#   username: ""
#   password: ""
#
# error_mail:
#   from: dagu@example.com
#   to: alerts@example.com
#   prefix: "[DAGU]"
#   attach_logs: true
#
# info_mail:
#   from: dagu@example.com
#   to: team@example.com
#   prefix: "[DAGU]"
#   attach_logs: false
#
# wait_mail:
#   from: dagu@example.com
#   to: approvers@example.com
#   prefix: "[DAGU APPROVAL]"
#   attach_logs: false
#
# mail_on:
#   failure: true
#   success: false
#   wait: false

# -- SSH Defaults --
# Default SSH configuration inherited by all ssh executor steps.
# ssh:
#   user: deploy
#   host: server.example.com
#   port: 22
#   key: ~/.ssh/id_rsa
#   password: ""
#   strict_host_key: true
#   known_host_file: ~/.ssh/known_hosts
#   shell: "bash"
#   timeout: "30s"
#   bastion:
#     host: bastion.example.com
#     port: 22
#     user: jump
#     key: ~/.ssh/bastion_key

# -- S3 Defaults --
# Default S3 configuration inherited by all s3 executor steps.
# s3:
#   region: us-east-1
#   bucket: my-bucket
#   endpoint: ""
#   access_key_id: ${AWS_ACCESS_KEY_ID}
#   secret_access_key: ${AWS_SECRET_ACCESS_KEY}
#   session_token: ""
#   profile: ""
#   force_path_style: false
#   disable_ssl: false

# -- Redis Defaults --
# Default Redis configuration inherited by all redis executor steps.
# redis:
#   host: localhost
#   port: 6379
#   password: ""
#   username: ""
#   db: 0
#   tls: false
#   tls_skip_verify: false
#   mode: standalone
#   max_retries: 0

# -- LLM Defaults --
# Default LLM configuration inherited by all chat/agent steps.
# llm:
#   provider: openai
#   model: gpt-4o-mini
#   base_url: ""
#   api_key_name: ""
#   system: ""
#   temperature: 0.7
#   max_tokens: 4096
#   top_p: 1.0

# -- OpenTelemetry --
# otel:
#   enabled: true
#   endpoint: http://localhost:4318
#   insecure: true
#   timeout: 10s
#   headers:
#     Authorization: "Bearer token"
#   resource:
#     service.name: dagu

# -- Docker Registry Auth --
# registry_auths:
#   "registry.example.com":
#     username: user
#     password: ${REGISTRY_PASSWORD}

# -- External Secrets --
# secrets:
#   - name: DB_PASSWORD
#     provider: env
#     key: DB_PASSWORD_ENV

# -- Run Config --
# Controls user interactions during DAG runs in the UI.
# run_config:
#   disable_param_edit: false
#   disable_run_id_edit: false
`
