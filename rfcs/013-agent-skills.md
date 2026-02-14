---
id: "013"
title: "Agent Skills"
status: draft
---

# RFC 013: Agent Skills

## Summary

User-defined and built-in "skills" that extend the Tsumugi agent's expertise beyond core DAG management — enabling domain-specific knowledge, workflow templates, custom tools, and composable personas without modifying source code.

---

## Motivation

Currently:

1. **Single-persona agent** — Tsumugi only knows about DAG management. Users working in specific domains (Kubernetes, ETL pipelines, CI/CD, data science) must provide context manually every session.
2. **No extensibility path** — adding new capabilities requires modifying Go source code and redeploying Dagu.
3. **Knowledge is unstructured** — the memory system captures free-form notes, but there is no way to provide curated, reusable domain expertise that the agent can reliably apply.
4. **Template workflows are manual** — users copy-paste YAML snippets from documentation. The agent cannot draw on a library of vetted, domain-specific templates.
5. **Tool set is fixed** — the 8 built-in tools serve general DAG operations well, but domain-specific actions (e.g., "dry-run a Helm chart", "validate a dbt model") require composing bash commands from scratch every time.

### Use Cases

- A **platform team** installs a "Kubernetes" skill so Tsumugi can generate Kubernetes-aware DAGs with best practices for pod scheduling, resource limits, and health checks.
- A **data engineer** enables an "ETL Best Practices" skill that teaches Tsumugi about idempotency patterns, incremental loads, and data validation steps.
- An **SRE team** authors a custom "Incident Response" skill with runbook templates and on-call escalation workflows.
- A **solo developer** composes "Docker" + "GitHub Actions" skills to get an agent that understands both container builds and CI/CD pipeline patterns.

---

## What Is a Skill

A skill is a self-contained package of domain expertise that the agent loads on demand. Each skill provides some combination of:

| Component | Purpose | Required |
|-----------|---------|----------|
| **Knowledge** | Domain-specific instructions, best practices, and reference material injected into the agent's context | Yes |
| **Templates** | Reusable DAG workflow patterns the agent can instantiate and customize | No |
| **Tools** | Custom tool definitions that extend the agent's action repertoire | No |
| **Metadata** | Name, description, version, author, tags, icon, and compatibility info | Yes |

A skill with only knowledge and metadata is the simplest form — a "knowledge pack." A skill that also registers tools is more powerful but requires more trust.

### Skills vs. Memory

| Aspect | Memory | Skills |
|--------|--------|--------|
| **Source** | Learned by the agent during conversations | Authored deliberately by humans |
| **Structure** | Free-form markdown (MEMORY.md) | Structured package with defined components |
| **Scope** | Per-user or per-DAG observations | Domain expertise applicable to any user |
| **Curation** | Informal, grows organically | Versioned, intentionally maintained |
| **Activation** | Always loaded into context | Enabled/disabled per session or per user |
| **Size** | Limited to ~200 lines | Can include substantial reference material, templates, and tool definitions |

Skills complement memory. Memory captures "what happened" and user preferences. Skills capture "how to do things" in a particular domain.

---

## User Experience

### Discovering Skills

**Skills Library page**: A dedicated page accessible from the Admin section of the sidebar. Shows all available skills in a searchable, filterable grid.

| View Element | Description |
|--------------|-------------|
| **Search bar** | Filter by name, description, or tags |
| **Category filter** | Built-in, Custom |
| **Status filter** | Enabled, Disabled, All |
| **Skill cards** | Each card shows: icon, name, short description, author, version, enabled/disabled toggle, tag badges |

Clicking a card opens a detail panel showing the full description, component list (knowledge, templates, tools), and configuration options.

**In-chat discovery**: Users can ask Tsumugi "what skills are available?" or "do you have a skill for Kubernetes?" The agent lists matching skills and can enable them on request (if the user has permission).

### Enabling and Disabling Skills

Skills can be activated at two levels:

| Level | Who Controls | Behavior |
|-------|-------------|----------|
| **User defaults** | Each user (via Skills Library or chat) | Skills automatically active in all new sessions for that user |
| **Per-session** | Any user (via chat) | Override defaults for the current session only; does not persist |

Enabling a skill in chat:

```
User: "Enable the Kubernetes skill for this session"
Tsumugi: "Kubernetes skill enabled. I now have access to K8s-specific
          templates and best practices. How can I help?"
```

Disabling follows the same pattern. The agent acknowledges the change and explains what capabilities were added or removed.

**Admin controls**: Admins can set instance-wide defaults (skills enabled for all users by default) and restrict which skills are available to non-admin users.

### Authoring Custom Skills

Users author skills as files placed in a designated skills directory within the Dagu data directory. The format is deliberately simple — a single file for straightforward skills, or a directory for skills with templates and tool definitions.

**Single-file skill** (knowledge-only):

```
skills/
  kubernetes.yaml
```

**Multi-file skill** (with templates and tools):

```
skills/
  incident-response/
    skill.yaml           # Metadata + knowledge + tool definitions
    templates/
      rollback.yaml      # DAG template
      healthcheck.yaml   # DAG template
```

The UI provides a basic skill editor for creating and editing single-file skills without leaving the browser. For multi-file skills, users manage the files directly (or through the agent's own `patch` tool).

**Validation**: When a skill file is saved, it is validated immediately. Invalid skills are flagged in the UI with clear error messages. The agent refuses to load invalid skills.

### Sharing Skills

Skills are plain files. Sharing works through existing mechanisms:

| Method | How |
|--------|-----|
| **Git** | Skills directory is included in version control. Teams share skills through their existing git workflow. |
| **Copy/paste** | Single-file skills can be shared as YAML snippets. |

### Interacting with a Skill-Enhanced Agent

When skills are active, the agent's behavior changes in observable ways:

1. **Domain knowledge** — the agent has access to domain-specific instructions and references them naturally in conversation.
2. **Template suggestions** — when the user's request matches a skill template, the agent proposes it: *"I have a template for Kubernetes CronJob DAGs. Would you like me to start from that?"*
3. **Additional tools** — skill-registered tools are available alongside the built-in tools. The agent uses them when appropriate.
4. **Skill attribution** — when the agent uses knowledge or templates from a skill, it mentions this: *"Based on the ETL Best Practices skill, I recommend adding a data validation step after the load."*

The user does not need to remember which skills are active. The agent seamlessly incorporates skill knowledge into its responses.

---

## Skill Types

| Type | Source | Management | Trust Level |
|------|--------|-----------|-------------|
| **Built-in** | Shipped with Dagu | Updated with Dagu releases; can be disabled but not deleted | High — vetted by maintainers |
| **Custom** | Created by users/teams | Full CRUD via UI or filesystem | Varies — depends on author |

**Built-in skills** ship as embedded resources within the Dagu binary. They cover common domains and serve as examples for custom skill authoring.

**Custom skills** are the primary extensibility mechanism. Users with appropriate roles can create, edit, and delete custom skills.

---

## Skill Composition

Multiple skills can be active simultaneously. This raises questions about conflicts and context budget.

### Composition Rules

| Concern | Resolution |
|---------|------------|
| **Knowledge merging** | Knowledge from all active skills is injected into the agent's context, each clearly delimited with the skill name. The agent sees all knowledge and synthesizes across skills. |
| **Template namespacing** | Templates are prefixed with their skill name to avoid collisions: `kubernetes/cronjob`, `etl/incremental-load`. |
| **Tool conflicts** | If two skills register a tool with the same name, the more recently enabled skill wins. A warning is shown in the UI and logged. |
| **Context budget** | Each skill's knowledge consumes context window tokens. A configurable total budget prevents unbounded context growth. If active skills exceed the budget, the user is warned and asked to disable some skills. Skills are loaded in priority order. |

### Priority and Ordering

The Skills Library page allows drag-to-reorder for user defaults, determining priority for conflict resolution. Per-session overrides append to the end of the priority list.

---

## Relationship to Existing Features

| Feature | Relationship |
|---------|-------------|
| **Memory** | Complementary. Memory is learned; skills are authored. Both inject into context. Memory is always loaded; skills are opt-in. |
| **Tool Policy** | Skill-registered tools are subject to the same policy engine. If a policy blocks `bash`, a skill that registers a bash-based tool is also blocked. Custom tools go through the existing hook pipeline. |
| **Audit Logging** | All tool executions from skill-registered tools are audited identically to built-in tools. Audit entries include the originating skill name for traceability. |
| **Authorization** | Skill management (create/edit/delete) requires appropriate role. Enabling/disabling for personal use requires any authenticated role. Skill-registered tools respect role-based permissions. |
| **Namespaces** (RFC 009) | When namespaces are implemented, skills can be scoped per-namespace. Built-in skills are available in all namespaces. Custom skills can be namespace-local or global. |
| **Safe Mode** | Skill-registered tools that execute commands are subject to the same safe mode approval flow as the built-in `bash` tool. |

---

## Examples

### Example 1: Kubernetes Skill (Built-in)

A knowledge-and-template skill that teaches the agent about Kubernetes-aware DAG patterns.

**Provides**: Knowledge about K8s executor configuration (resource limits, node selectors, tolerations, service accounts). Templates for common patterns: CronJob migration, rolling deployment, namespace provisioning.

```
User: "Create a DAG that deploys my app to staging"
Tsumugi: "I'll use the Kubernetes deployment template. I need a few details:
          - Container image and tag
          - Target namespace
          - Resource limits (or use defaults: 256Mi/500m)
          ..."
```

### Example 2: ETL Best Practices Skill (Custom, knowledge-only)

Authored by a data engineering team. Provides rules for idempotent DAG design, data validation step patterns, naming conventions for ETL DAGs, and guidance on using PostgreSQL and S3 executors effectively.

### Example 3: Incident Response Skill (Custom, with tools)

Authored by an SRE team. Provides knowledge about the team's incident process, templates for rollback/healthcheck DAGs, and custom tools: `check_service_health` (hits internal health endpoint) and `page_oncall` (triggers PagerDuty).

### Example 4: CI/CD Skill (Built-in)

Provides knowledge about CI/CD pipeline patterns expressible as DAGs, templates for build-test-deploy pipelines, and best practices for artifact passing between stages.

---

## Out of Scope

1. **Community skill marketplace or index** — sharing through git is sufficient for the initial release.
2. **Skill versioning and dependency resolution** — skills are standalone; no version constraint solver.
3. **Runtime sandboxing of skill tools** — skill tools run with the same privileges as built-in tools. The existing policy engine provides the safety layer.
4. **Skill-specific UI components** — skills cannot inject custom UI elements. They operate through the chat interface and system prompt.
5. **Automatic skill recommendation** — the system does not analyze behavior to suggest skills. Users discover and enable skills manually.
6. **Hot-reloading** — changing a skill file requires starting a new session for changes to take effect.

---

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Context window exhaustion | Configurable total skill knowledge budget. Warning when exceeded. Priority-ordered loading with overflow truncation. |
| Conflicting advice from multiple skills | Skill attribution in responses. Priority ordering. Admin curation of available skills. |
| Malicious custom tools | Policy engine, audit logging, and hook pipeline apply to all skill tools. Admin approval required for skills that register tools. |
| Prompt injection via skill content | Skill knowledge is injected into a clearly delimited section of context, treated as reference material. |
| Skill sprawl | Tags and categories for organization. Admin can disable or hide irrelevant skills. |
| Breaking changes to skill format | Schema version field in skills. Validation against declared version with migration guidance. |
