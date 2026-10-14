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
import { useSchedulerPause } from '../../hooks/useSchedulerPause';
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

vi.mock('../../hooks/useSchedulerPause', () => ({
  useSchedulerPause: vi.fn(),
}));

const useActivityMock = vi.mocked(useDAGRunActivity);
const useSchedulerPauseMock = vi.mocked(useSchedulerPause);

function mockScheduler(schedulerRunning: boolean | undefined) {
  useSchedulerPauseMock.mockReturnValue({
    schedulerRunning,
  } as ReturnType<typeof useSchedulerPause>);
}
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
    runs: [],
    attention: [],
    active: [],
    upcoming: [],
    queues: [],
    ...overrides,
  };
}

function mockActivity(state: Partial<DAGRunActivityState>) {
  useActivityMock.mockReturnValue({
    runs: state.activity?.runs ?? [],
    isLoading: false,
    unavailable: false,
    refresh: vi.fn().mockResolvedValue(undefined),
    hasMore: false,
    loadMore: vi.fn().mockResolvedValue(undefined),
    isLoadingMore: false,
    loadMoreError: null,
    paused: false,
    resume: vi.fn(),
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

const billingFailure = {
  name: 'billing',
  dagRunId: 'billing-9',
  status: Status.Failed,
  startedAt: NOW - 600,
  failedStep: 'reconcile',
  error: 'exit status 3',
};

const reportRuns = [1, 2, 3].map((index) => ({
  name: 'report',
  dagRunId: `report-${index}`,
  status: Status.Success,
  startedAt: NOW - 60 * index,
}));

const busyActivity = makeActivity({
  runs: [...reportRuns, billingFailure],
  attention: [billingFailure],
  active: [{ name: 'approve', dagRunId: 'a-1', status: Status.Waiting }],
  upcoming: [{ name: 'nightly', fileName: 'nightly', nextRun: NOW + 3600 }],
});

describe('DashboardPage', () => {
  beforeEach(() => {
    mockDAGList([]);
    mockScheduler(true);
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

  it('counts the runs and links each status to the window’s runs', () => {
    mockActivity({ activity: busyActivity });

    renderPage();

    const failed = screen.getByRole('link', { name: /^1\s*failed$/ });
    const url = new URL(failed.getAttribute('href')!, 'http://localhost');
    expect(url.pathname).toBe('/dag-runs');
    expect(Object.fromEntries(url.searchParams)).toEqual({
      status: String(Status.Failed),
      dateMode: 'custom',
      fromDate: String(FROM),
      toDate: String(NOW + 600),
    });
    expect(screen.getByRole('link', { name: /^4\s*runs$/ })).toBeVisible();
  });

  // With older runs left to load, the counts cover only the loaded ones.
  it('labels the counts as the latest runs and loads older ones', () => {
    const loadMore = vi.fn().mockResolvedValue(undefined);
    mockActivity({ activity: busyActivity, hasMore: true, loadMore });

    renderPage();

    expect(
      screen.getByRole('link', { name: /^4\s*latest runs$/ })
    ).toBeVisible();
    fireEvent.click(screen.getByRole('button', { name: 'Load more' }));
    expect(loadMore).toHaveBeenCalled();
  });

  it('returns to live from loaded older runs', () => {
    const resume = vi.fn();
    mockActivity({ activity: busyActivity, paused: true, resume });

    renderPage();
    fireEvent.click(screen.getByRole('button', { name: 'Back to live' }));

    expect(resume).toHaveBeenCalled();
  });

  it('warns when a stopped scheduler holds back scheduled runs', () => {
    mockScheduler(false);
    mockActivity({ activity: busyActivity });

    renderPage();

    const attention = screen.getByRole('region', { name: /Needs attention/ });
    expect(
      within(attention).getByText(/The scheduler is not running/)
    ).toBeVisible();
    expect(
      within(attention).getByRole('link', { name: /System Status/ })
    ).toHaveAttribute('href', '/system-status');
  });

  it('summarizes each queue instead of listing its queued runs', () => {
    // Running status text animates and reads the reduced-motion preference.
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation((query: string) => ({
        matches: false,
        media: query,
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
      })),
    });
    mockActivity({
      activity: makeActivity({
        runs: reportRuns,
        active: [
          { name: 'etl', dagRunId: 'etl-1', status: Status.Queued },
          {
            name: 'train',
            dagRunId: 'train-1',
            status: Status.Running,
            startedAt: NOW - 60,
          },
        ],
        queues: [
          {
            name: 'default',
            running: 1,
            maxConcurrency: 1,
            queued: 20,
            nextQueuedAt: NOW - 600,
          },
        ],
      }),
    });

    renderPage();

    const queues = screen.getByRole('list', { name: 'Queues' });
    expect(queues).toHaveTextContent('1 of 1 running');
    expect(queues).toHaveTextContent('20 queued');
    expect(queues).toHaveTextContent(/next waiting 10m/);
    expect(
      within(queues).getByRole('link', { name: 'default' })
    ).toHaveAttribute('href', '/queues/default');
    expect(screen.getByRole('link', { name: 'train' })).toBeVisible();
    expect(screen.queryByRole('link', { name: 'etl' })).not.toBeInTheDocument();
  });

  it('says all is clear when nothing needs attention', () => {
    mockActivity({ activity: makeActivity({ runs: reportRuns }) });

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
