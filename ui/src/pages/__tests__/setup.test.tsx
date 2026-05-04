// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Config } from '@/contexts/ConfigContext';
import { ConfigContext } from '@/contexts/ConfigContext';
import { useAuth } from '@/contexts/AuthContext';
import { useAgentAuthProviders } from '@/features/agent/hooks/useAgentAuthProviders';
import { useClient } from '@/hooks/api';
import SetupPage from '../setup';

const navigateMock = vi.fn();
const setupMock = vi.fn();
const completeSetupMock = vi.fn();
const getMock = vi.fn();
const patchMock = vi.fn();
const postMock = vi.fn();
const putMock = vi.fn();

vi.mock('react-router-dom', () => ({
  useNavigate: () => navigateMock,
}));

vi.mock('@/contexts/AuthContext', () => ({
  useAuth: vi.fn(),
}));

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
}));

vi.mock('@/features/agent/hooks/useAgentAuthProviders', () => ({
  useAgentAuthProviders: vi.fn(),
}));

const useAuthMock = vi.mocked(useAuth);
const useClientMock = vi.mocked(useClient);
const useAgentAuthProvidersMock = vi.mocked(useAgentAuthProviders);

const config: Config = {
  apiURL: '/api/v1',
  basePath: '/',
  title: 'Dagu',
  navbarColor: '',
  tz: 'UTC',
  tzOffsetInSec: 0,
  version: 'test',
  maxDashboardPageLimit: 100,
  remoteNodes: '',
  initialWorkspaces: [],
  authMode: 'builtin',
  setupRequired: true,
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
};

function renderPage() {
  return render(
    <ConfigContext.Provider value={config}>
      <SetupPage />
    </ConfigContext.Provider>
  );
}

beforeEach(() => {
  navigateMock.mockReset();
  setupMock.mockReset();
  completeSetupMock.mockReset();
  useClientMock.mockReset();
  useAgentAuthProvidersMock.mockReset();
  getMock.mockReset();
  patchMock.mockReset();
  postMock.mockReset();
  putMock.mockReset();

  setupMock.mockResolvedValue({
    token: 'token-1',
    user: { id: '1', username: 'admin-user', role: 'admin' },
  });
  getMock.mockResolvedValue({ data: {} });
  patchMock.mockResolvedValue({});
  postMock.mockResolvedValue({ data: {} });
  putMock.mockResolvedValue({});

  useAuthMock.mockReturnValue({
    user: null,
    token: null,
    isAuthenticated: false,
    isLoading: false,
    setupRequired: true,
    login: vi.fn(),
    setup: setupMock,
    logout: vi.fn(),
    refreshUser: vi.fn(),
    completeSetup: completeSetupMock,
  });

  useClientMock.mockReturnValue({
    GET: getMock,
    PATCH: patchMock,
    POST: postMock,
    PUT: putMock,
  } as never);
});

afterEach(() => {
  cleanup();
});

describe('SetupPage', () => {
  it('completes onboarding after creating the admin account without showing agent setup', async () => {
    renderPage();

    fireEvent.change(screen.getByLabelText('Username'), {
      target: { value: 'admin-user' },
    });
    fireEvent.change(screen.getByLabelText('Password'), {
      target: { value: 'password123' },
    });
    fireEvent.change(screen.getByLabelText('Confirm Password'), {
      target: { value: 'password123' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Create account' }));

    await waitFor(() => {
      expect(setupMock).toHaveBeenCalledWith('admin-user', 'password123');
    });

    expect(completeSetupMock).toHaveBeenCalledWith({
      token: 'token-1',
      user: { id: '1', username: 'admin-user', role: 'admin' },
    });
    expect(navigateMock).toHaveBeenCalledWith('/', { replace: true });
    expect(screen.queryByText('Enable AI Agent')).not.toBeInTheDocument();
    expect(useClientMock).not.toHaveBeenCalled();
    expect(useAgentAuthProvidersMock).not.toHaveBeenCalled();
    expect(getMock).not.toHaveBeenCalled();
    expect(patchMock).not.toHaveBeenCalled();
    expect(postMock).not.toHaveBeenCalled();
    expect(putMock).not.toHaveBeenCalled();
  });
});
