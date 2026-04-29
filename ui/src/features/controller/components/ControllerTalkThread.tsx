// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';

import { AgentMessageType, type components } from '@/api/v1/schema';
import { ToolCallList } from '@/features/agent/components/messages/ToolCallBadge';
import { ToolResultMessage } from '@/features/agent/components/messages/ToolResultMessage';
import type { ToolCall, ToolResult } from '@/features/agent/types';
import {
  agentMessageLabel,
  formatAbsoluteTime,
  type ControllerThreadItem,
} from '@/features/controller/detail-utils';
import { cn } from '@/lib/utils';

function toToolResults(
  results?: components['schemas']['AgentToolResult'][]
): ToolResult[] {
  return (results || []).map((result, index) => ({
    tool_call_id: result.toolCallId || `tool-result-${index}`,
    content: result.content || '',
    is_error: result.isError,
  }));
}

function ThreadBubble({
  item,
}: {
  item: ControllerThreadItem;
}): React.ReactElement {
  if (item.kind === 'queued') {
    return (
      <div className="flex items-start">
        <div className="max-w-[90%] rounded-2xl border border-amber-300/40 bg-amber-50 px-4 py-3 text-sm dark:border-amber-700/40 dark:bg-amber-950/20">
          <div className="mb-1 flex items-center justify-between gap-4 text-[11px] font-medium uppercase tracking-wide text-amber-800 dark:text-amber-200">
            <span>{item.queuedKind.replace(/_/g, ' ')} queued</span>
            {item.createdAt ? (
              <span className="normal-case tracking-normal">
                {formatAbsoluteTime(item.createdAt)}
              </span>
            ) : null}
          </div>
          <div className="whitespace-pre-wrap break-words">{item.message}</div>
        </div>
      </div>
    );
  }

  const message = item.message;
  const isToolResultMessage =
    message.type === AgentMessageType.user &&
    (message.toolResults?.length || 0) > 0;

  if (isToolResultMessage) {
    return (
      <div className="flex justify-start">
        <div className="max-w-[90%]">
          <ToolResultMessage toolResults={toToolResults(message.toolResults)} />
        </div>
      </div>
    );
  }

  const isUser = message.type === AgentMessageType.user;
  const isError = message.type === AgentMessageType.error;

  return (
    <div className={cn('flex', isUser ? 'justify-end' : 'justify-start')}>
      <div
        className={cn(
          'max-w-[90%] rounded-2xl border px-4 py-3 text-sm',
          isUser
            ? 'border-primary/20 bg-primary/5'
            : isError
              ? 'border-destructive/30 bg-destructive/5'
              : 'bg-muted/40'
        )}
      >
        <div className="mb-1 flex items-center justify-between gap-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <span>{agentMessageLabel(message.type)}</span>
          {message.createdAt ? (
            <span className="normal-case tracking-normal">
              {formatAbsoluteTime(message.createdAt)}
            </span>
          ) : null}
        </div>
        {message.content ? (
          <div className="whitespace-pre-wrap break-words">
            {message.content}
          </div>
        ) : null}
        {message.userPrompt?.question ? (
          <div className="whitespace-pre-wrap break-words">
            {message.userPrompt.question}
          </div>
        ) : null}
        {message.toolCalls?.length ? (
          <ToolCallList
            toolCalls={message.toolCalls as ToolCall[]}
            className="mt-3"
          />
        ) : null}
      </div>
    </div>
  );
}

export function ControllerTalkThread({
  items,
  sessionId,
  active,
}: {
  items: ControllerThreadItem[];
  sessionId?: string;
  active: boolean;
}): React.ReactElement {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const shouldFollowRef = React.useRef(true);

  const scrollToBottom = React.useCallback(() => {
    const node = containerRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, []);

  React.useEffect(() => {
    const node = containerRef.current;
    if (!node) {
      return;
    }

    const onScroll = () => {
      const remaining = node.scrollHeight - node.scrollTop - node.clientHeight;
      shouldFollowRef.current = remaining < 48;
    };

    onScroll();
    node.addEventListener('scroll', onScroll);
    return () => node.removeEventListener('scroll', onScroll);
  }, []);

  React.useLayoutEffect(() => {
    if (!active) {
      return;
    }
    shouldFollowRef.current = true;
    requestAnimationFrame(scrollToBottom);
  }, [active, scrollToBottom, sessionId]);

  React.useLayoutEffect(() => {
    if (!active || !shouldFollowRef.current) {
      return;
    }
    requestAnimationFrame(scrollToBottom);
  }, [active, items.length, scrollToBottom]);

  return (
    <div className="min-w-0 rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold">Talk Thread</h2>
        {sessionId ? (
          <span className="text-[11px] text-muted-foreground">
            Session: {sessionId}
          </span>
        ) : null}
      </div>
      {items.length ? (
        <div
          ref={containerRef}
          className="max-h-[34rem] space-y-3 overflow-y-auto pr-1"
        >
          {items.map((item) => (
            <ThreadBubble key={item.id} item={item} />
          ))}
        </div>
      ) : (
        <div className="text-sm text-muted-foreground">
          No session or queued messages yet.
        </div>
      )}
    </div>
  );
}
