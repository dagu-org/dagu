# Bundled Skills

This directory is the source of truth for the skills shipped inside the Dagu binary.

- `dagu/` is the installer/reference skill used by `dagu ai install`.
- `dagu-ai-workflows/`, `dagu-containers/`, and `dagu-server-worker/` are the example skills seeded into `{DAGsDir}/skills` for first-time users.

`embed.go` lives here because Go's `embed` patterns can only read files inside the package directory tree.
