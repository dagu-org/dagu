# Critical Pitfalls

## 1. env as map breaks variable references

Go maps iterate in random order. `env: {A: foo, B: ${A}/bar}` may fail to resolve `${A}`. Use list-of-maps:

```yaml
env:
  - A: foo
  - B: ${A}/bar
```

## 2. Default execution type is chain, not graph

Omitting `type:` gives sequential execution. In `graph` mode without `depends:`, all steps run in parallel.

## 3. chain mode rejects depends

Setting `depends:` on a step in `type: chain` is a validation error.

## 4. output captures stdout only

Stderr goes to log files. Output is trimmed via `strings.TrimSpace()`.

## 5. JSON extraction requires clean stdout

`${VAR.key}` JSON extraction fails if any non-JSON output appears before the JSON object.

## 6. Output capped at 1MB by default

Truncated with `[OUTPUT TRUNCATED]`. Override with `max_output_size:`.

## 7. retry_policy requires both limit and interval_sec

Neither has a default. Omitting either is a validation error.

## 8. continue_on runs after retries exhaust

They are independent — step retries first, then `continue_on: failure` applies.

## 9. Preconditions use exact string match

`expected: "1.2.3"` fails against `"version 1.2.3"`. Use a command that exits 0/1 for pattern matching.

## 10. Step precondition failure skips the step, not the DAG

Use DAG-level `preconditions:` to block the entire DAG.

## 11. Sub-DAGs do not inherit parent env vars

Pass them explicitly via `params:`.

## 12. handler_on, not on_failure

Keys: `init`, `success`, `failure`, `abort`, `exit`, `wait`.

## 13. container is polymorphic

String = exec into existing container. Object = create new with `image:`.

## 14. max_active_steps caps parallelism

In graph mode, limits concurrent step execution.

## 15. No schedule catch-up by default

Missed runs are skipped unless `catchup_window:` is set.

## 16. params values are always strings

Complex types become `fmt.Sprintf("%v")` output. Pass structured data as JSON strings.
