// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { KanbanCard } from '../KanbanCard';

vi.mock('framer-motion', () => ({
  motion: {
    div: ({
      children,
      layoutId: _layoutId,
      layout: _layout,
      initial: _initial,
      animate: _animate,
      exit: _exit,
      transition: _transition,
      ...props
    }: React.ComponentProps<'div'> & Record<string, unknown>) => (
      <div {...props}>{children}</div>
    ),
  },
}));

describe('KanbanCard', () => {
  it('shows the auto retry label when a retry policy exists', () => {
    render(
      <KanbanCard
        run={{
          dagRunId: 'run-1',
          name: 'retry-dag',
          status: Status.Failed,
          statusLabel: StatusLabel.failed,
          autoRetryCount: 1,
          autoRetryLimit: 3,
          triggerType: TriggerType.manual,
          queuedAt: '',
          scheduleTime: '',
          startedAt: '2026-03-16T00:00:00Z',
          finishedAt: '2026-03-16T00:01:00Z',
        }}
        onClick={() => {}}
      />
    );

    expect(screen.getByText('1/3 auto retries')).toBeInTheDocument();
  });

  it('shows the exhausted label once the retry limit is reached', () => {
    render(
      <KanbanCard
        run={{
          dagRunId: 'run-2',
          name: 'retry-dag',
          status: Status.Failed,
          statusLabel: StatusLabel.failed,
          autoRetryCount: 3,
          autoRetryLimit: 3,
          triggerType: TriggerType.manual,
          queuedAt: '',
          scheduleTime: '',
          startedAt: '2026-03-16T00:00:00Z',
          finishedAt: '2026-03-16T00:01:00Z',
        }}
        onClick={() => {}}
      />
    );

    expect(screen.getByText('auto retries exhausted')).toBeInTheDocument();
  });
});
