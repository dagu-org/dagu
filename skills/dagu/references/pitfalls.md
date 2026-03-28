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

`${VAR.key}` JSON extraction only works when stdout is clean JSON with no extra prefix or suffix text.

For `type: template`, step `output:` still means "capture stdout into a variable". If you use `config.output`, the rendered template is written to a file instead of stdout, so step `output:` will not contain that file content.

## 5. Output capped at 1MB by default

Truncated with `[OUTPUT TRUNCATED]`. Override with `max_output_size:`.

## 6. retry_policy requires both limit and interval_sec

Neither has a default. Omitting either is a validation error.

## 7. continue_on runs after retries exhaust

They are independent — step retries first, then `continue_on: failed` applies.

## 8. Preconditions use exact string match

`expected: "1.2.3"` fails against `"version 1.2.3"`. Use a command that exits 0/1 for pattern matching.

## 9. Step precondition failure skips the step, not the DAG

Use DAG-level `preconditions:` to block the entire DAG.

## 10. Sub-DAGs do not inherit parent env vars

Pass them explicitly via `params:`.

## 11. handler_on, not on_failure

Keys: `init`, `success`, `failure`, `abort`, `exit`, `wait`.

## 12. container is polymorphic

String = exec into existing container. Object = create new with `image:`.

## 13. max_active_steps caps parallelism

In graph mode, limits concurrent step execution.

## 14. No schedule catch-up by default

Missed runs are skipped unless `catchup_window:` is set.

## 15. params values are always strings

Complex types become `fmt.Sprintf("%v")` output. Pass structured data as JSON strings.

## 16. env: CAN reference params: values

`env:` is evaluated after `params:`, so `${param_name}` works directly in env values:

```yaml
params:
  base: /tmp
env:
  - FULL_PATH: "${base}/output"
```

## 17. Built-in jq executor cannot read files

The `type: jq` executor only accepts inline JSON in `script:`. It cannot read from `${step_id.stdout}` file paths. **Workaround:** use `gh api --jq` for GitHub API JSON, or use a shell step with the `jq` CLI for local files.

## 18. Sub-DAG output variables are not propagated to parent

When a child sub-DAG sets `output:` on its steps, the parent sees `"outputs": {}` in the parallel results. **Workaround:** have the child write results to a shared temp directory, and the parent reads from there after the parallel step completes.

## 19. parallel: only works with call: (sub-DAGs)

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

## 20. No simple expression-based step skipping

Conditional step execution requires `preconditions:` with a shell command + exact string match. There is no `skip_if:` expression syntax. **Workaround:** use a shell test command in preconditions:

```yaml
preconditions:
  - condition: 'test -n "${MY_VAR}" && echo yes'
    expected: "yes"
```

## 21. Avoid shell interpolation for large or arbitrary text content

If you interpolate large JSON or arbitrary text into a `script:` using `${VAR}`, Dagu expands it before the shell runs. Quotes, backticks, `$`, newlines, code blocks, and other shell metacharacters can break shell parsing or downstream tools such as `jq`.

Prefer file-based or non-shell techniques:
- Use `${step_id.stdout}` and read with `cat` or redirection when the content already exists in a previous step output file
- Use `printenv VAR_NAME` when reading raw params or environment values inside a shell step
- Use `type: template` with `config.data` and optional `config.output` when the goal is to generate a text file instead of execute shell logic

```yaml
params:
  - name: user_text
    type: string

steps:
  - id: save_input
    script: |
      printenv user_text > "${DAG_DOCS_DIR}/input.md"
```

## 22. Iterating over multiline output requires `parallel:` or file read

`output: VAR` stores multiline stdout as a single string. `for x in ${VAR}` does not split on newlines — it runs once with the entire string. Use `parallel: ${VAR}` with a sub-DAG (requires `call:`), or read the stdout file: `while IFS= read -r line; do ... done < "${step_id.stdout}"`.

## 23. `repeat_policy.limit` does not accept variable references

`repeat_policy.limit` requires a literal integer in YAML. `limit: ${max_rounds}` fails with a type error at parse time. **Workaround:** set a high fixed limit and enforce the real max inside the repeated step by checking a counter file.
