---
id: "005"
title: "Variable Expansion Syntax Refactoring"
status: draft
---

# RFC 005: Variable Expansion Syntax Refactoring

## Summary

Refactor Dagu's variable expansion system to use a syntax that does not conflict with shell variable expansion. This RFC provides deep analysis of all alternatives including pros, cons, edge cases, and interaction with the params system.

## Motivation

### The Problem

Dagu currently uses `${VAR}` syntax for variable expansion, which is identical to POSIX shell syntax. This creates ambiguity:

```yaml
steps:
  - name: example
    command: echo ${HOME}  # Is this Dagu or shell expansion?
```

**User Intent Scenarios:**

| Scenario | User Wants | Current Behavior | Problem |
|----------|-----------|------------------|---------|
| Use OS variable | Shell expands `$HOME` at runtime | Dagu expands at build time (if in DAG env) | Wrong value if DAG loaded on different machine |
| Use Dagu variable | Dagu expands `${MY_VAR}` | Works, but conflicts with shell | Confusion about when expansion happens |
| Pass literal to container | `${CONTAINER_VAR}` untouched | Dagu may expand it | Container variable lost |
| SSH remote variable | Remote shell expands `$REMOTE_VAR` | Local Dagu expands it | Wrong value, possibly empty |
| Shell parameter expansion | `${VAR:-default}` | Dagu doesn't support this | Syntax error or unexpected behavior |

### Current Escape Mechanism Inadequacy

Single quotes `'${VAR}'` preserve literals, but this approach has problems:

1. **Conflicts with shell quoting**: `echo '${VAR}'` in shell means literal, but in YAML it may be parsed differently
2. **Non-obvious**: Users must know Dagu's escape rules
3. **Fragile**: Mixing quoted and unquoted in same string is confusing
4. **No positive assertion**: No way to explicitly say "this IS a Dagu variable"

---

## Industry Research

### How Other Workflow Engines Handle This

| Engine | Template Syntax | Shell Passes Through | Escape Mechanism |
|--------|----------------|---------------------|------------------|
| **GitHub Actions** | `${{ env.VAR }}` | `${VAR}` | `${{ 'literal' }}` |
| **Airflow** | `{{ ds }}` (Jinja2) | `${VAR}` | `{% raw %}...{% endraw %}` |
| **Argo Workflows** | `{{variable}}` | `${VAR}` | None documented |
| **Kestra** | `{{ expr }}` (Pebble) | `${VAR}` | `render()` function |
| **Concourse CI** | `((variable))` | `${VAR}` | N/A (completely different) |
| **n8n** | `{{ expr }}` | `$VAR` | N/A |
| **GitLab CI** | `${VAR}` | `${VAR}` | `posix_escape` filter |
| **Helm/K8s** | `{{.Values.x}}` | `${VAR}` | N/A |
| **Terraform** | `${var.x}` | N/A | `$${}` for literal |
| **Jenkins** | `${VAR}` or `$VAR` | Same | `\$` escape |

**Key Insight**: Most modern engines deliberately use a **different syntax** from shell to eliminate ambiguity. The exceptions (GitLab CI, Jenkins) have documented pain points around escaping.

### JSON Value Handling in Popular Engines

- **GitHub Actions**: Requires explicit JSON parsing with `fromJSON()` before field access when the value is a string. Properties and indexing work after parsing.
- **Argo Workflows**: Uses an explicit `jsonpath()` function for JSON field access in templates.
- **Airflow**: Variables can be accessed via explicit JSON context (for example `var.json.*`) rather than implicit parsing of strings.
- **n8n**: Expressions access JSON values via `$json` and support JMESPath; JSON strings are typically parsed explicitly in expression code.

**Pattern**: JSON access is explicit in most systems (parse or JSON‑context helpers), which avoids ambiguity and makes errors predictable.

---

## Current Dagu Variable Types

Understanding what needs to be supported:

| Variable Type | Current Syntax | Example | Expansion Time |
|--------------|----------------|---------|----------------|
| DAG env var | `${VAR}` | `${OUTPUT_DIR}` | Build time |
| OS env var | `${VAR}` | `${HOME}` | Configurable |
| Positional param | `$1`, `$2` | `echo $1` | Build time |
| Named param | `${PARAM_NAME}` | `${batch_size}` | Build time |
| Step stdout | `${step.stdout}` | `${download.stdout}` | Runtime |
| Step stderr | `${step.stderr}` | `${fetch.stderr}` | Runtime |
| Step exit code | `${step.exitCode}` | `${check.exitCode}` | Runtime |
| JSON path | `${VAR.path}` | `${response.data.id}` | Runtime |
| String slice | `${VAR:start:len}` | `${uid:0:8}` | Runtime |
| Secrets | `${SECRET}` | `${API_KEY}` | Build time |
| Command substitution | `` `cmd` `` | `` `date +%Y` `` | Build time |

---

## All Syntax Alternatives - Deep Analysis

### Category A: Dollar-Prefixed Syntaxes

#### A1: `${{ VAR }}` (GitHub Actions Style) ⭐ RECOMMENDED

```yaml
env:
  OUTPUT_DIR: "${{ sys.HOME }}/output"
steps:
  - command: curl ${{ env.URL }} -o ${{ env.OUTPUT_DIR }}/data.json
```

**Pros:**
- Clear visual distinction from shell `${VAR}`
- `$` prefix semantically hints "substitution"
- Massive industry familiarity (GitHub Actions)
- Future extensible: `${{ env.X }}`, `${{ secrets.Y }}`, `${{ steps.Z.stdout }}`
- YAML-safe: no escaping needed
- Whitespace tolerant: `${{ VAR }}` and `${{VAR}}` both work

**Cons:**
- More typing (5 chars vs 3 chars)
- New syntax to learn for existing Dagu users

**Edge Cases:**
| Case | Input | Expected Output |
|------|-------|-----------------|
| Adjacent to text | `prefix${{ VAR }}suffix` | `prefixVALUEsuffix` |
| In quotes | `"${{ VAR }}"` | `"VALUE"` |
| Nested braces | `${{ fromJSON(env.data).items[0] }}` | JSON access works after parsing |
| Empty variable | `${{ env.UNDEFINED }}` | Error at evaluation time |
| Escape literal | `$${{ VAR }}` | Literal `${{ VAR }}` |
| Shell alongside | `echo ${{ env.DAGU_VAR }} and ${SHELL_VAR}` | Dagu expands first, shell expands second |

**Params Interaction:**
```yaml
params:
  - batch_size: 100
  - output_dir: "${{ params.batch_size }}_results"  # Reference previous param

steps:
  - command: process --batch ${{ params.batch_size }} --out ${{ params.output_dir }}
  - command: process ${{ args[0] }} ${{ args[1] }}  # Positional params
```

---

#### A2: `$[ VAR ]` (Bracket Style)

```yaml
env:
  OUTPUT_DIR: "$[ HOME ]/output"
```

**Pros:**
- Shorter than double braces
- `$` prefix hints substitution
- YAML-safe

**Cons:**
- `$[...]` is arithmetic expansion in Bash (though deprecated)
- Less industry familiarity
- Brackets commonly used for arrays in many languages

**Edge Cases:**
| Case | Issue |
|------|-------|
| Bash arithmetic | `$[1+2]` conflicts (bash arithmetic, deprecated) |
| Array-like syntax | `$[0]` might confuse users expecting array indexing |

**Not recommended** due to potential Bash arithmetic confusion.

---

#### A3: `$(( VAR ))` (Double Parens)

```yaml
env:
  OUTPUT_DIR: "$(( HOME ))/output"
```

**Pros:**
- Very distinct visually

**Cons:**
- **Critical**: `$((...))` is arithmetic expansion in all POSIX shells
- Would cause runtime shell errors or unexpected behavior

**Rejected** due to direct shell syntax conflict.

---

#### A4: `${ VAR }` (Spaced Braces)

```yaml
env:
  OUTPUT_DIR: "${ HOME }/output"
```

**Pros:**
- Minimal change from current
- Space distinguishes from shell `${VAR}`

**Cons:**
- Very subtle difference, easy to miss
- Shell may still try to expand (behavior varies)
- Not a reliable delimiter

**Not recommended** due to subtle visual difference.

---

### Category B: Mustache/Handlebars Family

#### B1: `{{ VAR }}` (Jinja2/Mustache Style)

```yaml
env:
  OUTPUT_DIR: "{{ HOME }}/output"
steps:
  - command: echo {{ OUTPUT_DIR }}
```

**Pros:**
- Clean, minimal syntax
- Extremely widespread: Jinja2, Django, Airflow, Mustache, Handlebars, Ansible
- YAML-safe
- Easy to type

**Cons:**
- No `$` prefix loses "this is a variable substitution" semantic hint
- Potential future conflict if Dagu adds Go template features
- Some users may expect Jinja2/Handlebars features (filters, conditionals)

**Edge Cases:**
| Case | Input | Notes |
|------|-------|-------|
| No space | `{{VAR}}` | Should work |
| Lots of space | `{{  VAR  }}` | Should work |
| Filters? | `{{ VAR \| upper }}` | Not supported initially |
| Conditionals? | `{% if VAR %}...{% endif %}` | Not supported |

**Params Interaction:**
```yaml
params:
  - batch_size: 100

steps:
  - command: process --batch {{ params.batch_size }}
  - command: process {{ args[0] }} {{ args[1] }}  # Positional
```

---

#### B2: `{{.VAR}}` (Go Template Style)

```yaml
env:
  OUTPUT_DIR: "{{.HOME}}/output"
steps:
  - command: echo {{.OUTPUT_DIR}}
  - command: cat {{.steps.download.stdout}}
```

**Pros:**
- Could leverage Go's `text/template` package directly
- Full expression support (functions, pipelines)
- Familiar to Kubernetes/Helm users
- Established ecosystem

**Cons:**
- Dot prefix is unusual for simple variable access
- Go template learning curve for non-Go developers
- May imply more features than we want to support initially
- Template errors can be cryptic

**Edge Cases:**
| Case | Input | Notes |
|------|-------|-------|
| Simple var | `{{.VAR}}` | Works |
| Nested | `{{.data.items}}` | Go template handles |
| Functions | `{{.VAR \| upper}}` | Could support Go template pipelines |
| Conditionals | `{{if .VAR}}...{{end}}` | Full Go template power |
| Range | `{{range .items}}...{{end}}` | Full Go template power |

**Consideration**: This option is more powerful but introduces significant complexity. If we want full templating, this is attractive. If we want simple substitution, it's overkill.

---

#### B3: `{{{ VAR }}}` (Triple Mustache)

```yaml
env:
  OUTPUT_DIR: "{{{ HOME }}}/output"
```

**Pros:**
- Very distinct
- Cannot conflict with anything

**Cons:**
- Triple braces in Mustache means "unescaped HTML" - semantic mismatch
- Excessive typing (6 chars)
- Unusual looking

**Not recommended** due to semantic confusion and verbosity.

---

### Category C: Alternative Delimiters

#### C1: `(( VAR ))` (Concourse Style)

```yaml
env:
  OUTPUT_DIR: "(( HOME ))/output"
steps:
  - command: echo (( OUTPUT_DIR ))
```

**Pros:**
- Proven in production (Concourse CI)
- Completely distinct from shell
- YAML-safe
- Concourse explicitly chose this after deprecating `{{}}`

**Cons:**
- Looks like mathematical grouping
- No `$` substitution hint
- Less industry familiarity outside Concourse
- Parentheses harder to balance visually

**Edge Cases:**
| Case | Input | Notes |
|------|-------|-------|
| Math-like | `((1+2))` | Might confuse (but not valid var name) |
| Nested | `((data.field))` | Works |
| In expressions | `if (( VAR ))` | Could confuse shell users |

**Historical Note**: Concourse deprecated `{{}}` syntax because:
1. `{{}}` is technically invalid YAML in some edge cases
2. Completely different syntax eliminates any possible confusion

---

#### C2: `[% VAR %]` (Perl Template Toolkit Style)

```yaml
env:
  OUTPUT_DIR: "[% HOME %]/output"
```

**Pros:**
- Very distinct
- No shell conflicts
- Used by Perl Template Toolkit (established)

**Cons:**
- Unfamiliar to most modern developers
- Perl association may feel dated
- Brackets + percent unusual combination

---

#### C3: `<% VAR %>` (ERB/EJS Style)

```yaml
env:
  OUTPUT_DIR: "<% HOME %>/output"
```

**Pros:**
- Familiar to Ruby (ERB) and JavaScript (EJS) developers
- Very distinct from shell

**Cons:**
- Looks like HTML/XML processing instructions
- Angle brackets may need YAML escaping in some contexts
- May imply server-side rendering semantics

---

#### C4: `%{ VAR }` (Terraform Style)

```yaml
env:
  OUTPUT_DIR: "%{ HOME }/output"
```

**Pros:**
- Terraform/HCL familiarity
- Reasonably distinct

**Cons:**
- `%` has formatting meaning in many contexts
- Less intuitive than brace-based syntaxes
- Terraform uses this for string directives, not simple substitution

---

### Category D: Prefix/Sigil Approaches

#### D1: `@VAR` or `@{VAR}` (At-Sign Style)

```yaml
env:
  OUTPUT_DIR: "@{HOME}/output"
steps:
  - command: echo @OUTPUT_DIR or @{OUTPUT_DIR}
```

**Pros:**
- Very short
- Familiar from Razor (ASP.NET)
- No shell conflict

**Cons:**
- `@` is YAML anchor syntax - potential conflict
- `@` in shell is rarely used but exists in some contexts
- May not be visually distinct enough in dense code

---

#### D2: `%VAR%` (Windows Batch Style)

```yaml
env:
  OUTPUT_DIR: "%HOME%/output"
```

**Pros:**
- Familiar to Windows users
- Very distinct from Unix shell

**Cons:**
- Feels dated
- Unix/Linux users unfamiliar
- Dagu primarily targets Unix environments

---

### Category E: Keep ${} with Modifier

#### E1: `$!{ VAR }` (Bang Prefix Inside)

```yaml
# $!{} = Dagu expansion, ${} = shell passthrough
env:
  OUTPUT_DIR: "$!{HOME}/output"
steps:
  - command: echo $!{OUTPUT_DIR} and ${SHELL}
```

**Pros:**
- Minimal change from current syntax
- `!` clearly marks "special" expansion
- Shell `${VAR}` passes through unchanged

**Cons:**
- `!` has meaning in bash (history expansion)
- Unusual looking
- Still using `${}` base syntax

---

#### E2: `${= VAR }` (Equals Prefix Inside)

```yaml
# ${=} = Dagu expansion, ${} = shell passthrough
env:
  OUTPUT_DIR: "${=HOME}/output"
```

**Pros:**
- Minimal change
- `=` implies "evaluate this"

**Cons:**
- Subtle, easy to forget
- `${=VAR}` doesn't parse well visually
- Still conflict-adjacent

---

#### E3: Reverse: Keep ${} for Dagu, use `$\{VAR\}` for shell

```yaml
# ${} = Dagu expansion (current), $\{} = shell passthrough
env:
  OUTPUT_DIR: "${HOME}/output"
steps:
  - command: echo ${DAGU_VAR} and $\{SHELL_VAR\}
```

**Pros:**
- No change to current Dagu syntax
- Explicit escape for shell pass-through

**Cons:**
- Escaping is error-prone
- Backslashes in YAML are tricky
- Users more often want shell pass-through than Dagu expansion

---

### Category F: Context-Based Approaches

#### F1: Field-Level Annotation

```yaml
steps:
  - command: echo ${OUTPUT_DIR}
    expand: dagu  # Controls how ${} is interpreted
```

```yaml
steps:
  - command: echo ${OUTPUT_DIR}
    expand: shell  # Skip Dagu expansion
```

**Pros:**
- Explicit control per-step
- No syntax change

**Cons:**
- Cannot mix in same string
- Verbose
- All-or-nothing per field

---

#### F2: YAML Custom Tags

```yaml
env:
  OUTPUT_DIR: !dagu "${HOME}/output"
  SHELL_VAR: !shell "${PATH}"
```

**Pros:**
- YAML-native
- Very explicit

**Cons:**
- Verbose
- Unfamiliar to most users
- Per-value, not per-reference

---

## Comparison Matrix

| Syntax | Clarity | Familiarity | Shell-Safe | Typing | Future-Proof | Overall |
|--------|---------|-------------|------------|--------|--------------|---------|
| `${{ VAR }}` | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ✅ | 5 chars | ⭐⭐⭐⭐⭐ | **BEST** |
| `{{ VAR }}` | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ✅ | 4 chars | ⭐⭐⭐⭐ | **Strong** |
| `{{.VAR}}` | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ✅ | 4 chars | ⭐⭐⭐⭐⭐ | **If Go templates wanted** |
| `(( VAR ))` | ⭐⭐⭐⭐ | ⭐⭐⭐ | ✅ | 4 chars | ⭐⭐⭐ | Good |
| `$!{VAR}` | ⭐⭐⭐ | ⭐⭐ | ✅ | 3 chars | ⭐⭐ | Minimal change |
| `[% VAR %]` | ⭐⭐⭐ | ⭐⭐ | ✅ | 4 chars | ⭐⭐ | Distinct |
| `<% VAR %>` | ⭐⭐⭐ | ⭐⭐⭐⭐ | ✅ | 4 chars | ⭐⭐ | Web familiar |

---

## Parameters Handling - Deep Dive

### Current Params Behavior

Dagu's params system is flexible and supports multiple formats:

**Format 1: String (space-separated)**
```yaml
params: "batch_size=100 environment=prod"
# Or positional:
params: "value1 value2 value3"
```

**Format 2: List**
```yaml
params:
  - batch_size=100
  - environment=prod
```

**Format 3: Map**
```yaml
params:
  batch_size: 100
  environment: prod
```

**Format 4: With JSON Schema**
```yaml
params:
  schema: ./params-schema.json
  values:
    batch_size: 100
    environment: prod
```

### Params Reference Syntax

Currently, params are referenced as:
- **Positional**: `$1`, `$2`, `$3`, etc.
- **Named**: `${param_name}` (same as env vars)
- **Params become environment variables**: Named params are exported as `NAME=value`

### Problems with Current Approach

1. **Ambiguity**: `${batch_size}` - is this a param, DAG env, or OS env?
2. **Positional syntax**: `$1` is identical to shell positional args
3. **No namespace**: Params, env vars, and secrets share same syntax

### Proposed Params Handling with New Syntax

**Explicit contexts required:**
```yaml
params:
  batch_size: 100
  environment: prod

steps:
  - command: process --batch ${{ params.batch_size }} --env ${{ params.environment }}

  # Positional args use zero-indexed args context:
  - command: process ${{ args[0] }} ${{ args[1] }}

  # Other contexts:
  - command: deploy --key ${{ secrets.API_KEY }} --dir ${{ env.OUTPUT_DIR }}
```

### Params in Params (Self-Reference)

Current behavior allows params to reference previous params:

```yaml
params:
  - base_dir: /data
  - output_dir: "${base_dir}/output"  # References previous param
```

With new syntax:
```yaml
params:
  - base_dir: /data
  - output_dir: "${{ params.base_dir }}/output"
```

Params are evaluated sequentially, so later params can reference earlier ones.
Named params are still exported as environment variables for step execution;
use `params.*` for explicit Dagu expansion, and `$PARAM` for shell expansion.

### Resolution Model

There is **no implicit resolution**. Every reference must use an explicit context.
This removes ambiguity and makes it possible to validate expressions deterministically.

- `${{ env.FOO }}` - DAG/step environment only (step overrides DAG)
- `${{ sys.FOO }}` - OS/process environment only (never merged into `env`)
- `${{ params.FOO }}` - params only
- `${{ secrets.FOO }}` - secrets only
- `${{ steps.X.stdout }}` - step output only

---

## Step Output References

### Current Syntax
```yaml
steps:
  - name: download
    command: curl -o /tmp/data.json https://api.example.com/data

  - name: process
    command: jq '.items[]' ${download.stdout}
    depends: download
```

### Proposed Syntax Options

**Option 1: Flat namespace (current style)**
```yaml
- command: jq '.items[]' ${{ download.stdout }}
```

**Option 2: Explicit steps context**
```yaml
- command: jq '.items[]' ${{ steps.download.stdout }}
- command: jq '.items[]' ${{ steps['build-prod'].stdout }}  # Step names with '-' use bracket form
```

**Recommendation**: Use explicit `${{ steps.X.property }}` for clarity. This:
- Clearly indicates it's a step output, not a variable
- Matches GitHub Actions pattern
- Prevents naming conflicts

### Available Step Properties
```yaml
${{ steps.STEP_NAME.stdout }}      # Captured stdout
${{ steps.STEP_NAME.stderr }}      # Captured stderr
${{ steps.STEP_NAME.exitCode }}   # Exit code (integer as string)
${{ steps.STEP_NAME.stdout[0:100] }}  # Substring of stdout
```

**Compatibility note:** v1 accepted both `exitCode` and `exit_code`. v2 standardizes on
`exitCode` only.

---

## Secrets Handling

### Current
```yaml
env:
  API_KEY: "${SECRET_API_KEY}"
```

### Proposed
```yaml
env:
  API_KEY: "${{ secrets.API_KEY }}"
```

This makes it explicit that the value comes from the secrets store, improving:
- Security auditing
- Code readability
- IDE tooling potential

Secret values are masked in logs and captured outputs.

---

## Command Substitution

### Current (Backticks)
```yaml
env:
  TODAY: "`date +%Y-%m-%d`"
  COMMIT: "`git rev-parse HEAD`"
```

### Should We Change This?

**Options:**

1. **Keep backticks**: `` `cmd` `` - familiar from shell
2. **Add alternative**: `${{ $(cmd) }}` - nested shell-style
3. **Add function**: `${{ exec('date +%Y-%m-%d') }}` - explicit function

**Recommendation**: Keep backticks for now. They work and are familiar. A separate RFC could address this if needed.

Backticks can be escaped with `\`` inside strings.
Command substitution runs at DAG load time.

---

## Expression Language (v1)

Expressions inside `${{ ... }}` are **single-expression**, side-effect-free, and deterministic.
They are evaluated when the containing field is evaluated (DAG load for DAG-level fields,
step runtime for step-level fields). Syntax errors are **build-time** failures.

### Lexical Rules
- **Identifiers**: `[A-Za-z_][A-Za-z0-9_]*`
- **Strings**: single-quoted, `''` escapes a literal `'` (e.g., `'it''s'`)
- **Numbers**: JSON number format
- **Booleans/Null**: `true`, `false`, `null`

### Context Access
The **root identifier must be one of**: `env`, `sys`, `params`, `args`, `secrets`, `steps`.

```yaml
${{ env.OUTPUT_DIR }}
${{ sys.PATH }}
${{ params.batch_size }}
${{ args[0] }}
${{ secrets.API_KEY }}
${{ steps.download.stdout }}
${{ steps['build-prod'].stdout }}  # bracket form for non-identifier keys
```

### Literal Escapes
To output a literal `${{ ... }}` sequence, prefix with an extra `$`:
`$${{ env.NOT_EXPANDED }}` → literal `${{ env.NOT_EXPANDED }}`.

Quotes do not disable Dagu expansion; `${{ ... }}` is always processed.

### Indexing and Slicing
- **Indexing**: `[index]` for arrays/strings, `['key']` for objects.
- **Slicing**: `[start:stop]` for arrays/strings (0-based, end-exclusive).
  Negative indexes are allowed. `step` (`[start:stop:step]`) is reserved for future.

```yaml
${{ env.ID[0:8] }}
${{ steps.download.stdout[0:200] }}
${{ fromJSON(env.response).items[0] }}
```

**Compatibility note:** v1 used `${VAR:start:length}` (length, not stop) and
required JSON paths to start with a dot. v2 uses `[start:stop]` and allows
direct indexing after `fromJSON()`.

### JSON Parsing
- `fromJSON(str)` parses a JSON string into an object/array/number/bool/null.
- Access JSON using `.prop` or `['prop']` and `[index]`.
- Indexing a non-array/non-object is an error.
 
**Compatibility note:** v1 allowed jq‑style access like `${VAR.field}` and
silently left invalid JSON or missing paths unchanged. v2 requires explicit
`fromJSON()` and raises errors on invalid JSON or missing paths.

### Functions (v1)
- `fromJSON(str)`
- `string(x)`, `number(x)`, `bool(x)` (explicit conversions)

No default-value or conditional helpers in v1.

### Type System and Coercion
- **No implicit coercion.** Mixed-type comparisons are errors.
- Equality and ordering are only valid for compatible types.

### Operators (v1)
- Logical: `!`, `&&`, `||`
- Comparison: `==`, `!=`, `<`, `<=`, `>`, `>=`
- Arithmetic: `+`, `-`, `*`, `/`, `%` (numbers only)

String concatenation is not supported inside expressions; use interpolation in the surrounding string.

### Error Surfacing
- **Parse/type errors**: build-time error with file, line, column, and expression snippet.
- **Missing keys**: error at evaluation time.
  - For `env/params` at DAG load if known.
  - For `sys/steps` at step evaluation time (runtime).
- **JSON parse errors**: runtime error with the failing expression and cause.

Example error format:
```
Error: invalid expression at workflow.yml:12:18: unknown key 'FOO' in context 'env'
```

---

## Evaluation Timing and Executor Behavior

### When Expressions Are Evaluated

- **DAG load**: `params:` values are evaluated. `sys.*` used here captures the
  process environment at load time.
- **Step runtime**: DAG-level `env:` values, step `env:` values, step `command:`,
  and executor configs are evaluated right before execution. `sys.*`, `steps.*`,
  and `secrets.*` are available at runtime.

Using `secrets.*` in `params:` is invalid because params are evaluated at load time.
Using `secrets.*` in DAG-level `env:` is allowed and evaluated at runtime.

**Compatibility note:** v1 implicitly expanded OS env only in DAG-level `env:`.
v2 requires explicit `sys.*` and allows it wherever expressions are evaluated.

### Shell vs Non-Shell Executors

- **Shell (`command`)**: Dagu expands only `${{ ... }}`. `$VAR` and `${VAR}`
  are passed through for the shell to expand at runtime.
- **Non-shell executors** (docker/http/ssh/mail/jq, etc.): Dagu expands all
  `${{ ... }}` expressions before invoking the executor. There is no shell
  expansion phase, so OS env access must use `sys.*`.

---

## Recommendations

### Primary Recommendation: `${{ context.VAR }}` with Explicit Contexts

**Final Syntax:**
```yaml
env:
  OUTPUT_DIR: "${{ sys.HOME }}/output"   # Explicit sys context (OS env)
  API_KEY: "${{ secrets.API_KEY }}"       # Explicit secrets

params:
  batch_size: 100
  environment: dev

steps:
  - name: download
    command: curl ${{ env.URL }} -o ${{ env.OUTPUT_DIR }}/data.json

  - name: process
    command: |
      jq '.items[]' ${{ steps.download.stdout }} > output.json
      echo "Processed ${{ params.batch_size }} items in ${{ params.environment }}"
    depends: download

  - name: deploy
    command: deploy.sh ${{ args[0] }} ${{ args[1] }}  # Zero-indexed positional
```

### Available Contexts

| Context | Description | Example |
|---------|-------------|---------|
| `env` | DAG/step environment variables | `${{ env.OUTPUT_DIR }}` |
| `sys` | OS/process environment variables | `${{ sys.HOME }}` |
| `params` | Named parameters | `${{ params.batch_size }}` |
| `args` | Positional arguments (zero-indexed) | `${{ args[0] }}` |
| `secrets` | Secret values | `${{ secrets.API_KEY }}` |
| `steps` | Step outputs | `${{ steps.download.stdout }}` |

### Secondary Recommendation: `{{ context.VAR }}`

If the team prefers shorter syntax without `$`:
```yaml
env:
  OUTPUT_DIR: "{{ sys.HOME }}/output"

steps:
  - command: curl {{ env.URL }} -o {{ env.OUTPUT_DIR }}/data.json
```

### Not Recommended

- `$(( ))` - shell arithmetic conflict
- `$[  ]` - bash arithmetic conflict
- `$${VAR}` - confusing escape semantics (legacy approach)
- YAML tags - too verbose
- Bare names without context - ambiguous

---

## Rules and Behavior

All variable references must use explicit context prefixes; bare names are not allowed.

| Context | Syntax | Example |
|---------|--------|---------|
| DAG/step env vars | `${{ env.NAME }}` | `${{ env.OUTPUT_DIR }}` |
| OS env vars | `${{ sys.NAME }}` | `${{ sys.HOME }}` |
| Parameters | `${{ params.NAME }}` | `${{ params.batch_size }}` |
| Positional args | `${{ args[N] }}` | `${{ args[0] }}`, `${{ args[1] }}` |
| Secrets | `${{ secrets.NAME }}` | `${{ secrets.API_KEY }}` |
| Step outputs | `${{ steps.ID.property }}` | `${{ steps.download.stdout }}` |

`env` is DAG/step environment only. `sys` is OS/process environment only. They are never merged, and
`env` does not implicitly fall back to `sys`.

Positional arguments use zero-indexed array syntax: `${{ args[0] }}`, `${{ args[1] }}`, etc.

New syntax only. Old `${VAR}` syntax will pass through to shell unchanged.

`$VAR` and `${VAR}` are treated as shell literals and are not expanded by Dagu.
Use `${{ ... }}` for Dagu expansion.

Undefined variables cause evaluation to fail with an error. This applies at DAG load for
`env/params/secrets` where available, and at step runtime for `sys/steps`.

Default value syntax like `${{ env.VAR || 'default' }}` is not supported in v1.
Handle defaults in scripts if needed:

```yaml
steps:
  - command: VAR="${VAR:-default}"; echo $VAR
```
