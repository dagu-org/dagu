# Executor Types

## command / shell (default)

Shell command execution. Uses step `command:`, `script:`, or `shell:` fields.

```yaml
steps:
  - name: example
    command: echo "hello"

  - name: multi-line
    script: |
      echo "step 1"
      echo "step 2"

  - name: custom-shell
    shell: /bin/bash
    script: |
      set -euo pipefail
      echo "running in bash"
```

Aliases: (empty), `command`, `shell`

Step-level fields:
- `command` — Command string to execute
- `args` — Arguments for the command
- `script` — Multi-line shell script content
- `shell` — Shell interpreter (e.g., `/bin/bash`)

## docker

Run commands in Docker containers.

```yaml
steps:
  - name: build
    type: docker
    config:
      image: golang:1.23
      pull: always
      auto_remove: true
      working_dir: /app
      volumes:
        - /local/src:/app
    command: go build ./...
```

Aliases: `docker`, `container`

Config fields:
- `image` — Docker image (required unless `container_name` is set)
- `container_name` — Name/ID of existing container for exec mode
- `pull` — Image pull policy: `always`, `never`, `missing` (default)
- `platform` — Target platform (e.g., `linux/amd64`)
- `auto_remove` — Remove container after exit
- `working_dir` — Working directory inside container
- `volumes` — Volume mounts (list of `host:container` strings)
- `shell` — Shell wrapper for step commands (e.g., `["/bin/bash", "-c"]`)
- `startup` — Startup mode: `keepalive` (default), `entrypoint`, `command`
- `wait_for` — Wait condition: `running` (default), `healthy`
- `log_pattern` — Regex pattern to wait for in container logs
- `container` — Docker SDK container config (env, entrypoint, cmd, user, etc.)
- `host` — Docker SDK host config (privileged, binds, mounts, etc.)
- `network` — Docker SDK network config
- `exec` — Exec options for running commands in an existing container

## dag

Execute another DAG as a sub-step.

```yaml
steps:
  - name: child
    type: dag
    call: child-workflow
    params:
      input: /data/file.csv
```

Aliases: `dag`, `subworkflow`

Uses step `call:` and `params:` fields. Sub-DAGs do not inherit parent env vars.

## parallel

Execute same DAG multiple times in parallel. Requires `call:` field.

```yaml
steps:
  # Simple list of items
  - name: fan-out
    call: process-item
    parallel:
      - item1
      - item2
      - item3

  # Object form with concurrency control
  - name: fan-out-limited
    call: process-item
    parallel:
      items:
        - item1
        - item2
        - item3
      max_concurrent: 5

  # Items with key-value parameters
  - name: fan-out-params
    call: process-item
    parallel:
      items:
        - SOURCE: s3://customers
        - SOURCE: s3://products

  # Variable reference (JSON array)
  - name: fan-out-dynamic
    call: process-item
    parallel: ${ITEMS}
```

Config fields:
- `items` — Array of items to process (strings or key-value param maps)
- `max_concurrent` — Max parallel executions (default 10)

Each parallel invocation receives the current item as the `ITEM` variable.

## ssh

Remote command execution.

```yaml
steps:
  - name: remote
    type: ssh
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      timeout: 60s
      strict_host_key: true
    command: systemctl restart app
```

Config fields:
- `user` — SSH username
- `host` — Hostname (alias: `ip`)
- `port` — Port (default 22)
- `key` — Path to private key
- `password` — Password
- `shell` — Remote shell interpreter
- `shell_args` — Shell arguments
- `timeout` — Connection timeout (default 30s, accepts duration strings like `60s`, `1m`)
- `strict_host_key` — Enable strict host key checking (default true)
- `known_host_file` — Path to known_hosts file
- `bastion` — Jump host config with fields: `host`, `port`, `user`, `key`, `password`

## sftp

File transfer over SSH.

```yaml
steps:
  - name: upload
    type: sftp
    config:
      user: deploy
      host: server.example.com
      key: ~/.ssh/id_rsa
      direction: upload
      source: /local/file.tar.gz
      destination: /remote/file.tar.gz
```

Config: same SSH connection fields (`user`, `host`, `port`, `key`, `password`, `strict_host_key`, `known_host_file`, `timeout`) plus:
- `direction` — `upload` (default) or `download`
- `source` — Source file/directory path (required)
- `destination` — Destination file/directory path (required)

## http

HTTP requests.

```yaml
steps:
  - name: api-call
    type: http
    command: "POST https://api.example.com/data"
    config:
      headers:
        Authorization: "Bearer ${TOKEN}"
        Content-Type: application/json
      body: '{"key": "value"}'
      json: true
      timeout: 30
```

Command format: `"METHOD URL"`. Config fields:
- `timeout` — Request timeout in seconds
- `headers` — HTTP headers (map)
- `query` — Query parameters (map)
- `body` — Request body string
- `silent` — Suppress headers/status output on success
- `debug` — Enable debug mode
- `json` — Format output as JSON
- `skip_tls_verify` — Skip TLS certificate verification

## jq

JSON processing.

```yaml
steps:
  - name: transform
    type: jq
    command: ".items[] | {name: .name, count: .quantity}"
    script: '{"items": [{"name": "a", "quantity": 1}]}'
```

Query from `command:`, input JSON from `script:`. Config fields:
- `raw` — Output raw strings without JSON encoding (like `jq -r`)

## postgres

PostgreSQL queries.

```yaml
steps:
  - name: query
    type: postgres
    config:
      dsn: "postgres://user:pass@localhost:5432/db"
      output_format: json
      timeout: 120
      transaction: true
      isolation_level: read_committed
    script: "SELECT * FROM users WHERE active = true"
```

SQL from `command:` (single statement) or `script:` (multiple statements, also supports `file:///path/to/file.sql`).

Config fields:
- `dsn` — Connection string (required)
- `params` — Query parameters (map or array)
- `timeout` — Query timeout in seconds
- `transaction` — Wrap in transaction
- `isolation_level` — Transaction isolation: `default`, `read_committed`, `repeatable_read`, `serializable`
- `advisory_lock` — Named advisory lock
- `output_format` — `jsonl`, `json`, `csv`
- `headers` — Include headers in CSV output
- `null_string` — NULL representation string
- `max_rows` — Maximum rows to return
- `streaming` — Stream results to file
- `output_file` — File path for streaming output
- `import` — Bulk import config (see below)

Import config fields:
- `input_file`, `table`, `format`, `has_header`, `delimiter`, `columns`, `null_values`, `batch_size`, `on_conflict`, `conflict_target`, `update_columns`, `skip_rows`, `max_rows`, `dry_run`

## sqlite

SQLite queries. Same config struct as postgres. SQLite-specific fields:
- `shared_memory` — Enable shared cache for `:memory:` databases across DAG steps
- `file_lock` — File-based locking for exclusive access

Note: `advisory_lock` is postgres-only and will error on SQLite.

```yaml
steps:
  - name: query
    type: sqlite
    config:
      dsn: "/path/to/db.sqlite"
      output_format: json
      max_rows: 1000
    script: "SELECT * FROM logs"
```

## redis

Redis operations.

```yaml
steps:
  - name: cache-set
    type: redis
    config:
      url: "redis://localhost:6379"
      command: SET
      key: mykey
      value: myvalue
      ttl: 3600

  - name: hash-set
    type: redis
    config:
      url: "redis://localhost:6379"
      command: HSET
      key: myhash
      fields:
        name: "John"
        age: 30
```

Config fields:

Connection:
- `url` — Redis URL (e.g., `redis://user:pass@host:port/db`)
- `host`, `port`, `password`, `username`, `db` — Alternative to `url`
- `tls`, `tls_cert`, `tls_key`, `tls_ca`, `tls_skip_verify` — TLS options
- `mode` — `standalone` (default), `sentinel`, `cluster`
- `sentinel_master`, `sentinel_addrs` — Sentinel mode
- `cluster_addrs` — Cluster mode
- `max_retries` — Max retry attempts
- `timeout` — Command timeout in seconds

Command execution:
- `command` — Redis command (e.g., `SET`, `GET`, `HSET`, `LPUSH`, etc.)
- `key`, `keys` — Key(s) to operate on
- `value`, `values` — Value(s)
- `field`, `fields` — Hash field(s)
- `ttl` — Expiration in seconds
- `nx`, `xx` — SET conditionals
- `keep_ttl` — Preserve existing TTL
- `count`, `match` — SCAN options

List options: `position`, `pivot`, `start`, `stop`
Sorted set options: `score`, `min`, `max`, `with_scores`
Pub/Sub: `channel`, `channels`, `message`
Streams: `stream`, `stream_id`, `group`, `consumer`, `stream_fields`, `max_len`, `block`, `no_ack`
Scripting: `script`, `script_file`, `script_sha`, `script_keys`, `script_args`
Pipeline/Transaction: `pipeline`, `watch`, `multi`
Distributed lock: `lock`, `lock_timeout`, `lock_retry`, `lock_wait`
Output: `output_format` (`json`, `jsonl`, `raw`, `csv`), `null_value`, `max_result_size`

## s3

S3 operations.

```yaml
steps:
  - name: upload
    type: s3
    command: upload
    config:
      region: us-east-1
      bucket: my-bucket
      key: data/output.csv
      source: /local/output.csv
      content_type: text/csv
      storage_class: STANDARD_IA
      sse: AES256
      tags:
        env: production
```

Commands: `upload`, `download`, `list`, `delete`.

Config fields:

AWS connection:
- `region` — AWS region
- `endpoint` — Custom S3-compatible endpoint
- `access_key_id`, `secret_access_key`, `session_token` — Credentials
- `profile` — AWS credentials profile name
- `force_path_style` — Path-style addressing (for S3-compatible services)
- `disable_ssl` — Disable SSL (local testing only)

Object operations:
- `bucket` — S3 bucket name (required)
- `key` — Object key (required for upload/download/delete)
- `source` — Local file path (required for upload)
- `destination` — Local file path (required for download)
- `content_type` — Content-Type for upload
- `storage_class` — Storage class (e.g., `STANDARD`, `STANDARD_IA`, `GLACIER`)
- `metadata` — Custom metadata (map)
- `acl` — Canned ACL (`private`, `public-read`, etc.)
- `sse` — Server-side encryption: `AES256` or `aws:kms`
- `sse_kms_key_id` — KMS key ID (required when sse is `aws:kms`)
- `tags` — Object tags (map)

Transfer options:
- `part_size` — Multipart upload part size in MB (default 10, min 5)
- `concurrency` — Concurrent upload/download parts (default 5)

List options:
- `prefix`, `delimiter`, `max_keys`, `recursive`, `output_format` (`json`, `jsonl`)

Delete options:
- `quiet` — Suppress output

## mail

Send email.

```yaml
steps:
  - name: notify
    type: mail
    config:
      from: noreply@example.com
      to: team@example.com
      subject: "Build Complete"
      message: "The build finished successfully."
```

Config fields:
- `from` — Sender email address
- `to` — Recipient(s) (string or array of strings)
- `subject` — Email subject line
- `message` — Email body content

Note: SMTP server is configured via Dagu's global settings, not per-step.

## archive

Archive operations.

```yaml
steps:
  - name: compress
    type: archive
    command: create
    config:
      source: /data/output
      destination: /data/output.tar.gz
      format: tar.gz
      compression_level: 9
      exclude:
        - "*.tmp"
        - "*.log"

  - name: extract
    type: archive
    command: extract
    config:
      source: /data/archive.zip
      destination: /data/extracted
      overwrite: true
      strip_components: 1
```

Commands: `create`, `extract`, `list`.

Config fields:
- `source` — File or directory to archive/extract (required)
- `destination` — Archive file path (required for create)
- `format` — Archive format: `zip`, `tar`, `tar.gz`, etc. (inferred from filename if omitted)
- `compression_level` — Compression level (-1 for default)
- `password` — Password (extract/list only)
- `overwrite` — Overwrite existing files
- `preserve_paths` — Preserve directory structure (default true)
- `strip_components` — Strip leading path components
- `include` — Glob patterns to include
- `exclude` — Glob patterns to exclude
- `dry_run` — Simulate without making changes
- `verbose` — Enable verbose output
- `follow_symlinks` — Follow symbolic links
- `verify_integrity` — Verify archive integrity
- `continue_on_error` — Continue on errors

## gha

Run GitHub Actions locally.

```yaml
steps:
  - name: checkout
    type: gha
    command: "actions/checkout@v4"
    config:
      runner: catthehacker/ubuntu:act-latest
      auto_remove: true
      privileged: false
```

Aliases: `gha`, `github_action`, `github-action`

Config fields:
- `runner` — Docker image for runner
- `auto_remove` — Remove container after execution
- `network` — Docker network mode
- `privileged` — Privileged mode
- `github_instance` — GitHub instance for action resolution
- `docker_socket` — Custom Docker socket path
- `reuse_containers` — Reuse containers between runs
- `force_rebuild` — Force rebuild of action images
- `container_options` — Additional Docker run options
- `artifacts` — Artifact server config (`path`, `port`)
- `capabilities` — Linux capabilities (`add`, `drop`)

Step-level fields:
- `command` — GitHub Action reference (e.g., `actions/checkout@v4`)
- `params` — Action input parameters

## chat

LLM chat step.

```yaml
steps:
  - name: summarize
    type: chat
    llm:
      provider: anthropic
      model: claude-sonnet-4-20250514
      system: "You are a helpful assistant."
      temperature: 0.7
      max_tokens: 2000
      stream: true
    messages:
      - role: user
        content: "Summarize this: ${INPUT}"
```

Uses step `llm:` config and `messages:` for conversation.

LLM config fields:
- `provider` — LLM provider: `openai`, `anthropic`, `gemini`, `openrouter`, `local`
- `model` — Model name (string) or array of model entries for fallback
- `system` — System prompt
- `temperature` — Randomness control (0.0–2.0)
- `max_tokens` — Maximum tokens to generate
- `top_p` — Nucleus sampling parameter
- `base_url` — Custom API endpoint
- `api_key_name` — Environment variable name for API key
- `stream` — Enable/disable streaming (default true)
- `tools` — List of DAG names to use as callable tools
- `max_tool_iterations` — Max tool calling rounds (default 10)
- `thinking` — Extended thinking config: `enabled`, `effort` (`low`/`medium`/`high`/`xhigh`), `budget_tokens`, `include_in_output`
- `web_search` — Web search config: `enabled`, `max_uses`, `allowed_domains`, `blocked_domains`, `user_location`

Model fallback (array form):
```yaml
llm:
  model:
    - provider: openai
      name: gpt-4o
    - provider: anthropic
      name: claude-sonnet-4-20250514
```

Message roles: `system`, `user`, `assistant`, `tool`

## agent

AI agent loop with tools.

```yaml
steps:
  - name: research
    type: agent
    agent:
      model: claude-sonnet-4-20250514
      tools:
        enabled:
          - web_search
          - bash
        bash_policy:
          default_behavior: allow
          deny_behavior: ask_user
          rules:
            - name: "Block destructive"
              pattern: "^(rm|truncate|dd)"
              action: deny
      skills:
        - my-skill-id
      prompt: "Research and summarize ${TOPIC}"
      max_iterations: 50
      safe_mode: true
      memory:
        enabled: true
    messages:
      - role: user
        content: "Begin research on ${TOPIC}"
```

Agent config fields (under `agent:`):
- `model` — Model override for this step
- `tools` — Tool configuration:
  - `enabled` — List of enabled tool names
  - `bash_policy` — Bash command security rules:
    - `default_behavior` — `allow` or `deny`
    - `deny_behavior` — `block` or `ask_user`
    - `rules` — Pattern-matching rules (each: `name`, `pattern`, `action`)
- `skills` — Skill IDs the agent can use
- `soul` — Soul ID for agent identity
- `memory` — Memory config: `enabled` (bool)
- `prompt` — Additional instructions appended to system prompt
- `max_iterations` — Max tool call rounds (default 50)
- `safe_mode` — Enable command approval via human review (default true)
- `web_search` — Web search config (same as chat)

Also accepts `messages:` at step level for initial conversation context.

## router

Conditional routing based on expression value. Routes reference existing step names — they do not define inline steps.

```yaml
steps:
  - name: check-status
    command: "curl -s -o /dev/null -w '%{http_code}' https://example.com"
    output: STATUS

  - name: route
    type: router
    value: ${STATUS}
    depends: check-status
    routes:
      "200":
        - handle-ok
      "re:5\\d{2}":
        - handle-error
        - send-alert

  - name: handle-ok
    command: echo "success"

  - name: handle-error
    command: echo "server error occurred"

  - name: send-alert
    command: echo "alerting on-call"
```

Step-level fields:
- `value` — Expression to evaluate (required)
- `routes` — Map of pattern to list of target step names (required)

Pattern matching:
- Exact match: `"200"` matches the literal value `200`
- Regex match: `"re:5\\d{2}"` matches `500`, `502`, etc.
- Catch-all: `"re:.*"` matches anything (sorted last automatically)

Routing rules:
- Routes are evaluated in priority order: exact matches first, then regex, then catch-all
- Each target step can only be targeted by one route pattern
- Multiple targets per route execute in parallel
- Steps not targeted by any matching route are skipped
