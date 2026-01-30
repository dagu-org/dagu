---
id: "001"
title: "Agent Authorization"
status: draft
---

# RFC 001: Agent Authorization

## Summary

Implement role-based authorization for the AI agent feature.

## Motivation

Currently:
1. The role system has a gap between "operator" (can run DAGs) and "manager" (can write DAGs + admin access)
2. The AI agent has authentication but lacks authorization - any authenticated user can use all agent capabilities

## Proposal

### New Role Hierarchy

| Role | Description |
|------|-------------|
| **admin** | Full system access including user management |
| **manager** | Can write DAGs + access audit logs, system status, webhooks |
| **developer** | Can write DAGs + access system status, webhooks |
| **operator** | Can run and stop DAGs only |
| **viewer** | Read-only access |

**Key distinction:** Manager can access audit logs; developer cannot.

### Permission Matrix

| Capability | viewer | operator | developer | manager | admin |
|------------|--------|----------|-----------|---------|-------|
| View DAGs | ✓ | ✓ | ✓ | ✓ | ✓ |
| Run/Stop DAGs | ✗ | ✓ | ✓ | ✓ | ✓ |
| Create/Edit/Delete DAGs | ✗ | ✗ | ✓ | ✓ | ✓ |
| System Status | ✗ | ✗ | ✓ | ✓ | ✓ |
| Webhooks | ✗ | ✗ | ✓ | ✓ | ✓ |
| Audit Logs | ✗ | ✗ | ✗ | ✓ | ✓ |
| Users Management | ✗ | ✗ | ✗ | ✗ | ✓ |
| API Keys Management | ✗ | ✗ | ✗ | ✗ | ✓ |
| Terminal Access | ✗ | ✗ | ✗ | ✗ | ✓ |
| Agent Settings | ✗ | ✗ | ✗ | ✗ | ✓ |

### AI Agent Authorization

#### Tool Permissions

| Tool | Required Permission | Allowed Roles |
|------|---------------------|---------------|
| bash | CanExecute() | operator, developer, manager, admin |
| patch (create/edit/delete) | CanWrite() | developer, manager, admin |
| read | None | all authenticated users |
| navigate (admin pages) | IsAdmin() | admin |
| navigate (other pages) | None | all authenticated users |
| think | None | all authenticated users |
| read_schema | None | all authenticated users |

#### Defense in Depth

Three layers of protection:

1. **System Prompt Injection**: The LLM is informed of the user's role and capabilities at conversation start. This allows the agent to:
   - Know what actions are available to this user
   - Proactively explain limitations
   - Avoid attempting forbidden actions

2. **Tool-Level Enforcement**: Each tool checks permissions before execution. Even if the LLM attempts a forbidden action (e.g., via prompt injection), the tool will reject it.

3. **Audit Logging**: All agent tool executions and permission denials are logged to the audit log for security monitoring and forensics.

## Security Considerations

1. **Defense in depth**: Both prompt injection and tool-level checks prevent unauthorized actions
2. **Fail-safe**: If User is nil (auth disabled), tools allow all actions for backwards compatibility
3. **Clear error messages**: Users receive clear feedback when actions are denied
4. **Audit trail**: Permission denials should be logged for security monitoring

## Migration

1. Existing users with "manager" role retain their access
2. New "developer" role can be assigned to users who need DAG write access without audit log access
3. No breaking changes to existing role assignments
