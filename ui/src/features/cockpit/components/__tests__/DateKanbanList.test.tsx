// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from '@testing-library/react';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { DateKanbanList } from '../DateKanbanList';
import { useInfiniteKanban } from '../../hooks/useInfiniteKanban';

const dagRunDetailsModalMock = vi.hoisted(() => vi.fn());

vi.mock('../../hooks/useInfiniteKanban', () => ({
  useInfiniteKanban: vi.fn(),
}));

vi.mock('../DateKanbanSection', () => ({
  DateKanbanSection: ({
    date,
    onCardClick,
  }: {
    date: string;
    onCardClick: (run: unknown) => void;
  }) => (
    <div>
      <span>{date}</span>
      <button
        type="button"
        onClick={() =>
          onCardClick({
            dagRunId: 'run-with-artifacts',
            name: 'artifact-dag',
            artifactsAvailable: true,
          })
        }
      >
        Open run {date}
      </button>
    </div>
  ),
}));

vi.mock('../ArtifactListModal', () => ({
  ArtifactListModal: () => null,
}));

vi.mock(
  '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal',
  () => ({
    default: (props: unknown) => {
      dagRunDetailsModalMock(props);
      return null;
    },
  })
);

const useInfiniteKanbanMock = useInfiniteKanban as unknown as {
  mockReturnValue: (value: unknown) => void;
};

function latestModalProps() {
  const calls = dagRunDetailsModalMock.mock.calls;
  return calls[calls.length - 1]?.[0] as {
    name: string;
    dagRunId: string;
    isOpen: boolean;
    initialTab: string;
    onClose: () => void;
  };
}

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

    render(<DateKanbanList selectedWorkspace="ops" />);

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

    render(<DateKanbanList selectedWorkspace="ops" suspendLoadMore={true} />);

    expect(
      screen.getByRole('button', { name: 'Load older day' })
    ).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: 'Load older day' }));
    expect(loadNextDate).not.toHaveBeenCalled();
  });

  it('opens artifact runs on the artifacts tab and keeps props during close', () => {
    useInfiniteKanbanMock.mockReturnValue({
      loadedDates: ['2026-03-21'],
      todayStr: '2026-03-21',
      hasMore: false,
      loadNextDate: vi.fn(),
    });

    render(<DateKanbanList selectedWorkspace="ops" />);

    fireEvent.click(
      screen.getByRole('button', { name: 'Open run 2026-03-21' })
    );

    let modalProps = latestModalProps();
    expect(modalProps).toEqual(
      expect.objectContaining({
        name: 'artifact-dag',
        dagRunId: 'run-with-artifacts',
        isOpen: true,
        initialTab: 'artifacts',
      })
    );

    act(() => {
      modalProps.onClose();
    });

    modalProps = latestModalProps();
    expect(modalProps).toEqual(
      expect.objectContaining({
        name: 'artifact-dag',
        dagRunId: 'run-with-artifacts',
        isOpen: false,
        initialTab: 'artifacts',
      })
    );
  });
});
