---
id: "008"
title: "Dollar Sign Escaping in v1 Variable Expansion"
status: draft
---

# RFC 008: Dollar Sign Escaping in v1 Variable Expansion

## Summary

Add two escape mechanisms to the v1 variable expansion system so that users can include literal `$` characters in DAG field values. Specifically:

1. **`$$` → literal `$`**: A doubled dollar sign produces a single literal `$` in the output.
2. **Single-quote stripping**: `'$VAR'` already suppresses expansion (per RFC 006), but the surrounding quotes are currently preserved in the output. This RFC changes the behavior so quotes are stripped, producing `$VAR` instead of `'$VAR'`.

This is a targeted fix for issue [#1628](https://github.com/dagu-org/dagu/issues/1628).

---

## Motivation

### The Problem

Users cannot include a literal `$` in any field that undergoes variable expansion. This breaks real-world patterns involving GraphQL queries, price strings, passwords, and regex patterns.

**GraphQL query (broken today):**

```yaml
steps:
  - name: query
    executor:
      type: http
      config:
        url: https://api.example.com/graphql
        body: '{"query": "{ user(id: $userId) { name } }"}'
```

Dagu attempts to expand `$userId` as a variable reference. If the variable is undefined, the literal `$userId` is preserved — but only by accident (undefined-variable passthrough). If the user happens to have an env var named `userId`, it silently substitutes the wrong value.

**Price string (broken today):**

```yaml
env:
  PRICE: "$9.99"
```

Dagu interprets `$9` as positional parameter 9 and `.99` as a JSON path suffix. The result is unpredictable.

**Password with dollar sign (broken today):**

```yaml
env:
  DB_PASS: "p@ss$word123"
```

Dagu interprets `$word123` as a variable reference.

### Every Escape Attempt Fails

Users have tried every reasonable escape convention — none work:

| Attempt | Input | Expected | Actual |
|---------|-------|----------|--------|
| Backslash | `\$HOME` | `$HOME` | `\$HOME` (backslash preserved, `$HOME` still expanded) |
| Double dollar | `$$HOME` | `$HOME` | `$$HOME` (no `$$` support) or `$<value>` |
| Single quotes | `'$HOME'` | `$HOME` | `'$HOME'` (quotes preserved in output) |
| Double backslash | `\\$HOME` | `\$HOME` | `\\` + expanded value |
| Backtick wrap | `` `echo '$HOME'` `` | `$HOME` | Runs shell command, fragile |

There is no reliable way to produce a literal `$` in the output.

### Root Cause in Code

The expansion pipeline (`internal/cmn/eval/pipeline.go`) processes strings through four phases:

1. **quoted-refs** — resolves `"${FOO.bar}"` patterns inside JSON-like strings
2. **variables** — resolves `$VAR`, `${VAR}`, and JSON path/step references
3. **substitute** — runs backtick command substitution
4. **shell-expand** — performs POSIX-style shell variable expansion

The variable-matching regex in `internal/cmn/eval/resolve.go:11` matches broadly:

```go
var reVarSubstitution = regexp.MustCompile(
    `[']{0,1}\$\{([^}]+)\}[']{0,1}|[']{0,1}\$([a-zA-Z0-9_][a-zA-Z0-9_]*)[']{0,1}`)
```

This regex captures any `$` followed by a valid identifier or brace-delimited expression. There is no escape sequence that prevents a `$` from being interpreted as the start of a variable reference.

The `extractVarKey` function (`resolve.go:115-123`) handles single-quoted matches by returning `false` to skip expansion — but the original match (including surrounding quotes) is preserved verbatim, leaving the quotes in the output.

---

## Proposal

### Fix 1: `$$` Produces Literal `$`

A doubled `$$` is converted to a single `$` before variable expansion runs. This is a pre-processing step inserted at the beginning of the pipeline, before any regex matching occurs.

**Convention alignment:** This matches the escape convention used by Make, Terraform (`$${}`), and GitHub Actions (`$${{ }}`). It is the most intuitive escape for users already familiar with shell-adjacent tools.

**Examples:**

| Input | Output |
|-------|--------|
| `$$HOME` | `$HOME` (literal, not expanded) |
| `$${userId}` | `${userId}` (literal braces preserved) |
| `Price: $$9.99` | `Price: $9.99` |
| `p@ss$$word` | `p@ss$word` |
| `$$$$` | `$$` (each pair collapses) |
| `$HOME` | value of HOME (unchanged behavior) |

### Fix 2: Single-Quote Stripping

When the regex matches a single-quoted variable reference like `'$VAR'` or `'${VAR}'`, the current behavior preserves the quotes in the output. This RFC changes the behavior to **strip the surrounding single quotes**, so the output is the literal variable reference without quotes.

**Examples:**

| Input | Current Output | Proposed Output |
|-------|---------------|-----------------|
| `'$HOME'` | `'$HOME'` | `$HOME` |
| `'${VAR}'` | `'${VAR}'` | `${VAR}` |
| `echo '$HOME'` | `echo '$HOME'` | `echo $HOME` |

This makes single quotes behave as a true escape mechanism rather than a passthrough that leaks quoting artifacts into the output.

**Note:** Single quotes only suppress Dagu expansion when they immediately wrap a variable reference (`'$...'`). Quotes elsewhere in a string are not affected.

---

## Behavior Table

Complete before/after for all escape patterns from issue #1628:

| Input | Before (current) | After (proposed) | Mechanism |
|-------|------------------|-------------------|-----------|
| `$HOME` | Expanded to env value | Expanded to env value | No change |
| `$$HOME` | `$$HOME` or broken | `$HOME` (literal) | `$$` escape |
| `\$HOME` | `\` + expanded value | `\` + expanded value | No change (not an escape) |
| `'$HOME'` | `'$HOME'` (with quotes) | `$HOME` (without quotes) | Quote stripping |
| `'${VAR}'` | `'${VAR}'` (with quotes) | `${VAR}` (without quotes) | Quote stripping |
| `$$9.99` | Broken | `$9.99` | `$$` escape |
| `$${query}` | Broken | `${query}` | `$$` escape |
| `p@ss$$word` | Broken | `p@ss$word` | `$$` escape |
| `$$$$` | Broken | `$$` | Double `$$` collapse |

---

## Relationship to Other RFCs

### RFC 005 — Variable Expansion Syntax Refactoring (draft)

RFC 005 proposes the `${{ context.VAR }}` syntax which eliminates `$VAR`/`${VAR}` ambiguity entirely. Under RFC 005, `$VAR` and `${VAR}` pass through to the shell unchanged, and only `${{ ... }}` is expanded by Dagu.

This RFC (008) is complementary:
- **RFC 008** fixes the immediate inability to escape `$` in v1 syntax.
- **RFC 005** makes escaping unnecessary long-term by using a distinct syntax.
- The `$$` convention aligns with RFC 005's own escape mechanism: `$${{ expr }}` produces a literal `${{ expr }}`.

When RFC 005 ships and v1 syntax is deprecated, the `$$` escape becomes relevant only for edge cases where a literal `$` is needed in a non-shell context (e.g., `$$9.99` in an HTTP body). The mechanism remains useful.

### RFC 006 — Variable Expansion Syntax v1 (implemented)

RFC 006 documents the current v1 behavior including the single-quote escape mechanism. This RFC amends RFC 006's "Escape Mechanisms" section:

- **Before (RFC 006):** Single-quoted variables are "preserved literally and not expanded" — but the quotes remain in output.
- **After (this RFC):** Single-quoted variables are preserved literally, quotes are stripped from output, and `$$` is available as an additional escape.

### RFC 007 — OS Environment Variable Expansion Rules (implemented)

RFC 007 restricts which variables are expanded for non-shell executors. This RFC is orthogonal — RFC 007 controls *which* variables expand, while RFC 008 controls *whether* expansion happens at all for a given `$` character. Both fixes improve the user experience independently.

---

## Testing Strategy

### `$$` Escape Tests

```go
// Basic: $$ → $
{"input": "$$HOME",       "expected": "$HOME"}
{"input": "Price: $$9.99", "expected": "Price: $9.99"}
{"input": "p@ss$$word",   "expected": "p@ss$word"}

// Double escape: $$$$ → $$
{"input": "$$$$",         "expected": "$$"}

// Mixed: $$ literal + real expansion
{"input": "$$literal and ${REAL}", "expected": "$literal and <value>"}

// $$ inside braces
{"input": "$${not_a_var}", "expected": "${not_a_var}"}

// $$ does not interfere with valid variables
{"input": "$HOME",        "expected": "<value of HOME>"}
{"input": "${HOME}",      "expected": "<value of HOME>"}
```

### Single-Quote Stripping Tests

```go
// Quotes stripped, expansion suppressed
{"input": "'$HOME'",      "expected": "$HOME"}
{"input": "'${VAR}'",     "expected": "${VAR}"}

// Quotes only stripped around variable references
{"input": "'hello'",      "expected": "'hello'"}  // no variable, no stripping

// Mixed with real expansion
{"input": "'$LITERAL' and ${REAL}", "expected": "$LITERAL and <value>"}
```

### Edge Cases to Verify

- `$$` at end of string: `"cost$$"` → `"cost$"`
- `$$` adjacent to real variable: `"$$$HOME"` → `$` + value of HOME
- `$$` in command substitution context: `` `echo $$` `` — should produce literal `$` before backtick evaluation
- `$$` in single-quoted context: `'$$HOME'` — quote stripping produces `$$HOME`, then `$$` collapses to `$HOME`? Or should `$$` processing happen first? (Order matters — document the chosen precedence.)
- Empty string and strings with no `$`: unchanged

