// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { Status } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { WorkspaceKind } from '@/lib/workspace';
import {
  type DAGRunActivityState,
  useDAGRunActivity,
} from '../../features/activity/hooks/useDAGRunActivity';
import type { DAGRunActivity } from '../../features/activity/lib/activityModel';
import { useQuery } from '../../hooks/api';
import DashboardPage from '../index';

vi.mock('../../features/activity/hooks/useDAGRunActivity', () => ({
  useDAGRunActivity: vi.fn(),
}));

vi.mock('../../features/dags/components/common', () => ({
  CreateDAGModal: () => <button type="button">New workflow</button>,
}));

vi.mock('@/features/dag-runs/components/dag-run-details', () => ({
  DAGRunDetailsModal: () => null,
}));

vi.mock('@/features/dag-runs/components/common', () => ({
  DAGRunActions: () => <button type="button">Retry</button>,
}));

vi.mock('../../features/system-status/components/PathsCard', () => ({
  default: () => null,
}));

vi.mock('../../hooks/api', () => ({
  useQuery: vi.fn(),
}));

const useActivityMock = vi.mocked(useDAGRunActivity);
const useQueryMock = vi.mocked(
  useQuery as unknown as (path: string, init: unknown) => unknown
);

const NOW = Math.floor(Date.now() / 1000);
const FROM = NOW - 24 * 3600;

function makeConfig(overrides: Partial<Config> = {}): Config {
  return {
    apiURL: '/api/v1',
    basePath: '/',
    title: 'Dagu',
    navbarColor: '',
    tz: 'UTC',
    tzOffsetInSec: 0,
    version: 'test',
    maxDashboardPageLimit: 100,
    remoteNodes: 'local,remote-a',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    proxyEnabled: false,
    proxyButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    updateAvailable: false,
    latestVersion: '',
    permissions: {
      writeDags: true,
      runDags: true,
    },
    license: {
      valid: true,
      plan: 'community',
      expiry: '',
      features: [],
      gracePeriod: false,
      community: true,
      source: 'test',
      warningCode: '',
    },
    paths: {
      dagsDir: '',
      logDir: '',
      suspendFlagsDir: '',
      adminLogsDir: '',
      baseConfig: '',
      dagRunsDir: '',
      queueDir: '',
      procDir: '',
      serviceRegistryDir: '',
      configFileUsed: '',
      gitSyncDir: '',
      auditLogsDir: '',
    },
    ...overrides,
  };
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/']}>
      <ConfigContext.Provider value={makeConfig()}>
        <AppBarContext.Provider
          value={{
            title: '',
            setTitle: () => undefined,
            remoteNodes: ['local'],
            setRemoteNodes: () => undefined,
            selectedRemoteNode: 'local',
            selectRemoteNode: () => undefined,
            workspaces: [],
            workspaceSelection: { kind: WorkspaceKind.all },
            selectWorkspace: () => undefined,
          }}
        >
          <DashboardPage />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

function makeActivity(overrides: Partial<DAGRunActivity> = {}): DAGRunActivity {
  return {
    from: FROM,
    to: NOW + 600,
    bucketSeconds: 900,
    truncated: false,
    counts: [],
    workflows: [],
    active: [],
    upcoming: [],
    ...overrides,
  };
}

function mockActivity(state: Partial<DAGRunActivityState>) {
  useActivityMock.mockReturnValue({
    isLoading: false,
    unavailable: false,
    refresh: vi.fn().mockResolvedValue(undefined),
    ...state,
  });
}

function mockDAGList(names: string[]) {
  useQueryMock.mockImplementation((path, init) =>
    path === '/dags' && init
      ? {
          data: {
            dags: names.map((name) => ({ dag: { name } })),
            pagination: { totalRecords: names.length },
          },
        }
      : { data: undefined }
  );
}

const busyActivity = makeActivity({
  counts: [
    { status: Status.Success, count: 40 },
    { status: Status.Failed, count: 3 },
  ],
  workflows: [
    {
      name: 'billing',
      runCount: 12,
      failedCount: 3,
      strip: 'f'.padStart(96, '.'),
      runs: [],
      latest: {
        dagRunId: 'billing-9',
        status: Status.Failed,
        startedAt: NOW - 600,
        failedStep: 'reconcile',
        error: 'exit status 3',
      },
    },
    {
      name: 'report',
      runCount: 31,
      failedCount: 0,
      strip: 's'.padStart(96, '.'),
      runs: [],
      latest: {
        dagRunId: 'report-1',
        status: Status.Success,
        startedAt: NOW - 60,
      },
    },
  ],
  active: [{ name: 'approve', dagRunId: 'a-1', status: Status.Waiting }],
  upcoming: [{ name: 'nightly', fileName: 'nightly', nextRun: NOW + 3600 }],
});

describe('DashboardPage', () => {
  beforeEach(() => {
    mockDAGList([]);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('shows what needs attention with the failing step', () => {
    mockActivity({ activity: busyActivity });

    renderPage();

    const attention = screen.getByRole('region', { name: /Needs attention/ });
    const items = within(attention).getAllByRole('listitem');
    expect(items[0]).toHaveTextContent('billing');
    expect(items[0]).toHaveTextContent('reconcile: exit status 3');
    expect(items[1]).toHaveTextContent('approve');
    expect(items[1]).toHaveTextContent('Waiting for input');
    expect(
      within(items[1]!).getByRole('link', { name: /Review/ })
    ).toHaveAttribute('href', '/dag-runs/approve/a-1?remoteNode=local');
  });

  it('links each exact count to the matching runs', () => {
    mockActivity({ activity: busyActivity });

    renderPage();

    const failed = screen.getByRole('link', { name: /^3\s*failed$/ });
    const url = new URL(failed.getAttribute('href')!, 'http://localhost');
    expect(url.pathname).toBe('/dag-runs');
    expect(Object.fromEntries(url.searchParams)).toEqual({
      status: String(Status.Failed),
      dateMode: 'custom',
      fromDate: String(FROM),
      toDate: String(NOW + 600),
    });
    expect(screen.getByRole('link', { name: /^43\s*runs$/ })).toBeVisible();
  });

  it('says all is clear when nothing needs attention', () => {
    mockActivity({
      activity: makeActivity({
        workflows: [busyActivity.workflows[1]!],
        counts: [{ status: Status.Success, count: 31 }],
      }),
    });

    renderPage();

    expect(screen.getByText(/All clear/)).toBeVisible();
  });

  it('filters workflows by name', () => {
    mockActivity({ activity: busyActivity });

    renderPage();
    fireEvent.change(screen.getByLabelText('Filter workflows'), {
      target: { value: 'rep' },
    });

    const lanes = within(
      screen.getByRole('list', { name: 'Runs by workflow' })
    );
    expect(lanes.getAllByRole('listitem')).toHaveLength(1);
    expect(lanes.getByText('report')).toBeVisible();
  });

  it('invites creating the first workflow when no workflows exist', () => {
    mockActivity({ activity: makeActivity() });

    renderPage();

    expect(screen.getByText('Create your first workflow')).toBeVisible();
    expect(screen.getByText('New workflow')).toBeVisible();
  });

  it('points at the Workflows page when workflows exist but nothing ran', () => {
    mockDAGList(['etl', 'backup']);
    mockActivity({ activity: makeActivity() });

    renderPage();

    expect(screen.getByText('No runs in the last 24 hours')).toBeVisible();
    expect(screen.getByText(/Start a workflow from the/)).toBeVisible();
    expect(
      screen.queryByText('Create your first workflow')
    ).not.toBeInTheDocument();
  });

  it('suggests the example workflows when only seeded examples exist', () => {
    mockDAGList(['example-01-basic-sequential']);
    mockActivity({ activity: makeActivity() });

    renderPage();

    expect(screen.getByText(/Run one of the example workflows/)).toBeVisible();
  });

  it('explains when the node cannot serve activity', () => {
    const refresh = vi.fn().mockResolvedValue(undefined);
    mockActivity({ unavailable: true, refresh });

    renderPage();

    expect(
      screen.getByText('Activity is not available from this node')
    ).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Try again' }));
    expect(refresh).toHaveBeenCalled();
  });
});
