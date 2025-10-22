# GitHub Actions Executor

Run individual GitHub Actions inside Dagu by delegating execution to [nektos/act](https://github.com/nektos/act). This executor is currently **experimental**, so APIs and behaviour may change in upcoming releases. Expect to provide feedback before depending on it for production use.

## Prerequisites

- Install Dagu by following the [installation guide](/getting-started/installation) so the CLI and scheduler are available locally.
- Docker or a compatible OCI runtime must be available because the executor launches action workloads inside containers.
- Network egress must be permitted so `act` can resolve action bundles and container images.
- Provide a GitHub token with at least `contents:read` scope when cloning private repositories or accessing private actions.

## Quick Start

1. Save the DAG below as `gha-hello.yaml` (or any filename you prefer):

   ```yaml
   steps:
     - name: run-action
       command: actions/hello-world-javascript-action@main
       executor:
         type: github_action
         config:
           runner: node:24-bookworm   # optional; defaults to node:24-bookworm
       params:
         who-to-greet: "Morning"
       output: ACTION_OUTPUT          # capture action outputs for later steps

     - name: inspect-output
       command: echo "You got ${ACTION_OUTPUT}"
   ```

2. Execute the workflow with:

   ```bash
   dagu run gha-hello.yaml
   ```

The first step runs the referenced GitHub Action inside an ephemeral workflow powered by `nektos/act`. Dagu binds the step's working directory into the runner container (so files persist locally), evaluates `params` into the action `with:` block, and exposes the step's `output` as an environment variable for downstream steps. The follow-up command step can read `ACTION_OUTPUT` like any other Dagu variable.

## Scheduled Runs

Let Dagu trigger your GitHub Action on a cron schedule:

```yaml
schedules:
  - cron: "0 7 * * 1-5"      # Weekdays at 07:00 server time

secrets:
  - name: GITHUB_TOKEN
    provider: env
    key: GITHUB_TOKEN

steps:
  - name: collect-pr-count
    executor:
      type: gha
    command: actions/github-script@v7
    params:
      github-token: $GITHUB_TOKEN
      script: |
        const { data } = await github.rest.pulls.list({
          owner: "dagu-org",
          repo: "dagu",
          state: "open",
        });
        core.setOutput("count", data.length);
    output: PR_COUNT

  - name: notify
    command: echo "Open PRs: ${PR_COUNT.count}"
```

The first step runs `actions/github-script` inside an isolated Act runner container, executes the JavaScript snippet, and exposes its output as `PR_COUNT`. The follow-up command uses the standard command executor on the host, so no GitHub Action runtime is required locally. The scheduler evaluates the cron expression using the server timezone and executes the DAG automatically, but you can still run it ad-hoc (`dagu run pr-count.yaml`) before enabling the schedule.

## Basic Usage

```yaml
secrets:
  - name: GITHUB_TOKEN
    provider: env
    key: GITHUB_TOKEN

workingDir: /tmp/gha-workspace

steps:
  - name: checkout
    command: actions/checkout@v4          # Action to run
    executor: gha                         # Aliases: github_action, github-action
    params:
      repository: dagu-org/dagu
      ref: main
      token: "${GITHUB_TOKEN}"            # Evaluation happens at runtime
```

- `command` holds the action reference (`owner/repo@ref`).
- `executor` can be the shorthand `gha` shown above, or a map with `type: gha` when you need to add `config`.
- `workingDir` is set at the DAG level and determines the workspace Dagu mounts into the action container (defaults to the process CWD if omitted).
- `executor.config` contains runner configuration options (see Configuration section below).
- `params` maps directly to the `with:` block in GitHub Actions YAML. Values support the same variable substitution rules as other step fields.

## Working Directory

Actions run in the step's resolved `workingDir`:

- Defaults to the DAG's `workingDir`, or the process CWD if none is configured.
- Override per step with `workingDir` / `dir` when you need a dedicated workspace.
- Files created by actions remain in that directory because the workspace is bind-mounted into the runner container.

## Passing Secrets

- Use the DAG-level `secrets` block to inject sensitive values; they are exposed to the action as GitHub secrets _and_ masked in logs.
- During parameter evaluation you can reference the resolved secret by name (for example `$GHA_TOKEN`). The executor forwards the resolved value to `act`, so action inputs like `token` receive the secret at runtime without additional templating.

```yaml
secrets:
  - name: GHA_TOKEN
    provider: env
    key: GITHUB_TOKEN

workingDir: /tmp/workspace

steps:
  - command: actions/checkout@v4
    executor: gha
    params:
      token: $GHA_TOKEN
      repository: myorg/myrepo
      ref: main
```

## Configuration

The `executor.config` object accepts the following parameters:

### `runner`

- **Type**: `string`
- **Default**: `node:24-bookworm`

### `autoRemove`

- **Type**: `boolean`
- **Default**: `true`

Automatically remove containers after execution. Set to `false` to keep containers for debugging.

### `network`

- **Type**: `string`
- **Default**: `""` (Docker default bridge)

Docker network mode for containers. Options: `bridge`, `host`, `none`, or a custom network name.

### `githubInstance`

- **Type**: `string`
- **Default**: `github.com`

GitHub instance for action resolution. Use for GitHub Enterprise Server (e.g., `github.company.com`).

### `dockerSocket`

- **Type**: `string`
- **Default**: `""` (Docker default socket)

Custom Docker socket path. Examples:
- `/var/run/docker.sock` - Default Unix socket
- `tcp://remote-docker:2375` - Remote Docker daemon
- `/run/user/1000/docker.sock` - Rootless Docker

### `containerOptions`

- **Type**: `string`
- **Default**: `""`

Additional Docker run options passed to the container (e.g., `--memory=2g --cpus=2`).

### `reuseContainers`

- **Type**: `boolean`
- **Default**: `false`

Reuse containers between action runs for performance. May cause state pollution.

### `forceRebuild`

- **Type**: `boolean`
- **Default**: `false`

Force rebuild of action Docker images. Useful when developing custom actions.

### `privileged`

- **Type**: `boolean`
- **Default**: `false`

Run containers in privileged mode (full host access). Required for Docker-in-Docker but poses security risks.

### `capabilities`

- **Type**: `object`
- **Default**: `{}`

Linux capabilities configuration:

```yaml
capabilities:
  add: [SYS_ADMIN, NET_ADMIN]    # Capabilities to add
  drop: [NET_RAW, CHOWN]         # Capabilities to drop
```

### `artifacts`

- **Type**: `object`
- **Default**: `{}`

Artifact server configuration for `actions/upload-artifact` and `actions/download-artifact`:

```yaml
artifacts:
  path: /tmp/dagu-artifacts      # Directory for artifact storage
  port: "34567"                  # Artifact server port
```

## Limitations

- Only single-step workflows are supported today; each Dagu step maps to a single GitHub Action invocation.
- The executor synthesises a minimal `push` event payload. Actions that rely on richer event context may need additional wiring.
- Marketplace actions fetch remote resources on demand. Make sure your environment allows outbound requests and Docker image pulls.

We would love feedback while this feature incubates. Please report issues or ideas in the GitHub Actions executor design discussion.
