// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import { formatDurationMs } from '@/lib/formatDuration';
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

// jsdom lays nothing out, so pointer tests give a lane an explicit width.
function stubLaneWidth(lane: HTMLElement, width: number) {
  lane.getBoundingClientRect = () =>
    ({
      left: 0,
      top: 0,
      right: width,
      bottom: 36,
      width,
      height: 36,
      x: 0,
      y: 0,
      toJSON: () => ({}),
    }) as DOMRect;
}

function renderLanes(
  laneRuns: ActivityRun[] = runs,
  active: ActivityRun[] = []
) {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route
          path="/"
          element={
            <ActivityLanes
              runs={laneRuns}
              active={active}
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

  it('opens a long run from anywhere along its bar', () => {
    renderLanes([
      {
        name: 'nightly',
        dagRunId: 'nightly-1',
        status: Status.Success,
        startedAt: FROM,
        finishedAt: FROM + 48 * 900,
      },
    ]);
    const lane = screen.getByRole('group', { name: /^nightly:/ });
    stubLaneWidth(lane, 960);

    fireEvent.pointerDown(lane, { button: 0, clientX: 240 });
    fireEvent.pointerUp(lane, { clientX: 240 });

    expect(screen.getByRole('dialog')).toHaveAccessibleName(
      'nightly nightly-1'
    );
  });

  // The window lists only runs created inside it; a run still going from
  // before it comes from the in-progress list.
  it('shows a run that started before the window', () => {
    renderLanes(
      [],
      [
        {
          name: 'backfill',
          dagRunId: 'backfill-1',
          status: Status.Running,
          startedAt: FROM - 3600,
        },
      ]
    );
    const lane = screen.getByRole('group', { name: /^backfill:/ });
    stubLaneWidth(lane, 960);

    lane.focus();
    fireEvent.keyDown(lane, { key: 'ArrowRight' });
    expect(screen.getByRole('tooltip')).toHaveTextContent('Running');
    // A running run shows how long it has run so far.
    expect(screen.getByRole('tooltip')).toHaveTextContent(
      formatDurationMs((TO - 60 - (FROM - 3600)) * 1000)
    );

    fireEvent.pointerDown(lane, { button: 0, clientX: 4 });
    fireEvent.pointerUp(lane, { clientX: 4 });
    expect(screen.getByRole('dialog')).toHaveAccessibleName(
      'backfill backfill-1'
    );
  });
});
