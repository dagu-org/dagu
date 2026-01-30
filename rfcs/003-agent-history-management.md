---
id: "003"
title: "Agent Conversation History Management"
status: draft
---

# RFC 003: Agent Conversation History Management

## Summary

Improve how agent conversation history is stored, retrieved, and displayed to support long-running conversations efficiently.

## Motivation

Currently:
1. All messages are loaded at once regardless of conversation length
2. UI becomes slow and unresponsive with large conversations
3. No mechanism to incrementally load historical messages
4. Memory usage grows unbounded with conversation length

## Proposal

### Message Retrieval

Support paginated message retrieval:
- Load most recent messages first
- Allow loading older messages on demand
- Use `sequence_id` as cursor for reliable pagination

### API Changes

Add optional parameters to conversation retrieval:

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

Add paginated retrieval to `ConversationStore`:
```
GetMessagesPaginated(ctx, conversationID, beforeSequenceID, limit) -> (messages, hasMore, error)
```

## Future Considerations

1. **Conversation archival**: Auto-archive old conversations
2. **Message retention**: Configurable retention policies
3. **Conversation search**: Search across message history
4. **Export**: Export conversation history

## Security Considerations

1. Pagination respects existing authorization checks
2. No new data exposure - only changes delivery mechanism
