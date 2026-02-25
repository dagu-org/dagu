# RFC 003: Agent Session History Management

## Summary

Improve how agent session history is stored, retrieved, and displayed to support long-running sessions efficiently.

## Motivation

Currently:
1. All messages are loaded at once regardless of session length
2. UI becomes slow and unresponsive with large sessions
3. No mechanism to incrementally load historical messages
4. Memory usage grows unbounded with session length

## Proposal

### Message Retrieval

Support paginated message retrieval:
- Load most recent messages first
- Allow loading older messages on demand
- Use `sequence_id` as cursor for reliable pagination

### API Changes

Add optional parameters to session retrieval:

| Parameter | Description |
|-----------|-------------|
| `before` | Cursor: return messages before this sequence_id |
| `limit` | Maximum messages to return (default: 50) |

Response includes:
- `has_more`: whether older messages exist
- `oldest_sequence_id`: cursor for next page

### UI Behavior

- Initial load shows recent messages
- Scroll up to load older history
- Preserve scroll position when loading older messages
- Real-time messages continue via SSE

### Store Interface

Add paginated retrieval to `SessionStore`:
```
GetMessagesPaginated(ctx, sessionID, beforeSequenceID, limit) -> (messages, hasMore, error)
```

## Future Considerations

1. **Session archival**: Auto-archive old sessions
2. **Message retention**: Configurable retention policies
3. **Session search**: Search across message history
4. **Export**: Export session history

## Security Considerations

1. Pagination respects existing authorization checks
2. No new data exposure - only changes delivery mechanism
