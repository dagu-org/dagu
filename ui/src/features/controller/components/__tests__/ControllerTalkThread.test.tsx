// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it } from 'vitest';

import { AgentMessageType } from '@/api/v1/schema';
import { TOOL_RESULT_PREVIEW_LENGTH } from '@/features/agent/constants';
import { ControllerTalkThread } from '@/features/controller/components/ControllerTalkThread';
import type { ControllerThreadItem } from '@/features/controller/detail-utils';

afterEach(() => {
  cleanup();
});

describe('ControllerTalkThread', () => {
  it('renders tool calls and compact tool results without mislabeling them as user messages', () => {
    const longContent = 'x'.repeat(TOOL_RESULT_PREVIEW_LENGTH + 20);
    const preview = `${longContent.slice(0, TOOL_RESULT_PREVIEW_LENGTH)}...`;
    const items: ControllerThreadItem[] = [
      {
        id: 'm-1',
        kind: 'message',
        createdAt: '2026-04-29T12:48:00Z',
        message: {
          id: 'm-1',
          sessionId: 'sess-1',
          type: AgentMessageType.assistant,
          sequenceId: 1,
          createdAt: '2026-04-29T12:48:00Z',
          content: 'I am checking the current workflows.',
          toolCalls: [
            {
              id: 'call-1',
              type: 'function',
              function: {
                name: 'read',
                arguments: '{"path":"/tmp/spec.yaml"}',
              },
            },
          ],
        },
      },
      {
        id: 'm-2',
        kind: 'message',
        createdAt: '2026-04-29T12:48:01Z',
        message: {
          id: 'm-2',
          sessionId: 'sess-1',
          type: AgentMessageType.user,
          sequenceId: 2,
          createdAt: '2026-04-29T12:48:01Z',
          toolResults: [
            {
              toolCallId: 'call-1',
              content: longContent,
              isError: false,
            },
          ],
        },
      },
    ];

    render(
      <ControllerTalkThread items={items} sessionId="sess-1" active={false} />
    );

    expect(screen.getByText('Controller')).toBeInTheDocument();
    expect(screen.getByText('read')).toBeInTheDocument();
    expect(screen.getByText(preview)).toBeInTheDocument();
    expect(screen.queryByText('You')).not.toBeInTheDocument();
    expect(screen.queryByText(longContent)).not.toBeInTheDocument();
  });
});
