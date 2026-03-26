# Feature Request: `template` executor for inline text rendering

## Problem

Every DAG that produces a report, config file, or notification body devolves into 40-60 lines of `echo` statements in a shell script. The content structure is buried in shell noise (`mkdir -p`, `exec >`, `echo ""`, `if/while/done`). This makes report steps:

- Hard to read (content is invisible behind shell syntax)
- Error-prone (`set -e` + `grep -c` returning exit 1, quoting issues)
- Untestable (you must run the whole DAG to see the output)
- Copy-paste heavy (every DAG reinvents the same `echo` patterns)

### Real example from a working DAG

A step that compiles a domain availability report currently requires this:

```yaml
- id: compile_report
  script: |
    set -e
    mkdir -p "${DAG_DOCS_DIR}"
    TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
    OUTFILE="${DAG_DOCS_DIR}/${TIMESTAMP}_report.md"
    AVAIL_DOMAINS=$(grep '^AVAIL:' "${check_domains.stdout}" | sed 's/^AVAIL://' || true)
    TAKEN_DOMAINS=$(grep '^TAKEN:' "${check_domains.stdout}" | sed 's/^TAKEN://' || true)
    AVAIL_COUNT=0
    if [ -n "$AVAIL_DOMAINS" ]; then
      AVAIL_COUNT=$(echo "$AVAIL_DOMAINS" | grep -c '\.com$' || true)
    fi

    exec > "$OUTFILE"
    echo "# Domain Report"
    echo ""
    echo "**Hint:** $(printenv hint)"
    echo "**Available:** ${AVAIL_COUNT}"
    echo ""
    if [ -n "$AVAIL_DOMAINS" ] && [ "$AVAIL_COUNT" -gt 0 ] 2>/dev/null; then
      echo "| # | Domain | Status |"
      echo "|---|--------|--------|"
      i=1
      echo "$AVAIL_DOMAINS" | while IFS= read -r d; do
        [ -z "$d" ] && continue
        echo "| ${i} | **${d}** | Available |"
        i=$((i + 1))
      done
    else
      echo "*No available domains found.*"
    fi
    # ... 30 more lines for taken domains, summary, sources
```

## Proposal

A new built-in executor `type: template` that renders `script:` as a Go template and writes the result to a file or captures it as step output.

```yaml
- id: compile_report
  type: template
  config:
    output: "${DAG_DOCS_DIR}/${TIMESTAMP}_report.md"
    data:
      hint: ${hint}
      available: ${AVAIL_DOMAINS}
      taken: ${TAKEN_DOMAINS}
  script: |
    # Domain Report

    **Hint:** {{ .hint }}
    **Available:** {{ .available | split "\n" | count }}

    {{ if .available | empty | not }}
    | # | Domain | Status |
    |---|--------|--------|
    {{ range $i, $d := .available | split "\n" }}
    | {{ add $i 1 }} | **{{ $d }}** | Available |
    {{ end }}
    {{ else }}
    *No available domains found.*
    {{ end }}

    <details><summary>Taken ({{ .taken | split "\n" | count }})</summary>

    {{ range .taken | split "\n" }}
    - ~~{{ . }}~~
    {{ end }}

    </details>
```

## Design principles

1. **The content IS the step.** The template reads like the final output, not like code that produces output.
2. **No log scraping.** Data comes in via `config.data`, fully resolved by Dagu before the executor runs. The executor never reads log files or step stdout paths directly.
3. **Fits existing patterns.** Just another executor like `jq`, `http`, or `mail`. Uses `script:` for inline content (same as `jq`). No new top-level YAML fields.
4. **Go templates.** Already in stdlib, zero new dependencies, Dagu authors already know Go.

## Behavior

| Scenario | Behavior |
|----------|----------|
| `config.output` is set | Renders to file (auto `mkdir -p` on parent dir). Step stdout captures the file path. |
| `config.output` is unset | Renders to stdout, capturable via `output:` on the step. |
| Template parse error | Step fails immediately with template line number in error message. |
| Render produces empty/whitespace-only | Step succeeds, no file written, stdout is empty. |
| `config.data` key is missing | Go template zero-value behavior (empty string, 0, nil). Use `default` function for explicit fallback. |

## Built-in template functions

### Strings

| Function | Example | Purpose |
|----------|---------|---------|
| `upper` / `lower` | `{{ .name \| lower }}` | Case conversion |
| `title` | `{{ .name \| title }}` | Title Case |
| `trimSpace` | `{{ .input \| trimSpace }}` | Strip leading/trailing whitespace |
| `replace` | `{{ .x \| replace " " "_" }}` | String replacement |
| `truncate` | `{{ .desc \| truncate 80 }}` | Truncate with trailing ellipsis |
| `shellquote` | `{{ .hint \| shellquote }}` | Single-quote escape for safe shell embedding |
| `indent` | `{{ .block \| indent 4 }}` | Indent every line by N spaces |
| `padRight` / `padLeft` | `{{ .col \| padRight 20 }}` | Fixed-width column alignment |

### Collections

| Function | Example | Purpose |
|----------|---------|---------|
| `split` | `{{ .names \| split "\n" }}` | String to list by delimiter |
| `join` | `{{ .items \| join ", " }}` | List to string with delimiter |
| `first` / `last` | `{{ .items \| first }}` | First or last element |
| `count` | `{{ .items \| count }}` | Length of list or string |
| `sortAlpha` | `{{ .names \| sortAlpha }}` | Alphabetical sort |
| `uniq` | `{{ .names \| uniq }}` | Deduplicate adjacent items |
| `filter` | `{{ .lines \| filter "^AVAIL:" }}` | Keep items matching regex |
| `reject` | `{{ .lines \| reject "^#" }}` | Remove items matching regex |

### Defaults and conditions

| Function | Example | Purpose |
|----------|---------|---------|
| `default` | `{{ .x \| default "none" }}` | Fallback for empty values |
| `empty` | `{{ if .items \| empty }}N/A{{ end }}` | Test for empty/zero/nil |
| `ternary` | `{{ ternary "yes" "no" .ok }}` | Inline conditional |
| `coalesce` | `{{ coalesce .a .b .c }}` | First non-empty value |

### Formatting

| Function | Example | Purpose |
|----------|---------|---------|
| `json` | `{{ .data \| json }}` | Marshal as JSON |
| `yaml` | `{{ .data \| yaml }}` | Marshal as YAML |
| `toColumns` | `{{ .rows \| toColumns "\|" }}` | Aligned column output |
| `markdownTable` | `{{ markdownTable .headers .rows }}` | Render a markdown table |

### Date and time

| Function | Example | Purpose |
|----------|---------|---------|
| `now` | `{{ now \| date "2006-01-02" }}` | Current timestamp |
| `date` | `{{ .ts \| date "15:04" }}` | Format a timestamp |

### Paths

| Function | Example | Purpose |
|----------|---------|---------|
| `base` | `{{ .path \| base }}` | Filename from path |
| `dir` | `{{ .path \| dir }}` | Directory from path |
| `ext` | `{{ .path \| ext }}` | File extension |

### Math

| Function | Example | Purpose |
|----------|---------|---------|
| `add` / `sub` | `{{ add .avail .taken }}` | Basic arithmetic |
| `percent` | `{{ percent .avail .total }}` | Formatted percentage (e.g. `"42%"`) |

## Scope boundary

The `template` executor renders text. It does not:

- Read files from disk
- Execute shell commands
- Query APIs or databases
- Access step stdout/stderr file paths

All data enters through `config.data`, fully resolved by Dagu's variable system before the executor is invoked. This preserves the existing separation between captured output (`output:`) and log files (`stdout:`/`stderr:`).

## Implementation notes

- Register as a built-in executor in `internal/runtime/builtin/` alongside `jq`, `http`, `mail`, etc.
- Use Go's `text/template` with a custom `FuncMap` for the built-in functions.
- `config.data` values are strings (per pitfall #16). Functions like `split` convert to lists at render time.
- Template parse errors should include the template line number and a snippet of the failing line.
- File output should use atomic write (write to temp file, then rename) to avoid partial files on failure.
