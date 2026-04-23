# UI Component Layer

Use `@/components/ui/*` as the only shared component-library import path.

The files in `src/ui/*` are compatibility shims for older imports. New code
should not import from `@/ui/*` or relative `../ui/*` paths.

## Conventions

- Build app-specific components from these primitives instead of importing
  Radix primitives directly in feature code.
- Use `@/components/ui/dialog` for modal behavior.
- Use `@/components/ui/confirm-dialog` for confirmation flows.
- Use `@/components/ui/table` for table markup and TanStack Table for table
  state.
- Use `@/components/ui/status-chip` for DAG and DAG run status labels.
- Use `@/components/ui/action-button` for compact icon actions.

ESLint blocks legacy `src/ui` imports and relative `components/ui` imports so
new code keeps a single import style.
