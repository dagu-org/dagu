---
id: "008"
title: "Dollar Sign Escaping in v1 Variable Expansion"
status: draft
---

# RFC 008: Dollar Sign Escaping in v1 Variable Expansion

## Summary

Add a single escape mechanism to the v1 variable expansion system so that users can include literal `$` characters in DAG field values:

1. **`\$` → literal `$`**: A backslash immediately before `$` produces a single literal `$` in the output.

This escape is applied only when Dagu is the final evaluator (non-shell executors/config fields). Shell-executed commands are left untouched so shell semantics remain intact.
Single-quoted `$VAR`/`${VAR}` patterns remain verbatim and keep their quotes; no stripping is performed.

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

Users have tried every reasonable escape convention — none produce a clean literal `$` without side effects:

| Attempt | Input | Expected | Actual |
|---------|-------|----------|--------|
| Backslash | `\$HOME` | `$HOME` | `\` + expanded value |
| Double dollar | `$$HOME` | `$HOME` | `$$HOME` (no escape) or `$<value>` |
| Single quotes | `'$HOME'` | `'$HOME'` | `'$HOME'` (works, but quotes remain) |
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

The `extractVarKey` function (`resolve.go:115-123`) handles single-quoted matches by returning `false` to skip expansion — but the original match (including surrounding quotes) is preserved verbatim, leaving the quotes in the output. This behavior is unchanged.

---

## Proposal

### Fix 1: `\$` Produces Literal `$`

A backslash immediately preceding `$` is converted to a single literal `$` before variable expansion runs. This is a pre-processing step inserted at the beginning of the pipeline, before any regex matching occurs.

**Convention alignment:** This matches standard shell escaping and avoids introducing a new escape syntax.

**Examples:**

| Input | Output |
|-------|--------|
| `\$HOME` | `$HOME` (literal, not expanded) |
| `\${userId}` | `${userId}` (literal braces preserved) |
| `Price: \$9.99` | `Price: $9.99` |
| `p@ss\$word` | `p@ss$word` |
| `\$\$` | `$$` (escape both dollars) |
| `$HOME` | value of HOME (unchanged behavior) |

### Single Quotes (No Change)

Single-quoted variables like `'$VAR'` and `'${VAR}'` continue to suppress Dagu expansion and remain preserved verbatim in the output. This keeps quoting behavior explicit and avoids surprising transformations.

---

## Behavior (Proposed)

| Input | Output | Notes |
|-------|--------|-------|
| `$HOME` | Expanded to env value | No change |
| `\$HOME` | `$HOME` | Backslash escape |
| `\${VAR}` | `${VAR}` | Backslash escape |
| `\\$HOME` | `\\` + expanded value | Even backslashes do not escape `$` |
| `'$HOME'` | `'$HOME'` (with quotes) | No change |
| `\$\$` | `$$` | Escape both dollars |

`\$` escape is applied only in non-shell evaluation contexts. Shell-executed commands are left unchanged.

---

## Relationship to Other RFCs

### RFC 005 — Variable Expansion Syntax Refactoring (draft)

RFC 005 proposes the `${{ context.VAR }}` syntax which eliminates `$VAR`/`${VAR}` ambiguity entirely. Under RFC 005, `$VAR` and `${VAR}` pass through to the shell unchanged, and only `${{ ... }}` is expanded by Dagu.

This RFC (008) is complementary:
- **RFC 008** fixes the immediate inability to escape `$` in v1 syntax.
- **RFC 005** makes escaping unnecessary long-term by using a distinct syntax.
- `\$` remains useful for literal `$` in non-shell contexts (e.g., `\$9.99` in an HTTP body).

### RFC 006 — Variable Expansion Syntax v1 (implemented)

RFC 006 documents the current v1 behavior including the single-quote escape mechanism. This RFC amends RFC 006's "Escape Mechanisms" section:

- **Before (RFC 006):** Single-quoted variables are "preserved literally and not expanded".
- **After (this RFC):** Single-quoted variables are still preserved literally, and `\$` is available as an additional escape in non-shell contexts.

### RFC 007 — OS Environment Variable Expansion Rules (implemented)

RFC 007 restricts which variables are expanded for non-shell executors. This RFC is orthogonal — RFC 007 controls *which* variables expand, while RFC 008 controls *whether* expansion happens at all for a given `$` character. Both fixes improve the user experience independently.

---

## Testing Strategy

### `\$` Escape Tests

```go
// Basic: \$ → $
{"input": "\\$HOME",       "expected": "$HOME"}
{"input": "Price: \\${PRICE}", "expected": "Price: ${PRICE}"}
{"input": "Price: \\$9.99", "expected": "Price: $9.99"}
{"input": "p@ss\\$word",   "expected": "p@ss$word"}

// Escape both dollars: \$\$ → $$
{"input": "\\$\\$", "expected": "$$"}

// Mixed: literal + real expansion
{"input": "\\$literal and ${REAL}", "expected": "$literal and <value>"}

// \$ does not interfere with valid variables
{"input": "$HOME",   "expected": "<value of HOME>"}
{"input": "${HOME}", "expected": "<value of HOME>"}
```

### Edge Cases to Verify

- `\$` at end of string: `"cost\\$"` → `"cost$"`
- Odd vs even backslashes: `"\\\\$HOME"` → `"\\" + <value>`; `"\\\\\\$HOME"` → `"\\$HOME"`
- `\$` adjacent to real variable: `"\\$HOME and $HOME"` → `"$HOME and <value>"`
- `\$` in command substitution context: `` `echo \\$` `` — should produce literal `$` before backtick evaluation
- `\$` in single-quoted context: `"'\\$HOME'"` — unchanged (quotes preserved)
- Empty string and strings with no `$`: unchanged
