---
id: "011"
title: "Working Directory Behavior"
status: draft
---

# RFC 011: Working Directory Behavior

## Summary

How `workingDir` is determined, inherited, and applied across DAG execution — including steps, sub-DAGs, SSH, Docker, and scripts.

## Where `workingDir` Can Be Set

| Level | YAML Field | Description |
|-------|------------|-------------|
| DAG | `workingDir` | Working directory for the entire DAG |
| Step | `workingDir` | Per-step override |
| Container | `workingDir` (inside `container:`) | Working directory inside a Docker container |

## How `workingDir` Is Resolved

### DAG-Level

The DAG's `workingDir` is determined by the first match:

1. Explicit `workingDir` in the DAG YAML
2. Value inherited from a parent DAG (for sub-DAGs)
3. Value from `base.yaml` (if set)
4. The directory containing the DAG file
5. Current working directory, or the user's home directory

### Step-Level

Each step's working directory is determined by the first match:

1. The step's own `workingDir` (if set)
2. The DAG's `workingDir`
3. Current working directory, or the user's home directory

### Path Formats

All `workingDir` values support:

- **Absolute paths** (`/foo/bar`) — used as-is
- **Home-relative paths** (`~/foo`) — expanded to the user's home directory
- **Environment variables** (`$MY_DIR`, `${MY_DIR}`) — expanded before use
- **Relative paths** (`./scripts`, `../other`) — resolved against the DAG file's directory at the DAG level, or against the DAG's `workingDir` at the step level

### Directory Creation

Working directories are created automatically if they don't exist.

## Execution-Specific Behavior

### Local Commands and Scripts

Commands run in the resolved working directory. Each step is isolated — one step's directory does not affect another.

For steps with a `script` field, the temporary script file is written to the step's working directory (or the system temp directory if empty). The file is cleaned up after execution.

### Sub-DAGs

- Sub-DAG **with** explicit `workingDir` — uses its own value
- Sub-DAG **without** `workingDir` — inherits the parent step's working directory

### SSH

- **Only uses step-level `workingDir`** — DAG-level `workingDir` is ignored, since it refers to a local path that likely doesn't exist on the remote host
- If the step's `workingDir` is set, the remote command is prefixed with `cd <dir>`
- If not set, the command runs in the SSH user's home directory

### Docker

Docker containers have two separate working directories:

1. **Host-side** — the step's resolved `workingDir`, used only for resolving relative volume mount paths
2. **Container-side** — the `workingDir` inside the `container:` config block, which sets the `WORKDIR` inside the container

These are independent. Setting the step's `workingDir` does NOT change where commands run inside the container.

## Base Config Inheritance

When `workingDir` is set in `base.yaml`, it acts as the default for all DAGs. Any DAG with an explicit `workingDir` overrides the base value. DAGs without one inherit it.

## Dotenv File Resolution

Dotenv files are searched for in order:

1. Relative to the DAG's `workingDir`
2. Relative to the DAG file's directory (if different)

A `.env` file in the working directory is always loaded, even if not listed in the `dotenv` field.

## Schema File Resolution

JSON schema files referenced in DAG parameters are searched for in order:

1. As-is (absolute path or environment variable)
2. Relative to the DAG's `workingDir`
3. Relative to the DAG file's directory

## Quick Reference

| Execution Type | Working Directory Source | Notes |
|---------------|------------------------|-------|
| Local command | Step or DAG `workingDir` | Subprocess isolation |
| Script | Step or DAG `workingDir` | Temp file written to working dir |
| Sub-DAG | Inherited from parent | Unless child has explicit `workingDir` |
| SSH | Step `workingDir` only | DAG-level ignored |
| Docker | Container `workingDir` | Host-side used only for volume paths |
