// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import * as React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import CockpitPage from '../index';

vi.mock('@/features/cockpit/hooks/useCockpitState', () => ({
  useCockpitState: () => ({
    workspaces: [],
    selectedWorkspace: '',
    selectedTemplate: '',
    createWorkspace: vi.fn(),
    deleteWorkspace: vi.fn(),
    selectWorkspace: vi.fn(),
    selectTemplate: vi.fn(),
  }),
}));

vi.mock('@/features/cockpit/components/CockpitToolbar', () => ({
  CockpitToolbar: () => <div data-testid="cockpit-toolbar" />,
}));

vi.mock('@/features/cockpit/components/DateKanbanList', () => ({
  DateKanbanList: () => <div data-testid="dag-runs-cockpit" />,
}));

vi.mock('@/features/cockpit/components/AutopilotCockpit', () => ({
  AutopilotCockpit: () => <div data-testid="autopilot-cockpit" />,
}));

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
    remoteNodes: 'local',
    initialWorkspaces: [],
    authMode: 'none',
    setupRequired: false,
    oidcEnabled: false,
    oidcButtonLabel: '',
    terminalEnabled: false,
    gitSyncEnabled: false,
    autopilotEnabled: false,
    agentEnabled: true,
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

function renderPage(config: Config, initialEntry = '/cockpit') {
  return render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <ConfigContext.Provider value={config}>
        <AppBarContext.Provider
          value={{
            title: '',
            setTitle: () => undefined,
            remoteNodes: ['local'],
            setRemoteNodes: () => undefined,
            selectedRemoteNode: 'local',
            selectRemoteNode: () => undefined,
          }}
        >
          <CockpitPage />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('CockpitPage', () => {
  afterEach(() => {
    cleanup();
    localStorage.clear();
  });

  it('hides the Autopilot mode switch when Autopilot UI is disabled', () => {
    renderPage(makeConfig({ autopilotEnabled: false }));

    expect(
      screen.queryByRole('button', { name: 'Autopilot cockpit' })
    ).not.toBeInTheDocument();
    expect(screen.getByTestId('dag-runs-cockpit')).toBeInTheDocument();
  });

  it('shows the Autopilot mode switch when Autopilot UI is enabled', () => {
    renderPage(makeConfig({ autopilotEnabled: true }));

    fireEvent.click(screen.getByRole('button', { name: 'Autopilot cockpit' }));

    expect(screen.getByTestId('autopilot-cockpit')).toBeInTheDocument();
  });

  it('opens Autopilot mode from query params', () => {
    renderPage(
      makeConfig({ autopilotEnabled: true }),
      '/cockpit?mode=autopilot&autopilot=builder'
    );

    expect(screen.getByTestId('autopilot-cockpit')).toBeInTheDocument();
  });

  it('opens Autopilot mode from legacy Automata query params', () => {
    renderPage(
      makeConfig({ autopilotEnabled: true }),
      '/cockpit?mode=automata&automata=builder'
    );

    expect(screen.getByTestId('autopilot-cockpit')).toBeInTheDocument();
  });
});
