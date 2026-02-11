---
id: "002"
title: "Agent Audit Logging"
status: draft
---

# RFC 002: Agent Audit Logging

## Summary

Record audit logs for all AI agent actions with proper user attribution and traceability.

## Motivation

Currently:
1. When the AI agent executes actions, the audit log records "admin" instead of the real user
2. Agent actions are not consistently logged to the audit system

## Proposal

### Use Existing Audit Structure

No schema changes needed. Use the existing `agent` category for all agent tool executions.

### Audit Cases

| Case | Category | Action | User | Details |
|------|----------|--------|------|---------|
| User runs DAG directly | dag | start | real user | dag_name |
| Agent runs DAG | agent | dag_start | real user | dag_name, session_id |
| Agent stops DAG | agent | dag_stop | real user | dag_name, run_id, session_id |
| Agent executes bash | agent | bash_exec | real user | command, exit_code, session_id |
| Agent reads file | agent | file_read | real user | path, session_id |
| Agent modifies file | agent | file_patch | real user | path, session_id |

### User Context

Pass the authenticated user's identity from the session owner through agent tool executions, rather than defaulting to "admin".

### Traceability

Include `session_id` in the details field to trace actions back to specific agent sessions.

## Security Considerations

1. **Bash command output**: NOT included in audit details (may contain secrets)
2. **Failed actions**: Should be audited with error info for security monitoring
3. **No schema changes**: Fully backward compatible
