// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { Config } from '@/contexts/ConfigContext';
import { ConfigContext, ConfigUpdateContext } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useAuth } from '@/contexts/AuthContext';
import { useAgentAuthProviders } from '@/features/agent/hooks/useAgentAuthProviders';
import { useClient } from '@/hooks/api';
import SetupPage from '../setup';

const navigateMock = vi.fn();
const setupMock = vi.fn();
const completeSetupMock = vi.fn();
const updateConfigMock = vi.fn();
const getMock = vi.fn();
const patchMock = vi.fn();
const postMock = vi.fn();
const putMock = vi.fn();
const startLoginMock = vi.fn();
const completeLoginMock = vi.fn();
const disconnectMock = vi.fn();

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

const appBarContextValue = {
  title: '',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const presets = [
  {
    name: 'Claude Sonnet 4.6',
    provider: 'anthropic',
    model: 'claude-sonnet-4-6',
    description: 'Best balance of speed and intelligence.',
    contextWindow: 200_000,
    maxOutputTokens: 64_000,
    inputCostPer1M: 3,
    outputCostPer1M: 15,
    supportsThinking: true,
  },
  {
    name: 'GPT-5.4 Codex',
    provider: 'openai-codex',
    model: 'gpt-5.4',
    description: 'Latest Codex model via your ChatGPT Plus/Pro subscription.',
    contextWindow: 1_000_000,
    maxOutputTokens: 128_000,
    inputCostPer1M: 0,
    outputCostPer1M: 0,
    supportsThinking: true,
  },
];

function renderPage() {
  return render(
    <ConfigContext.Provider value={config}>
      <ConfigUpdateContext.Provider value={updateConfigMock}>
        <AppBarContext.Provider value={appBarContextValue}>
          <SetupPage />
        </AppBarContext.Provider>
      </ConfigUpdateContext.Provider>
    </ConfigContext.Provider>
  );
}

async function moveToAgentStep() {
  fireEvent.change(screen.getByLabelText('Username'), {
    target: { value: 'admin-user' },
  });
  fireEvent.change(screen.getByLabelText('Password'), {
    target: { value: 'password123' },
  });
  fireEvent.change(screen.getByLabelText('Confirm Password'), {
    target: { value: 'password123' },
  });

  fireEvent.click(screen.getByRole('button', { name: 'Continue' }));

  await screen.findByText('Enable AI Agent');
  await waitFor(() => {
    expect(getMock).toHaveBeenCalledWith('/settings/agent/model-presets', {
      params: { query: { remoteNode: 'local' } },
    });
  });
}

beforeEach(() => {
  navigateMock.mockReset();
  setupMock.mockReset();
  completeSetupMock.mockReset();
  updateConfigMock.mockReset();
  getMock.mockReset();
  patchMock.mockReset();
  postMock.mockReset();
  putMock.mockReset();
  startLoginMock.mockReset();
  completeLoginMock.mockReset();
  disconnectMock.mockReset();

  setupMock.mockResolvedValue({
    token: 'token-1',
    user: { id: '1', username: 'admin-user', role: 'admin' },
  });
  getMock.mockResolvedValue({ data: { presets } });
  patchMock.mockResolvedValue({});
  postMock.mockResolvedValue({ data: { id: 'created-model' } });
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

  useAgentAuthProvidersMock.mockReturnValue({
    providers: [],
    providerMap: {
      'openai-codex': {
        id: 'openai-codex',
        name: 'OpenAI Codex',
        connected: false,
        expiresAt: '',
        accountId: '',
      },
    },
    remoteNode: 'local',
    isLoading: false,
    error: null,
    refreshProviders: vi.fn().mockResolvedValue([]),
    startLogin: startLoginMock,
    completeLogin: completeLoginMock,
    disconnect: disconnectMock,
  });
});

afterEach(() => {
  cleanup();
});

describe('SetupPage', () => {
  it('allows local onboarding without an API key and omits blank credentials', async () => {
    renderPage();
    await moveToAgentStep();

    fireEvent.click(screen.getByRole('button', { name: 'Local' }));

    expect(screen.getByText('API Key (optional)')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Leave empty for local endpoints that do not require authentication.'
      )
    ).toBeInTheDocument();
    expect(
      screen.getByPlaceholderText('Defaults to http://localhost:11434/v1')
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText('Model'), {
      target: { value: 'llama3.2' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Complete Setup' }));

    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(1));

    expect(patchMock).toHaveBeenCalledWith('/settings/agent', {
      params: { query: { remoteNode: 'local' } },
      body: { enabled: true },
    });
    expect(postMock).toHaveBeenCalledWith('/settings/agent/models', {
      params: { query: { remoteNode: 'local' } },
      body: expect.objectContaining({
        id: 'llama3-2',
        name: 'llama3.2',
        provider: 'local',
        model: 'llama3.2',
        supportsThinking: false,
      }),
    });
    const createBody = postMock.mock.calls[0]![1]!.body as Record<
      string,
      unknown
    >;
    expect(createBody.apiKey).toBeUndefined();
    expect(createBody.baseUrl).toBeUndefined();
    expect(putMock).toHaveBeenCalledWith('/settings/agent/default-model', {
      params: { query: { remoteNode: 'local' } },
      body: { modelId: 'created-model' },
    });
    expect(updateConfigMock).toHaveBeenCalledWith({ agentEnabled: true });
    expect(navigateMock).toHaveBeenCalledWith('/', {
      replace: true,
      state: { openAgent: true },
    });
  });

  it('requires an API key for manual OpenRouter setup', async () => {
    renderPage();
    await moveToAgentStep();

    fireEvent.click(screen.getByRole('button', { name: 'OpenRouter' }));
    fireEvent.change(screen.getByLabelText('Model'), {
      target: { value: 'anthropic/claude-sonnet-4-6' },
    });

    fireEvent.click(screen.getByRole('button', { name: 'Complete Setup' }));

    expect(
      await screen.findByText('Please enter an API key')
    ).toBeInTheDocument();
    expect(postMock).not.toHaveBeenCalled();
  });

  it('keeps preset selection and subscription auth for OpenAI Codex', async () => {
    renderPage();
    await moveToAgentStep();

    fireEvent.click(screen.getByRole('button', { name: 'OpenAI Codex' }));

    expect(screen.queryByLabelText('API Key')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Connect' })).toBeInTheDocument();
    expect(screen.getByText('Select a model...')).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Complete Setup' })
    ).toBeDisabled();
    expect(postMock).not.toHaveBeenCalled();
  });
});
