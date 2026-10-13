// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import { MemoryRouter, Route, Routes, useLocation } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import type { DAGRunActivity } from '../../lib/activityModel';
import { ActivityLanes, stripGradient } from '../ActivityLanes';

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

const activity: DAGRunActivity = {
  from: FROM,
  to: TO,
  bucketSeconds: 900,
  truncated: false,
  counts: [],
  active: [],
  upcoming: [],
  workflows: [
    {
      name: 'report',
      runCount: 2,
      failedCount: 0,
      strip: 'ss'.padEnd(96, '.'),
      runs: [
        { dagRunId: 'report-1', status: Status.Success, startedAt: FROM },
        { dagRunId: 'report-2', status: Status.Success, startedAt: FROM + 900 },
      ],
      latest: {
        dagRunId: 'report-2',
        status: Status.Success,
        startedAt: FROM + 900,
      },
    },
    {
      name: 'billing',
      runCount: 3,
      failedCount: 2,
      strip: '.ff'.padEnd(96, '.'),
      runs: [
        {
          dagRunId: 'billing-3',
          status: Status.Failed,
          startedAt: FROM + 1800,
        },
      ],
      latest: {
        dagRunId: 'billing-3',
        status: Status.Failed,
        startedAt: FROM + 1800,
      },
    },
  ],
};

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
              activity={activity}
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

describe('stripGradient', () => {
  it('merges neighboring buckets of the same kind into one band', () => {
    const gradient = stripGradient('ss..f'.padEnd(96, '.'), activity, {
      from: FROM,
      to: TO,
    });

    expect(gradient?.match(/var\(--status-success\)/g)).toHaveLength(2);
    expect(gradient).toContain('transparent 0.000%');
    expect(gradient).toContain('2.083%');
  });

  it('paints nothing for an empty strip', () => {
    expect(
      stripGradient('.'.repeat(96), activity, { from: FROM, to: TO })
    ).toBeUndefined();
  });
});

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
});
