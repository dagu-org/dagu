// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { KanbanColumn } from '../KanbanColumn';
import { MobileKanbanBoard } from '../MobileKanbanBoard';
import type {
  KanbanColumnData,
  KanbanColumns,
} from '../../hooks/useDateKanbanData';

vi.mock('framer-motion', () => ({
  AnimatePresence: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
  LayoutGroup: ({ children }: { children: React.ReactNode }) => <>{children}</>,
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

vi.mock('../KanbanCard', () => ({
  KanbanCard: ({ run }: { run: { dagRunId: string } }) => (
    <div data-testid={`kanban-card-${run.dagRunId}`} />
  ),
}));

function createColumn(
  overrides: Partial<KanbanColumnData> = {}
): KanbanColumnData {
  return {
    runs: [
      {
        dagRunId: 'run-1',
        name: 'example',
        status: Status.Running,
        statusLabel: StatusLabel.running,
        autoRetryCount: 0,
        triggerType: TriggerType.manual,
        queuedAt: '',
        scheduleTime: '',
        startedAt: '',
        finishedAt: '',
      },
      {
        dagRunId: 'run-2',
        name: 'example',
        status: Status.Running,
        statusLabel: StatusLabel.running,
        autoRetryCount: 0,
        triggerType: TriggerType.manual,
        queuedAt: '',
        scheduleTime: '',
        startedAt: '',
        finishedAt: '',
      },
    ],
    hasMore: false,
    isInitialLoading: false,
    isLoadingMore: false,
    error: null,
    loadMoreError: null,
    loadMore: vi.fn(async () => {}),
    retry: vi.fn(async () => {}),
    ...overrides,
  };
}

describe('cockpit count labels', () => {
  it('shows a plus in desktop headers when the column has more runs', () => {
    render(
      <KanbanColumn
        title="Running"
        column={createColumn({ hasMore: true })}
        onCardClick={() => {}}
      />
    );

    expect(screen.getByText('2+')).toBeInTheDocument();
  });

  it('shows a plus in mobile tabs when the column has more runs', () => {
    const columns: KanbanColumns = {
      queued: createColumn(),
      running: createColumn({ hasMore: true }),
      review: createColumn({ runs: [] }),
      done: createColumn({ runs: [] }),
      failed: createColumn({ runs: [] }),
    };

    render(<MobileKanbanBoard columns={columns} onCardClick={() => {}} />);

    expect(screen.getByRole('button', { name: /Running/ })).toHaveTextContent(
      'Running2+'
    );
  });

  it('bounds the mobile column area so the active tab can scroll internally', () => {
    const columns: KanbanColumns = {
      queued: createColumn(),
      running: createColumn({ hasMore: true }),
      review: createColumn({ runs: [] }),
      done: createColumn({ runs: [] }),
      failed: createColumn({ runs: [] }),
    };

    const { container } = render(
      <MobileKanbanBoard columns={columns} onCardClick={() => {}} />
    );

    const boardRoot = container.firstElementChild;
    expect(boardRoot).toHaveClass('max-h-[70vh]');
    expect(boardRoot).toHaveClass('overflow-hidden');
    expect(boardRoot?.lastElementChild).toHaveClass('overflow-hidden');
  });
});
