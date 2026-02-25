# RFC 008: Dollar Sign Escaping in v1 Variable Expansion

## Goal

Add `\$` as an escape mechanism to the v1 variable expansion system so that users can include literal `$` characters in DAG field values. This escape is applied only when Dagu is the final evaluator (non-shell executors and config fields). Shell-executed commands are left untouched so shell semantics remain intact.

This is a targeted fix for issue [#1628](https://github.com/dagu-org/dagu/issues/1628).

## Scope

| In scope | Out of scope |
|----------|-------------|
| `\$` escape sequence producing a literal `$` | Shell command escaping (shell handles its own `$`) |
| Pre-processing step before variable expansion | New variable syntax (see RFC 005) |
| Non-shell evaluation contexts only | Quote stripping for single-quoted `'$VAR'` patterns |
| Backslash-dollar conversion in the expansion pipeline | Changes to the variable-matching regex |

---

## Motivation

Users cannot include a literal `$` in any field that undergoes variable expansion. This breaks real-world patterns involving GraphQL queries, price strings, passwords, and regex patterns.

**GraphQL query (broken):**

```yaml
steps:
  - name: query
    type: http
    config:
      url: https://api.example.com/graphql
      body: '{"query": "{ user(id: $userId) { name } }"}'
```

Dagu attempts to expand `$userId` as a variable reference. If the user happens to have an env var named `userId`, it silently substitutes the wrong value.

**Price string (broken):**

```yaml
env:
  PRICE: "$9.99"
```

Dagu interprets `$9` as positional parameter 9. The result is unpredictable.

**Password with dollar sign (broken):**

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

---

## Solution

### Escape Mechanism: `\$` Produces Literal `$`

A backslash immediately preceding `$` is converted to a single literal `$` before variable expansion runs. This is a pre-processing step inserted at the beginning of the expansion pipeline, before any variable matching occurs.

**Convention alignment:** This matches standard POSIX shell escaping and avoids introducing a new escape syntax.

### Processing Pipeline

The escape conversion is inserted as the first stage of the expansion pipeline, so that escaped dollar signs are neutralized before any variable matching can capture them.

```mermaid
flowchart LR
    A["Raw input string"] --> B["1. Escape pre-processing\n\\$ → placeholder"]
    B --> C["2. Quoted-refs resolution\n\"${FOO.bar}\" patterns"]
    C --> D["3. Variable expansion\n$VAR, ${VAR}"]
    D --> E["4. Command substitution\nbacktick expressions"]
    E --> F["5. Shell expansion\nPOSIX-style"]
    F --> G["6. Placeholder → literal $"]
    G --> H["Final output"]
```

The pre-processing step replaces `\$` with an internal placeholder that no other pipeline stage recognizes. After all expansion is complete, the placeholder is converted to a literal `$`.

### Behavior

| Input | Output | Notes |
|-------|--------|-------|
| `$HOME` | Expanded to env value | No change |
| `\$HOME` | `$HOME` | Backslash escape |
| `\${VAR}` | `${VAR}` | Backslash escape with braces |
| `\\$HOME` | `\\` + expanded value | Even backslashes do not escape `$` |
| `'$HOME'` | `'$HOME'` (with quotes) | No change |
| `\$\$` | `$$` | Escape both dollars |

The `\$` escape is applied only in non-shell evaluation contexts. Shell-executed commands are left unchanged so that shell escaping rules apply normally.

### Single Quotes (No Change)

Single-quoted variables like `'$VAR'` and `'${VAR}'` continue to suppress Dagu expansion and remain preserved verbatim in the output, including the surrounding quotes. This behavior is unchanged.

### Examples

**GraphQL query — fixed with escape:**

```yaml
steps:
  - name: query
    type: http
    config:
      url: https://api.example.com/graphql
      body: '{"query": "{ user(id: \$userId) { name } }"}'
```

Output body: `{"query": "{ user(id: $userId) { name } }"}`

**Price string — fixed with escape:**

```yaml
env:
  PRICE: "\$9.99"
```

The value of `PRICE` is the literal string `$9.99`.

**Mixed literal and expanded:**

```yaml
env:
  CURRENCY: USD

steps:
  - name: report
    type: http
    config:
      url: https://api.example.com/report
      body: "Total: \$42.00 ${CURRENCY}"
```

Output body: `Total: $42.00 USD`

---

## Data Model

No new stored state. This is a syntax processing feature that adds an escape sequence to the expansion pipeline. No persistent fields, configuration options, or schema changes are introduced.

---

## Edge Cases & Tradeoffs

| Chosen | Considered | Why |
|--------|-----------|-----|
| `\$` escape syntax | `$$` escape syntax | `\$` matches POSIX convention; `$$` conflicts with potential future uses and is less intuitive |
| Pre-processing step (placeholder-based) | Modify the variable-matching regex | Pre-processing is simpler, keeps the regex untouched, and avoids subtle matching bugs |
| Non-shell contexts only | All contexts including shell commands | Shell already handles `\$` natively; double-processing would break shell escaping semantics |
| Preserve single-quote behavior unchanged | Strip quotes from `'$VAR'` output | Quote stripping would be a separate behavioral change with its own edge cases; out of scope |
| Internal placeholder during pipeline | Direct string replacement | A placeholder avoids the risk of later pipeline stages re-interpreting the literal `$` as a variable start |

---

## Relationship to Other RFCs

**RFC 005** — Proposes `${{ context.VAR }}` syntax that eliminates `$VAR`/`${VAR}` ambiguity entirely. This RFC (008) fixes the immediate inability to escape `$` in v1 syntax. The `\$` escape remains useful long-term for literal dollar signs in non-shell contexts (e.g., `\$9.99` in an HTTP body).

**RFC 006** — This RFC amends RFC 006's "Escape Mechanisms" section by adding `\$` as an additional escape alongside the existing single-quote mechanism.

**RFC 007** — Controls which variables are expanded (OS env fallback removal). This RFC controls whether expansion happens at all for a given `$` character. The two are orthogonal and improve user experience independently.
