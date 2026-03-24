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

They are independent — step retries first, then `continue_on: failed` applies.

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

## 17. env: CAN reference params: values

`env:` is evaluated after `params:`, so `${param_name}` works directly in env values:

```yaml
params:
  base: /tmp
env:
  - FULL_PATH: "${base}/output"
```

## 18. Built-in jq executor cannot read files

The `type: jq` executor only accepts inline JSON in `script:`. It cannot read from `${step_id.stdout}` file paths. **Workaround:** use `gh api --jq` for GitHub API JSON, or use a shell step with the `jq` CLI for local files.

## 19. Sub-DAG output variables are not propagated to parent

When a child sub-DAG sets `output:` on its steps, the parent sees `"outputs": {}` in the parallel results. **Workaround:** have the child write results to a shared temp directory, and the parent reads from there after the parallel step completes.

## 20. parallel: only works with call: (sub-DAGs)

You cannot use `parallel:` on a regular step to iterate over items. It requires `call:` to a sub-DAG. **Workaround:** define an inline sub-DAG with `---` separator, even for single-step operations.

```yaml
- id: fan_out
  call: my-step
  parallel: ${ITEMS}
---
name: my-step
steps:
  - id: run
    script: echo "Processing ${ITEM}"
```

## 21. No simple expression-based step skipping

Conditional step execution requires `preconditions:` with a shell command + exact string match. There is no `skip_if:` expression syntax. **Workaround:** use a shell test command in preconditions:

```yaml
preconditions:
  - condition: 'test -n "${MY_VAR}" && echo yes'
    expected: "yes"
```

## 22. Large output variables break jq script: interpolation

If a step captures large JSON via `output: VAR`, using `${VAR}` inside a `script:` field (especially for `type: jq`) can break JSON parsing due to unescaped characters. **Workaround:** use `${step_id.stdout}` file paths and read with `cat` in shell steps instead of string interpolation.

## 23. Iterating over multiline output requires `parallel:` or file read

`output: VAR` stores multiline stdout as a single string. `for x in ${VAR}` does not split on newlines — it runs once with the entire string. Use `parallel: ${VAR}` with a sub-DAG (requires `call:`), or read the stdout file: `while IFS= read -r line; do ... done < "${step_id.stdout}"`.

## 24. Use `printenv` for params with arbitrary content

`${param_name}` in a `script:` block is expanded by Dagu before the shell runs. If the param value contains shell metacharacters (backticks, `$`, quotes, newlines, code blocks), the expanded script breaks shell parsing. Step-level `env:` has the same problem — Dagu evaluates backtick expressions found in the expanded value.

**Use `printenv param_name` instead.** Dagu sets all params as OS environment variables. `printenv` reads the raw value from the OS without any shell or Dagu interpretation.

```yaml
params:
  - name: user_text
    type: string

steps:
  - id: save_input
    script: |
      printenv user_text > "${DAG_DOCS_DIR}/input.md"
```

## 25. `repeat_policy.limit` does not accept variable references

`repeat_policy.limit` requires a literal integer in YAML. `limit: ${max_rounds}` fails with a type error at parse time. **Workaround:** set a high fixed limit and enforce the real max inside the repeated step by checking a counter file.
