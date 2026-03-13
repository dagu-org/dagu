// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { Status, StatusLabel, TriggerType } from '@/api/v1/schema';
import { Config, ConfigContext } from '@/contexts/ConfigContext';
import DAGRunTable from '../DAGRunTable';

vi.mock('../StepDetailsTooltip', () => ({
  StepDetailsTooltip: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

const config = {
  apiURL: '',
  basePath: '',
  title: 'Dagu',
  navbarColor: '',
  tz: 'UTC',
  tzOffsetInSec: 0,
  version: 'test',
  maxDashboardPageLimit: 100,
  remoteNodes: '',
  authMode: 'none',
  setupRequired: false,
  oidcEnabled: false,
  oidcButtonLabel: '',
  terminalEnabled: false,
  gitSyncEnabled: false,
  agentEnabled: false,
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
} as Config;

describe('DAGRunTable', () => {
  it('shows the scheduled at column and value when schedule time exists', () => {
    render(
      <MemoryRouter>
        <ConfigContext.Provider value={config}>
          <DAGRunTable
            dagRuns={[
              {
                dagRunId: 'run-1',
                name: 'scheduled-dag',
                status: Status.Queued,
                statusLabel: StatusLabel.queued,
                triggerType: TriggerType.scheduler,
                queuedAt: '2026-03-13T10:00:30Z',
                scheduleTime: '2026-03-13T10:00:00Z',
                startedAt: '',
                finishedAt: '',
              },
            ]}
          />
        </ConfigContext.Provider>
      </MemoryRouter>
    );

    expect(screen.getByText('Scheduled At')).toBeInTheDocument();
    expect(screen.getByText('2026-03-13T10:00:00Z')).toBeInTheDocument();
    expect(screen.getByText('2026-03-13T10:00:30Z')).toBeInTheDocument();
  });
});
