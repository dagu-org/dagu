---
id: "019"
title: "Multi-Soul SOUL.md System"
status: implemented
---

# RFC 019: Multi-Soul SOUL.md System

## Summary

Replace the hardcoded agent personality with a configurable multi-soul system. Users can create, manage, and switch between different agent personalities via `SOUL.md` files — stored alongside DAGs and selectable from the agent chat modal.

---

## Motivation

The Dagu agent has a single hardcoded personality compiled into the binary (`internal/agent/system_prompt.txt`). Changing the agent's name, tone, or priorities requires modifying Go source and redeploying. This is limiting because:

1. **One size doesn't fit all** — different teams have different communication preferences (concise vs. verbose, formal vs. casual).
2. **No customization path** — administrators cannot tailor the agent's identity without source changes.
3. **Personality is invisible** — users cannot see or understand what drives the agent's communication style.

### Use Cases

- A **platform team** uses a "concise-ops" soul that gives minimal, action-oriented responses.
- A **training environment** uses a "verbose-teacher" soul that explains each step in detail.
- An **enterprise deployment** uses a branded soul with company-specific terminology and policies.

---

## Architecture: Two-Layer System Prompt

The system prompt is split into two layers:

| Layer | Ownership | Content |
|-------|-----------|---------|
| **SOUL.md** | User-configurable, per-soul | Identity, priorities, communication style, custom rules |
| **system_prompt.txt** | System-managed, embedded | Environment, safety rules, tools, workflows, memory, schema |

The `<identity>` block in `system_prompt.txt` becomes a template variable `{{.SoulContent}}`, populated from the selected soul at runtime. System safety rules in `<rules>` remain embedded and non-removable.

---

## Soul File Format

YAML frontmatter + Markdown body (consistent with the skill file pattern):

```markdown
---
name: Dagu Assistant
description: General-purpose workflow automation assistant
---

# Identity

You are Dagu Assistant, an AI assistant specialized in workflow automation...

# Priorities

1. Safety: avoid unintended side effects; confirm before executing.
...

# Communication Style

- Be concise and professional.
...
```

---

## Directory Structure

```
{DAGsDir}/souls/
├── default.md
├── concise-ops.md
├── verbose-teacher.md
└── .examples-created          # one-time seed marker
```

Flat file layout: each soul is a single `{soul-id}.md` file. The soul ID is the filename without extension (e.g., `concise-ops.md` -> ID `concise-ops`).

**ID validation:** Reuses `validateSlugID()` — enforces `[a-z0-9]+(-[a-z0-9]+)*` pattern (lowercase alphanumeric + hyphens, max 128 chars).

---

## Domain Entity

```go
type Soul struct {
    ID          string
    Name        string
    Description string
    Content     string   // markdown body -- injected as identity
}

type SoulStore interface {
    Create(ctx, soul) error
    GetByID(ctx, id) (*Soul, error)
    List(ctx) ([]*Soul, error)
    Search(ctx, opts) (*PaginatedResult[SoulMetadata], error)
    Update(ctx, soul) error
    Delete(ctx, id) error
}
```

---

## Soul Selection

The selected soul ID is stored in `agent.Config.SelectedSoulID`. When empty, falls back to the `"default"` soul.

Selection happens via the existing config endpoint:
- `PATCH /settings/agent` with `{ "selectedSoulId": "concise-ops" }`

At session creation time, the selected soul is loaded and its `Content` is injected into the system prompt.

---

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/settings/agent/souls` | List souls (paginated, searchable) |
| POST | `/settings/agent/souls` | Create soul |
| GET | `/settings/agent/souls/{soulId}` | Get soul |
| PATCH | `/settings/agent/souls/{soulId}` | Update soul |
| DELETE | `/settings/agent/souls/{soulId}` | Delete soul |

---

## Default Soul

Ships embedded in the binary and is seeded on first startup (via `.examples-created` marker file):

- **Name:** Dagu Assistant
- **Content:** Generic workflow automation identity with safety-first priorities and concise communication style

---

## Git Sync Integration

Souls participate in Git sync alongside DAGs, memory, and skills:

- New `DAGKindSoul` kind in the sync state tracker
- Soul files scanned from `{DAGsDir}/souls/` directory
- Full conflict detection, publish, and discard support
- `soul` added to the `SyncItemKind` enum

---

## Safety Boundary

- System `<rules>` section (safety, security, correctness, data hygiene, UI flow) stays in the embedded `system_prompt.txt` — always present, non-removable.
- Users can define identity, priorities, and communication style in SOUL.md but cannot override system guardrails.

---

## Out of Scope (Future Work)

- Per-user soul preferences (different users see different default souls)
- Per-DAG soul overrides in agent step configuration
- Soul hot-reloading mid-session
- Soul content validation/linting
- Soul marketplace/sharing
