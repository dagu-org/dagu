// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import type { ActivityRun } from '../../lib/activityModel';
import { ActivityLanes } from '../ActivityLanes';

vi.mock('@/features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: ({
    name,
    dagRunId,
    onNavigate,
  }: {
    name: string;
    dagRunId: string;
    onNavigate: (direction: 'up' | 'down') => void;
  }) => (
    <div role="dialog" aria-label={`${name} ${dagRunId}`}>
      <button type="button" onClick={() => onNavigate('down')}>
        Older run
      </button>
    </div>
  ),
}));

const FROM = 1_790_000_000 - (1_790_000_000 % 900);
const TO = FROM + 96 * 900;

// Loaded runs, newest first, as the API lists them.
const runs: ActivityRun[] = [
  {
    name: 'billing',
    dagRunId: 'billing-3',
    status: Status.Failed,
    startedAt: FROM + 1800,
  },
  {
    name: 'billing',
    dagRunId: 'billing-2',
    status: Status.Failed,
    startedAt: FROM + 1500,
  },
  {
    name: 'report',
    dagRunId: 'report-2',
    status: Status.Success,
    startedAt: FROM + 900,
  },
  {
    name: 'billing',
    dagRunId: 'billing-1',
    status: Status.Success,
    startedAt: FROM + 600,
  },
  {
    name: 'report',
    dagRunId: 'report-1',
    status: Status.Success,
    startedAt: FROM,
  },
];

function LocationProbe() {
  const location = useLocation();
  return <div data-testid="location">{location.pathname}</div>;
}

function renderLanes() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route
          path="/"
          element={
            <ActivityLanes
              runs={runs}
              active={[]}
              window={{ from: FROM, to: TO }}
              hasMore={false}
              range={{ from: FROM, to: TO }}
              nowSeconds={TO - 60}
              remoteNode="local"
              onZoom={vi.fn()}
            />
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </Routes>
    </MemoryRouter>
  );
}

describe('ActivityLanes', () => {
  it('lists failing workflows first with their failure share', () => {
    renderLanes();

    const lanes = within(
      screen.getByRole('list', { name: 'Runs by workflow' })
    ).getAllByRole('listitem');
    expect(lanes[0]).toHaveTextContent('billing');
    expect(lanes[0]).toHaveTextContent('2 of 3 failed');
    expect(lanes[1]).toHaveTextContent('report');
    expect(lanes[1]).toHaveTextContent('2 runs');
  });

  // Entering a lane selects its newest run; arrows step through the rest.
  it('moves between a lane’s runs with the keyboard and opens one', () => {
    renderLanes();
    const lane = screen.getByRole('group', { name: /^report:/ });

    lane.focus();
    fireEvent.keyDown(lane, { key: 'ArrowRight' });
    expect(screen.getByRole('tooltip')).toHaveTextContent('report');
    fireEvent.keyDown(lane, { key: 'Enter' });

    expect(screen.getByRole('dialog')).toHaveAccessibleName('report report-2');
  });

  it('steps the run modal through the lane’s runs, newest to oldest', () => {
    renderLanes();
    const lane = screen.getByRole('group', { name: /^report:/ });
    lane.focus();
    fireEvent.keyDown(lane, { key: 'ArrowRight' });
    fireEvent.keyDown(lane, { key: 'Enter' });

    fireEvent.click(screen.getByRole('button', { name: 'Older run' }));

    expect(screen.getByRole('dialog')).toHaveAccessibleName('report report-1');
  });

  it('draws a run as a bar from its start to its end', () => {
    const long: ActivityRun[] = [
      {
        name: 'nightly',
        dagRunId: 'nightly-1',
        status: Status.Success,
        startedAt: FROM,
        finishedAt: FROM + 24 * 900,
      },
    ];
    render(
      <MemoryRouter>
        <ActivityLanes
          runs={long}
          active={[]}
          window={{ from: FROM, to: TO }}
          hasMore={false}
          range={{ from: FROM, to: TO }}
          nowSeconds={TO - 60}
          remoteNode="local"
          onZoom={vi.fn()}
        />
      </MemoryRouter>
    );

    const lane = screen.getByRole('group', { name: /^nightly:/ });
    const bar = lane.querySelector<HTMLElement>('span[style*="left: 0%"]');
    expect(bar?.style.width).toBe('25%');
  });
});
