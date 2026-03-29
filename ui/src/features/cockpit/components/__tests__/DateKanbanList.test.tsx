// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DateKanbanList } from '../DateKanbanList';
import { useInfiniteKanban } from '../../hooks/useInfiniteKanban';

vi.mock('../../hooks/useInfiniteKanban', () => ({
  useInfiniteKanban: vi.fn(),
}));

vi.mock('../DateKanbanSection', () => ({
  DateKanbanSection: ({ date }: { date: string }) => <div>{date}</div>,
}));

vi.mock('@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal', () => ({
  default: () => null,
}));

const useInfiniteKanbanMock = useInfiniteKanban as unknown as {
  mockReturnValue: (value: unknown) => void;
};

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

describe('DateKanbanList', () => {
  it('renders the fallback button and loads one older day on demand', () => {
    const loadNextDate = vi.fn();
    useInfiniteKanbanMock.mockReturnValue({
      loadedDates: ['2026-03-21', '2026-03-20'],
      todayStr: '2026-03-21',
      hasMore: true,
      loadNextDate,
    });

    render(<DateKanbanList selectedWorkspace="ops" workspaceReady={true} />);

    fireEvent.click(screen.getByRole('button', { name: 'Load older day' }));

    expect(loadNextDate).toHaveBeenCalledTimes(1);
  });

  it('disables demand loading while the template details modal is open', () => {
    const loadNextDate = vi.fn();
    useInfiniteKanbanMock.mockReturnValue({
      loadedDates: ['2026-03-21', '2026-03-20'],
      todayStr: '2026-03-21',
      hasMore: true,
      loadNextDate,
    });

    render(
      <DateKanbanList
        selectedWorkspace="ops"
        workspaceReady={true}
        suspendLoadMore={true}
      />
    );

    expect(screen.getByRole('button', { name: 'Load older day' })).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Load older day' }));
    expect(loadNextDate).not.toHaveBeenCalled();
  });
});
