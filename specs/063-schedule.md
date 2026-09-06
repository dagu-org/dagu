# Spec 063: Schedule Descriptors

## Status

Implemented.

## Scope

This specification covers calendar descriptors in workflow schedules.
Scheduler dispatch and timing guarantees are outside this conformance suite.

## Goal

Use readable calendar schedules with the same behavior as standard cron.

## Behavior

Dagu accepts these descriptors wherever a cron schedule expression is accepted:

| Descriptor | Canonical expression |
| --- | --- |
| `@hourly` | `0 * * * *` |
| `@daily` | `0 0 * * *` |
| `@weekly` | `0 0 * * 0` |
| `@monthly` | `0 0 1 * *` |
| `@yearly` | `0 0 1 1 *` |

A descriptor has the same next run and canonical identity as its corresponding
cron expression.

## Errors

Unknown descriptors fail validation. Runtime failure, timeout, abort, and
cleanup behavior belong to workflow execution and are outside this scope.

## Examples

```yaml
schedule: "@daily"
steps:
  - run: echo scheduled
```

## Conformance

`conformance/spec063_schedule/` checks descriptor validation and compares the
next run reported by `dagu ls -n` for hourly and equivalent cron schedules.
