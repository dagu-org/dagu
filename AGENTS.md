# Agent Coding Guide

A file for [guiding coding agents](https://agents.md/).

## Project Structure & Module Organization
- Backend entrypoint in `cmd/` orchestrates the scheduler and CLI; runtime, persistence, and service layers sit under `internal/*` (for example `internal/runtime`, `internal/persistence`).
- API definitions live in `api/v1` and `api/v2`; generated server stubs land in `internal/service`, while matching TypeScript clients flow into `ui/src/api`.
- The React + TypeScript frontend resides in `ui/`, with production bundles copied to `internal/service/frontend/assets` by `make ui`.
- Shared assets, docs, proto schemas, and scripts are grouped in `assets/`, `docs/`, `proto/`, and `scripts/`; integration fixtures live in `internal/test` and `internal/testdata`.

## Build, Test, and Development Commands
- `make run` starts the Go scheduler and serves the compiled UI (fails fast if `ui/dist` is missing).
- `make bin` creates a local `dagu` binary under `.local/bin/` with version metadata baked in.
- `make lint` installs `golangci-lint` to `.local/bin` and runs it across `./...`.
- `make test` (or `make test-coverage`) executes the Go suite via `gotestsum`; append `TEST_TARGET=./internal/...` to focus packages.
- Frontend workflows: `cd ui && pnpm dev` for hot reload, `pnpm build` for production bundles, `pnpm lint` to auto-fix TypeScript/React style issues.

## Coding Style & Naming Conventions
- Keep Go files `gofmt`/`goimports` clean; use tabs, PascalCase for exported symbols (`SchedulerClient`), lowerCamelCase for locals, and `Err...` names for package-level errors.
- Repository linting relies on `golangci-lint`; prefer idiomatic Go patterns, minimal global state, and structured logging helpers in `internal/common`.
- UI code follows ESLint + Prettier (2-space indent) and Tailwind utilities; name React components in PascalCase (`JobList.tsx`) and hooks with `use*` (`useJobs.ts`).

## Testing Guidelines
- Co-locate Go tests as `*_test.go`; favour table-driven cases and cover failure paths.
- Use `stretchr/testify/require` and shared fixtures from `internal/test` instead of duplicating mocks.
- Run `make test-coverage` for coverage targets and `make open-coverage` to inspect the HTML report before merging.

## Commit & Pull Request Guidelines
- Commit summaries follow the Go convention `package: change` (lowercase package or area, present tense summary); keep body paragraphs wrapped at 72 chars when needed.
- Verify `make lint` and `make test` locally; include unit or integration tests whenever behaviour or APIs change.
- PR descriptions must link related issues, outline risk areas, and attach UI screenshots or sample payloads when touching `ui/` or `api/` schemas.
- Use the template at `.github/pull_request_template.md` for every PR; keep all checklist items addressed or justify unchecked boxes in the Additional Notes.
- Call out configuration edits (e.g., under `config/` or cert tooling in `Makefile`) so reviewers can validate deployment impact.
