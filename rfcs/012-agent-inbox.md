---
id: "012"
title: "Agent In-box"
status: draft
---

# RFC 012: Agent In-box

## Summary

A persistent, per-user notification system ("In-box") that delivers messages from DAG runs, HITL approvals, agents, and system events. Messages appear in a sidebar bell icon, a quick preview panel, and a dedicated `/inbox` page with filtering, actions, and real-time delivery via SSE.

---

## 1. Message Schema & Categories

### Schema

| Field | Type | Description |
|-------|------|-------------|
| `id` | UUID | Unique identifier |
| `timestamp` | UTC time | When the message was created |
| `category` | enum | See categories below |
| `priority` | enum | `critical`, `normal`, `low` |
| `title` | string | One-line summary (max 120 chars) |
| `body` | string | Markdown content (max 4KB) |
| `sender` | SenderRef | Who/what sent it (type + name + optional DAG/step/user refs) |
| `recipients` | []RecipientRef | Who should see it (`user:<id>`, `role:<role>`, `role:*`) |
| `status` | enum | `unread`, `read`, `archived` (per-user) |
| `deepLink` | string? | URL path to the relevant page |
| `actions` | []Action | Inline action buttons (approve, retry, dismiss, open link) |
| `metadata` | map | Arbitrary key-value pairs |
| `dagName` | string? | Associated DAG (for grouping/filtering) |
| `dagRunID` | string? | Associated run |
| `stepName` | string? | Associated step |
| `expiresAt` | time? | Auto-archive TTL |

### Categories

| Category | Description | Default Priority |
|----------|-------------|-----------------|
| `dag.failure` | A DAG run failed | `critical` |
| `dag.success` | A DAG run succeeded | `low` (off by default) |
| `dag.partial_success` | Partial success | `normal` |
| `approval.requested` | HITL step waiting for approval | `critical` |
| `approval.completed` | HITL approved/rejected | `normal` |
| `agent.message` | Agent explicitly sent a message | `normal` |
| `agent.completed` | Agent-type DAG finished | `low` |
| `system.announcement` | Admin broadcast | `normal` |
| `dag.custom` | DAG step sent via `notify` executor | `normal` |

### Priority Levels

| Priority | Badge | Browser Notification | Tab Title |
|----------|-------|---------------------|-----------|
| `critical` | Red | Yes (if permitted) | `(N) Dagu` |
| `normal` | Blue | No | `(N) Dagu` |
| `low` | None | No | No change |

### Message Senders

| Sender Type | Example |
|-------------|---------|
| `system` | Auto-generated on DAG failure, HITL wait |
| `dag` | A `notify` step in a YAML workflow |
| `agent` | An LLM agent using a `send_message` tool |
| `user` | Admin broadcast announcement |

### Message Recipients

| Target | Description |
|--------|-------------|
| `user:<id>` | Specific user |
| `role:<role>` | All users with that role |
| `role:*` | Broadcast to all authenticated users |

Default targeting:
- DAG failure → `role:operator` and above
- HITL approval → `role:operator` and above
- Agent completion → user who started the run (or `role:operator`)
- System announcement → `role:*`

Read/archive status is **per-user** — one admin reading a role-targeted message does not affect another admin's unread count.

### Lifecycle

```
created → unread → read → archived
                \→ archived (direct dismiss)
```

Default TTL: 30 days (`low`), 90 days (`normal`), no auto-expiry (`critical`).

---

## 2. UX Design — Notification Entry Points

### 2.1 Bell Icon & Badge (Sidebar Footer)

**Placement:** In the sidebar footer area, between the Agent button and the theme toggle. This is where persistent utility controls live in the GCP-style sidebar.

| Sidebar State | Appearance |
|---------------|------------|
| Collapsed (56px) | Bell icon + red dot (critical) or blue dot (normal) |
| Expanded (240px) | `SidebarButton` showing "In-box" + count badge pill |

Badge rules:
- Red pill with count → unread critical messages exist
- Blue pill with count → only normal unread messages
- No badge → only `low` priority or no unread messages

**Click:** Opens the quick preview panel.

### 2.2 Quick Preview Panel (Right Slide-Over)

A **320px slide-over panel from the right edge** with slight dim overlay. Not a dropdown — provides room for content while keeping the current page visible. Matches the pattern used by GitHub Notifications and Linear.

**Panel structure:**
- **Header (h-9):** "In-box" title, unread count, "Mark all read" link, "View all →" link
- **Message list:** Most recent 20 messages, grouped by time (Today, Yesterday, Earlier, Older)
- **Each message row (h-14):**
  - Left: Category icon (color-coded by priority)
  - Center: Title (single line, truncated), sender info (muted), relative timestamp
  - Right: Primary action button (if available) or dismiss (X) on hover
  - Unread: 3px left-border accent matching priority color (consistent with NavItem active indicator)
- **Empty state:** "No new messages" in muted text. No illustrations.

**Interactions:**
- Click message → navigate to `deepLink` + mark as read → panel auto-closes
- Messages without deepLink → expand inline to show body text
- Keyboard: `↑↓` navigate, `Enter` opens, `Escape` closes, `x` dismisses

### 2.3 Full Inbox Page

**Route:** `/inbox`

**Navigation placement:** In the "System" section of the sidebar, below Dashboard:

```
SYSTEM
  Dashboard
  In-box (3)    ← NEW, visible to all authenticated users
  System Status  (admin only)
```

Uses `Bell` icon from lucide-react. The existing Queues nav item's `Inbox` icon should be changed to `ListOrdered` to avoid visual confusion.

**Layout:** Uses existing `SplitLayout` component. Left panel = message list (40%), right panel = message detail.

**Left panel:**
- Toolbar (h-9): Filter dropdown (All / Unread / Critical / By Category), sort toggle, search input
- Bulk actions bar (h-8, appears on multi-select): "Mark read", "Archive" buttons
- Message rows (h-12): Compact design with checkboxes for multi-select
- Grouping: By date (Today, Yesterday, etc.) by default. By DAG name when DAG filter is active
- Infinite scroll, 50-message pages

**Right panel:**
- Header: Title, sender, timestamp, category badge, priority indicator
- Contextual breadcrumb: `DAG Name > Run ID > Step Name` (each clickable)
- Body: Rendered GitHub Flavored Markdown
- Action buttons: Approve/Reject for HITL, Retry for failures
- Metadata: Collapsible key-value pairs section

### 2.4 Real-Time Delivery

When a message arrives while the user is active (delivered via SSE):

| Priority | Bell Badge | Toast | Browser Notification |
|----------|-----------|-------|---------------------|
| `critical` | Count updates + red pulse (600ms) | Slide-in from top-right, auto-dismiss 5s, shows title + "View" link | Yes (if permission granted) |
| `normal` | Count updates quietly | No | No |
| `low` | Count updates quietly | No | No |

**No sounds.** Browser notifications are sufficient for critical events.

**Tab title:** `(N) Dagu` where N = unread count of critical + normal messages. `low` priority excluded from count to avoid badge fatigue.

### 2.5 Login Experience

When a user logs in with pending messages, a **summary banner** appears at the top of the Dashboard:

**Banner (h-10):**
- Left: Bell icon + "3 unread messages while you were away" (or "1 approval waiting" if HITL approvals pending)
- Right: "View In-box" link
- Dismissible (X button), does not reappear for that session

Intentionally minimal — a nudge, not a blocker.

### 2.6 Mobile

- Bell icon in the mobile header bar
- Quick panel opens as full-screen overlay (not 320px — not enough space)
- Full inbox page is single-column. Tapping a message navigates to a full-page detail view
- Swipe-to-archive on individual messages

---

## 3. Message Content & Actionability

### Inline Actions

| Action Type | Used For | Behavior |
|-------------|----------|----------|
| `approve_hitl` | HITL approval requests | Calls existing approve API, marks read |
| `reject_hitl` | HITL approval requests | Calls existing reject API, marks read |
| `retry_dag_run` | DAG failures | Calls retry API, marks read |
| `open_link` | Generic navigation | Navigate to deep link |
| `dismiss` | Any message | Archive the message |

Actions rendered as buttons in detail view, compact icon-buttons in quick panel. Primary action gets visual emphasis (filled button).

### Deep Links

| Category | Deep Link |
|----------|-----------|
| `dag.failure` | `/dag-runs/{name}/{dagRunId}` |
| `dag.success` | `/dag-runs/{name}/{dagRunId}` |
| `approval.requested` | `/dag-runs/{name}/{dagRunId}` (to the waiting step) |
| `agent.message` | `/dag-runs/{name}/{dagRunId}` |
| `dag.custom` | Configurable in YAML; defaults to DAG run page |

### Rich Content

Message bodies support **GitHub Flavored Markdown** (already used elsewhere in the app via `react-markdown`):
- Code blocks with syntax highlighting (for error messages, stack traces)
- Tables (for structured output summaries)
- Bold/italic, links

No images, charts, or arbitrary HTML. For rich data, deep-link to the appropriate page.

---

## 4. DAG-Side Authoring

### 4.1 New `notify` Executor

Follows the established executor registration pattern (same as `hitl`, `mail`, `docker`, etc. in `internal/runtime/builtin/`):

```yaml
steps:
  - name: alert-on-completion
    type: notify
    config:
      title: "Data pipeline completed"
      body: |
        Records processed: ${RECORD_COUNT}
        Duration: ${DURATION}
      priority: normal
      recipients:
        - role:operator
```

Config schema:
- `title` (required): Supports variable expansion (`${VAR}`, `$1`, etc.)
- `body` (optional): Markdown, supports variable expansion
- `priority` (optional): `critical` | `normal` | `low` (default: `normal`)
- `category` (optional): Default: `dag.custom`
- `recipients` (optional): Default: `role:operator`
- `deepLink` (optional): Custom path (default: auto-generated to current DAG run)
- `actions` (optional): Custom inline actions

### 4.2 Auto-Generated Messages via `notifyOn`

New DAG-level YAML property:

```yaml
notifyOn:
  failure: true       # Default: true
  success: false      # Default: false
  approval: true      # Default: true
  recipients:
    - role:admin
```

Separate from `mailOn` — the two channels are independent. Can be overridden globally in config and per-DAG in YAML.

### 4.3 Agent-Sent Messages

Agent-type DAGs get a new tool: `send_message`:

```
Tool: send_message
Parameters:
  title: string (required)
  body: string (optional, markdown)
  priority: "critical" | "normal" | "low" (default: "normal")
  recipients: string[] (default: user who started the agent)
```

---

## 5. UX Micro-Interactions

### Mark as Read
- Marked read when: user **clicks** on the message (opening it in detail view) or explicitly clicks "Mark as read"
- **NOT** auto-marked by hovering, scrolling past, or opening the quick panel
- Rationale: Explicit marking prevents "I saw it but didn't process it" — matches Linear's behavior

### Keyboard Shortcuts

| Shortcut | Context | Action |
|----------|---------|--------|
| `i` | Any page (not in input) | Toggle quick preview panel |
| `Escape` | Panel or inbox | Close panel / deselect message |
| `j`/`k` or `↑`/`↓` | Message list | Navigate messages |
| `Enter` | Message focused | Open / navigate to deep link |
| `e` | Message focused | Archive |
| `Shift+I` | Message focused | Toggle read/unread |

### Empty States
- Quick panel: "No new messages" in muted text, centered
- Inbox page left panel: "No messages" + "Messages from your workflows, agents, and system events appear here."
- Inbox page right panel: Standard `SplitLayout` empty message

### Notification Preferences (Per User)
Settings accessed via gear icon in inbox toolbar (Radix `Popover` with toggle switches):

```yaml
categories:
  dag.failure:          { enabled: true }
  dag.success:          { enabled: false }
  approval.requested:   { enabled: true }
  agent.message:        { enabled: true }
  system.announcement:  { enabled: true }
browserNotifications: true
quietHours:
  enabled: false
  start: "22:00"
  end: "08:00"
```

### Quiet Hours
When active: no browser notifications, no toasts. Badge still updates. Messages still stored. Purely a UI-layer muting mechanism.

### Browser Tab Title
`(N) Dagu` where N = unread count of `critical` + `normal`. Clears when messages are read or tab regains focus with inbox open.

---

## 6. Information Architecture

### Relationship to Existing Pages

| Page | Relationship |
|------|-------------|
| **Dashboard** | Login banner links to inbox. Dashboard = operational overview; inbox = personal notifications. |
| **DAG Runs** | DAG Run detail page could show a "Messages" section for messages related to that run. Bidirectional navigation. |
| **Audit Logs** | Separate purposes. Audit = compliance/admin. Inbox = personal. Message delivery is an auditable event. |
| **Agent Chat** | Chat = real-time conversational. Inbox = async persistent. Agents can send inbox messages that persist beyond chat sessions. |

### Access Control

All authenticated users can:
- View their own inbox
- Mark as read/archived
- Configure their notification preferences

Admins only:
- Send broadcast announcements
- Configure global notification defaults

Not visible to unauthenticated users.

---

## 7. What This RFC Intentionally Excludes

- Email integration from inbox messages (inbox and `mailOn` remain separate channels)
- Webhook delivery of inbox messages
- Message threading/replies (for conversation, use Agent Chat)
- @mentions
- Rich media (images, charts) — deep-link instead
- Custom notification sounds

---

## 8. Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Notification fatigue | Conservative defaults. Per-category muting. Quiet hours. Low-priority = no badge. |
| Performance at scale | JSONL file storage with date partitioning. Lightweight SSE count topic. Pagination everywhere. |
| Message accumulation | Retention policy with auto-cleanup. Archive mechanism. |
| "Inbox" vs "Queues" naming | Change Queues icon from `Inbox` to `ListOrdered`. Clear section labeling. |
| Auth-disabled mode | Single shared inbox with `role:*`. Per-user features degrade gracefully. |

---

## Key Files

| Purpose | File |
|---------|------|
| Sidebar navigation (add bell + nav item) | `ui/src/menu.tsx` |
| SSE topic types (add inbox topics) | `internal/service/frontend/sse/types.go` |
| Audit store pattern to follow | `internal/persis/fileaudit/store.go` |
| HITL executor pattern (for notify executor + auto-messages) | `internal/runtime/builtin/hitl/hitl.go` |
| OpenAPI spec (add inbox endpoints) | `api/v1/api.yaml` |
| Executor registration pattern | `internal/runtime/executor/executor.go` |
| Existing toast component (extend for inbox toasts) | `ui/src/components/ui/simple-toast.tsx` |
| App routes (add /inbox route) | `ui/src/App.tsx` |
| SplitLayout (reuse for inbox page) | `ui/src/layouts/` |
