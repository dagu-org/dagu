# RFC 019: Multi-Soul SOUL.md System

## Goal

Replace the hardcoded agent personality with a configurable multi-soul system. Users can create, manage, and switch between different agent personalities via `SOUL.md` files — stored alongside DAGs and selectable from the agent chat modal.

---

## Scope

| In Scope | Out of Scope |
|----------|--------------|
| SOUL.md file format with YAML frontmatter + markdown body | Per-user soul preferences |
| CRUD API for soul management | Soul hot-reloading mid-session |
| Soul selection via settings endpoint | Soul content validation/linting |
| Two-layer system prompt (soul + embedded safety rules) | Soul marketplace/sharing |
| Default soul seeded on first startup | |
| Git sync integration for soul files | |

---

## Motivation

The Dagu agent has a single hardcoded personality compiled into the binary. Changing the agent's name, tone, or priorities requires modifying source and redeploying. This is limiting because:

1. **One size doesn't fit all** — different teams have different communication preferences (concise vs. verbose, formal vs. casual).
2. **No customization path** — administrators cannot tailor the agent's identity without source changes.
3. **Personality is invisible** — users cannot see or understand what drives the agent's communication style.

### Use Cases

- A **platform team** uses a "concise-ops" soul that gives minimal, action-oriented responses.
- A **training environment** uses a "verbose-teacher" soul that explains each step in detail.
- An **enterprise deployment** uses a branded soul with company-specific terminology and policies.

---

## Solution

### Two-Layer System Prompt

The system prompt is split into two layers:

| Layer | Ownership | Content |
|-------|-----------|---------|
| **SOUL.md** | User-configurable, per-soul | Identity, priorities, communication style, custom rules |
| **system_prompt.txt** | System-managed, embedded | Environment, safety rules, tools, workflows, memory, schema |

The identity block in the embedded system prompt becomes a template variable populated from the selected soul at runtime. System safety rules remain embedded and non-removable.

### Soul File Format

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

### Directory Structure

```
{DAGsDir}/souls/
├── default.md
├── concise-ops.md
├── verbose-teacher.md
└── .examples-created          # one-time seed marker
```

Flat file layout: each soul is a single `{soul-id}.md` file. The soul ID is the filename without extension (e.g., `concise-ops.md` has ID `concise-ops`). ID validation enforces `[a-z0-9]+(-[a-z0-9]+)*` pattern (lowercase alphanumeric + hyphens, max 128 chars).

### Soul Selection

The selected soul ID is stored in the agent configuration. When empty, falls back to the `"default"` soul. Selection happens via the existing config endpoint:

- `PATCH /settings/agent` with `{ "selectedSoulId": "concise-ops" }`

At session creation time, the selected soul is loaded and its content is injected into the system prompt.

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/settings/agent/souls` | List souls (paginated, searchable) |
| POST | `/settings/agent/souls` | Create soul |
| GET | `/settings/agent/souls/{soulId}` | Get soul |
| PATCH | `/settings/agent/souls/{soulId}` | Update soul |
| DELETE | `/settings/agent/souls/{soulId}` | Delete soul |

### Default Soul

Ships embedded in the binary and is seeded on first startup (via `.examples-created` marker file):

- **Name:** Dagu Assistant
- **Content:** Generic workflow automation identity with safety-first priorities and concise communication style

### Git Sync Integration

Souls participate in Git sync alongside DAGs, memory, and skills:

- A new `soul` kind in the sync state tracker
- Soul files scanned from `{DAGsDir}/souls/` directory
- Full conflict detection, publish, and discard support

### Safety Boundary

System safety rules (security, correctness, data hygiene, UI flow) stay in the embedded system prompt — always present, non-removable. Users can define identity, priorities, and communication style in SOUL.md but cannot override system guardrails.

---

## Data Model

### Soul Entity

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ID` | string | — | Derived from filename; slug format (`[a-z0-9]+(-[a-z0-9]+)*`, max 128 chars) |
| `Name` | string | — | Human-readable display name (from YAML frontmatter) |
| `Description` | string | `""` | Optional description (from YAML frontmatter) |
| `Content` | string | — | Markdown body injected as the agent identity in the system prompt |

### Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `agent.selectedSoulId` | string | `""` | ID of the active soul; empty falls back to `"default"` |

A store interface provides CRUD operations for souls with pagination and search support.

---

## Edge Cases & Tradeoffs

| Chosen | Considered | Why |
|--------|------------|-----|
| Two-layer system prompt | Single configurable prompt | Safety rules must remain non-removable; separating identity from guardrails ensures users cannot override security constraints |
| YAML frontmatter + markdown | Pure YAML | Consistent with skill file pattern; markdown is natural for prose-heavy identity content |
| Flat directory structure | Nested/categorized directories | Simpler; souls are expected to be few enough that hierarchy is unnecessary |
| Slug-based ID from filename | UUID or database-generated ID | Human-readable, filesystem-friendly, consistent with existing DAG ID conventions |
| Embedded default soul | No default / user must create first | Ensures a working agent experience out of the box without any configuration |
