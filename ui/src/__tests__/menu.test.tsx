// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { UserRole } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import { mainListItems as MainListItems } from '../menu';
import { defaultWorkspaceSelection } from '@/lib/workspace';

const useAuthMock = vi.fn();
const useIsAdminMock = vi.fn();
const useCanAccessSystemStatusMock = vi.fn();
const useCanViewEventLogsMock = vi.fn();
const useCanManageWebhooksMock = vi.fn();
const useCanViewAuditLogsMock = vi.fn();
const useHasFeatureMock = vi.fn();
const updatePreferenceMock = vi.fn();
const toggleChatMock = vi.fn();

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: () => useAuthMock(),
  useIsAdmin: () => useIsAdminMock(),
  useCanAccessSystemStatus: () => useCanAccessSystemStatusMock(),
  useCanViewEventLogs: () => useCanViewEventLogsMock(),
  useCanManageWebhooks: () => useCanManageWebhooksMock(),
  useCanViewAuditLogs: () => useCanViewAuditLogsMock(),
}));

vi.mock('@/hooks/useLicense', () => ({
  useHasFeature: (feature: string) => useHasFeatureMock(feature),
}));

vi.mock('../contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: { theme: 'dark' },
    updatePreference: updatePreferenceMock,
  }),
}));

vi.mock('../features/agent', () => ({
  useAgentChatContext: () => ({ toggleChat: toggleChatMock }),
}));

const config: Config = {
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
  authMode: 'builtin',
  setupRequired: false,
  oidcEnabled: false,
  oidcButtonLabel: '',
  terminalEnabled: true,
  gitSyncEnabled: true,
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
    features: ['audit', 'rbac'],
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
};

function renderMenu(
  initialEntry = '/cockpit',
  configOverride: Partial<Config> = {}
): void {
  render(
    <MemoryRouter initialEntries={[initialEntry]}>
      <ConfigContext.Provider value={{ ...config, ...configOverride }}>
        <AppBarContext.Provider
          value={{
            title: '',
            setTitle: vi.fn(),
            remoteNodes: ['local'],
            setRemoteNodes: vi.fn(),
            selectedRemoteNode: 'local',
            selectRemoteNode: vi.fn(),
            workspaces: [],
            workspaceError: null,
            workspaceSelection: defaultWorkspaceSelection(),
            selectWorkspace: vi.fn(),
            createWorkspace: vi.fn(),
            deleteWorkspace: vi.fn(),
          }}
        >
          <MainListItems isOpen />
        </AppBarContext.Provider>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  localStorage.clear();
  useAuthMock.mockReturnValue({
    user: { id: '1', username: 'admin', role: UserRole.admin },
  });
  useIsAdminMock.mockReturnValue(true);
  useCanAccessSystemStatusMock.mockReturnValue(true);
  useCanViewEventLogsMock.mockReturnValue(true);
  useCanManageWebhooksMock.mockReturnValue(true);
  useCanViewAuditLogsMock.mockReturnValue(true);
  useHasFeatureMock.mockReturnValue(true);
});

describe('sidebar menu', () => {
  it('organizes navigation into multi-level operational accordions', () => {
    renderMenu();

    expect(screen.getByRole('link', { name: /overview/i })).toHaveAttribute(
      'href',
      '/'
    );
    expect(screen.getByRole('link', { name: /overview/i })).toHaveAttribute(
      'aria-current',
      'page'
    );
    expect(
      screen.queryByRole('button', { name: /toggle overview section/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /timeline/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /cockpit/i })
    ).not.toBeInTheDocument();
    expect(screen.getByRole('link', { name: /workflows/i })).toHaveAttribute(
      'href',
      '/dags'
    );
    expect(
      screen.getByRole('button', { name: /toggle workflows section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(
      screen.queryByRole('link', { name: /definitions/i })
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole('link', { name: /workflows/i }).querySelector('svg')
    ).not.toBeNull();
    expect(screen.getByRole('link', { name: /^executions$/i })).toHaveAttribute(
      'href',
      '/dag-runs'
    );
    expect(
      screen.getByRole('button', { name: /toggle executions section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByRole('link', { name: /monitor/i })).toHaveAttribute(
      'href',
      '/system-status'
    );
    expect(
      screen.getByRole('button', { name: /toggle monitor section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(screen.getByRole('link', { name: /integrations/i })).toHaveAttribute(
      'href',
      '/integrations'
    );
    expect(
      screen.getByRole('button', { name: /toggle integrations section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(
      screen.getByRole('link', { name: /administration/i })
    ).toHaveAttribute('href', '/administration');
    expect(
      screen.getByRole('button', { name: /toggle administration section/i })
    ).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(screen.getByRole('link', { name: /workflows/i }));
    expect(
      screen.getByRole('button', { name: /toggle workflows section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(
      screen.getByRole('button', { name: /toggle workflows section/i })
    );
    expect(
      screen.getByRole('button', { name: /toggle workflows section/i })
    ).toHaveAttribute('aria-expanded', 'true');
    expect(screen.queryByRole('link', { name: /definitions/i })).toBeNull();
    expect(screen.getByRole('link', { name: /git sync/i })).toBeVisible();

    fireEvent.click(
      screen.getByRole('button', { name: /toggle executions section/i })
    );
    expect(screen.queryByRole('link', { name: /^runs$/i })).toBeNull();
    const executionSubmenuItems = [
      screen.getByRole('link', { name: /queues/i }),
    ];
    for (const item of executionSubmenuItems) {
      expect(item).toBeVisible();
      expect(item.querySelector('svg')).toBeNull();
    }

    fireEvent.click(
      screen.getByRole('button', { name: /toggle monitor section/i })
    );
    expect(
      screen.queryByRole('link', { name: /system status/i })
    ).not.toBeInTheDocument();
    const monitorSubmenuItems = [
      screen.getByRole('link', { name: /events/i }),
      screen.getByRole('link', { name: /audit logs/i }),
    ];
    for (const item of monitorSubmenuItems) {
      expect(item).toBeVisible();
      expect(item.querySelector('svg')).toBeNull();
    }

    const workflowSubmenuItems = [
      screen.getByRole('link', { name: /search/i }),
      screen.getByRole('link', { name: /base config/i }),
      screen.getByRole('link', { name: /runbooks/i }),
      screen.getByRole('link', { name: /git sync/i }),
    ];
    for (const item of workflowSubmenuItems) {
      expect(item).toBeVisible();
      expect(item.querySelector('svg')).toBeNull();
    }

    fireEvent.click(
      screen.getByRole('button', { name: /toggle integrations section/i })
    );
    const integrationSubmenuItems = [
      screen.getByRole('link', { name: /webhooks/i }),
      screen.getByRole('link', { name: /api reference/i }),
    ];
    for (const item of integrationSubmenuItems) {
      expect(item).toBeVisible();
      expect(item.querySelector('svg')).toBeNull();
    }

    fireEvent.click(
      screen.getByRole('button', { name: /toggle administration section/i })
    );
    const accessSection = screen.getByRole('button', {
      name: /access section/i,
    });
    const infrastructureSection = screen.getByRole('button', {
      name: /infrastructure section/i,
    });
    expect(accessSection).toBeVisible();
    expect(
      accessSection.querySelector('svg:not(.lucide-chevron-down)')
    ).toBeNull();
    expect(infrastructureSection).toBeVisible();
    expect(
      infrastructureSection.querySelector('svg:not(.lucide-chevron-down)')
    ).toBeNull();

    expect(screen.getByRole('link', { name: /agent/i })).toHaveAttribute(
      'href',
      '/agent'
    );
    expect(
      screen.getByRole('button', { name: /toggle agent section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(
      screen.getByRole('button', { name: /toggle agent section/i })
    );
    const agentSubmenuItems = [
      screen.getByRole('link', { name: /models/i }),
      screen.getByRole('link', { name: /tools/i }),
      screen.getByRole('link', { name: /memory/i }),
      screen.getByRole('link', { name: /souls/i }),
    ];
    for (const item of agentSubmenuItems) {
      expect(item).toBeVisible();
      expect(item.querySelector('svg')).toBeNull();
    }
    expect(screen.getByRole('link', { name: /models/i })).toHaveAttribute(
      'href',
      '/agent-settings'
    );
    expect(screen.getByRole('link', { name: /tools/i })).toHaveAttribute(
      'href',
      '/agent-tools'
    );

    expect(
      screen.queryByRole('link', { name: /^docs$/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /api docs/i })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: /^dashboard$/i })
    ).not.toBeInTheDocument();
  });

  it('uses Workflows as the selectable Definitions entry', () => {
    renderMenu('/dags');

    expect(screen.getByRole('link', { name: /workflows/i })).toHaveAttribute(
      'href',
      '/dags'
    );
    expect(screen.getByRole('link', { name: /workflows/i })).toHaveAttribute(
      'aria-current',
      'page'
    );
    expect(
      screen.queryByRole('link', { name: /definitions/i })
    ).not.toBeInTheDocument();
  });

  it.each([
    ['/dag-runs', 'executions'],
    ['/system-status', 'monitor'],
    ['/integrations', 'integrations'],
    ['/administration', 'administration'],
  ])('marks %s as the active section entry', (path, label) => {
    renderMenu(path);

    expect(
      screen.getByRole('link', { name: new RegExp(label, 'i') })
    ).toHaveAttribute('aria-current', 'page');
  });

  it.each([
    ['/git-sync', 'workflows'],
    ['/queues', 'executions'],
    ['/event-logs', 'monitor'],
    ['/webhooks', 'integrations'],
    ['/users', 'administration'],
  ])('does not auto-expand %s inside %s', (path, label) => {
    renderMenu(path);

    expect(
      screen.getByRole('button', {
        name: new RegExp(`toggle ${label} section`, 'i'),
      })
    ).toHaveAttribute('aria-expanded', 'false');
  });

  it('does not auto-expand Administration nested groups when opened', () => {
    localStorage.setItem('navgroup_expanded_administration-access', 'true');
    localStorage.setItem(
      'navgroup_expanded_administration-infrastructure',
      'true'
    );

    renderMenu('/administration');

    fireEvent.click(
      screen.getByRole('button', { name: /toggle administration section/i })
    );

    expect(
      screen.getByRole('button', { name: /access section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(
      screen.getByRole('button', { name: /infrastructure section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    expect(
      screen.getByRole('button', { name: /toggle agent section/i })
    ).toHaveAttribute('aria-expanded', 'false');
  });

  it('keeps agent settings reachable when agent is disabled', () => {
    renderMenu('/administration', { agentEnabled: false });

    fireEvent.click(
      screen.getByRole('button', { name: /toggle administration section/i })
    );
    expect(screen.getByRole('link', { name: /agent/i })).toHaveAttribute(
      'href',
      '/agent'
    );
    expect(
      screen.getByRole('button', { name: /toggle agent section/i })
    ).toHaveAttribute('aria-expanded', 'false');
    fireEvent.click(
      screen.getByRole('button', { name: /toggle agent section/i })
    );
    expect(screen.getByRole('link', { name: /models/i })).toHaveAttribute(
      'href',
      '/agent-settings'
    );
    expect(screen.getByRole('link', { name: /tools/i })).toHaveAttribute(
      'href',
      '/agent-tools'
    );
    expect(screen.getByRole('link', { name: /memory/i })).toBeVisible();
    expect(screen.getByRole('link', { name: /souls/i })).toBeVisible();
  });
});
