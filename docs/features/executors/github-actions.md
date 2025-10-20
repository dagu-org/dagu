# GitHub Actions Executor

Run individual GitHub Actions inside Dagu by delegating execution to [nektos/act](https://github.com/nektos/act). This executor is currently **experimental**, so APIs and behaviour may change in upcoming releases. Expect to provide feedback before depending on it for production use.

## Prerequisites

- Docker or a compatible OCI runtime must be available because the executor launches action workloads inside containers.
- Network egress must be permitted so `act` can resolve action bundles and container images.
- Provide a GitHub token with at least `contents:read` scope when cloning private repositories or accessing private actions.

## Basic Usage

```yaml
secrets:
  - name: GITHUB_TOKEN
    provider: env
    key: GITHUB_TOKEN

steps:
  - name: checkout
    command: actions/checkout@v4          # Action to run
    executor:
      type: gha                           # Aliases: github_action, github-action
      config:
        runner: node:22-bookworm          # Optional; defaults to node:22-bookworm
    params:
      repository: dagu-org/dagu
      ref: main
      token: "${GITHUB_TOKEN}"            # Evaluation happens at runtime
```

- `command` holds the action reference (`owner/repo@ref`).
- `executor.type` must be `gha` (or one of the registered aliases shown above).
- `executor.config.runner` overrides the Docker image used as the virtual runner; omit it to use the default Node.js image.
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

## Example: Repository Checkout

```yaml
workingDir: /tmp/workspace

steps:
  - command: actions/checkout@v4
    executor:
      type: gha
    params:
      repository: dagu-org/dagu
      ref: main
      token: "<github token>"
```

## Limitations

- Only single-step workflows are supported today; each Dagu step maps to a single GitHub Action invocation.
- The executor synthesises a minimal `push` event payload. Actions that rely on richer event context may need additional wiring.
- Marketplace actions fetch remote resources on demand. Make sure your environment allows outbound requests and Docker image pulls.

We would love feedback while this feature incubates. Please report issues or ideas in the GitHub Actions executor design discussion.
