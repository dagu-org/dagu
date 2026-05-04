// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react';
import fs from 'node:fs';
import path from 'node:path';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import AgentPage from '..';

const getMock = vi.fn();
const patchMock = vi.fn();
const updateConfigMock = vi.fn();

vi.mock('@/hooks/api', () => ({
  useClient: () => ({
    GET: getMock,
    PATCH: patchMock,
  }),
}));

vi.mock('@/contexts/ConfigContext', () => ({
  useUpdateConfig: () => updateConfigMock,
}));

function createAgentData(enabled = true) {
  return {
    enabled,
    defaultModelId: 'main',
    toolPolicy: {
      tools: {},
      bash: {
        rules: [],
      },
    },
    webSearch: { enabled: false },
  };
}

function renderPage(): void {
  render(
    <MemoryRouter>
      <AppBarContext.Provider
        value={{ setTitle: vi.fn(), selectedRemoteNode: 'local' } as never}
      >
        <AgentPage />
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  getMock.mockResolvedValue({ data: createAgentData(true) });
  patchMock.mockResolvedValue({ data: createAgentData(false) });
});

describe('AgentPage', () => {
  it('bundles agent links with the same sectioned homepage layout as administration', async () => {
    renderPage();

    const statusHeading = screen.getByRole('heading', { name: /status/i });
    const configurationHeading = screen.getByRole('heading', {
      name: /configuration/i,
    });
    const contextHeading = screen.getByRole('heading', { name: /context/i });
    const statusSection = statusHeading.closest('section');
    const configurationSection = configurationHeading.closest('section');
    const contextSection = contextHeading.closest('section');

    expect(screen.getByRole('heading', { name: /^agent$/i })).toBeVisible();
    expect(statusSection).not.toBeNull();
    expect(configurationSection).not.toBeNull();
    expect(contextSection).not.toBeNull();
    expect(statusHeading.nextElementSibling).toHaveClass(
      'grid',
      'gap-2',
      'md:grid-cols-2',
      'xl:grid-cols-3'
    );
    expect(configurationHeading.nextElementSibling).toHaveClass(
      'grid',
      'gap-2',
      'md:grid-cols-2',
      'xl:grid-cols-3'
    );
    expect(contextHeading.nextElementSibling).toHaveClass(
      'grid',
      'gap-2',
      'md:grid-cols-2',
      'xl:grid-cols-3'
    );
    expect(
      within(configurationSection as HTMLElement).getByRole('link', {
        name: /models/i,
      })
    ).toHaveAttribute('href', '/agent-settings');
    expect(
      within(configurationSection as HTMLElement).getByRole('link', {
        name: /tools/i,
      })
    ).toHaveAttribute('href', '/agent-tools');
    expect(
      within(contextSection as HTMLElement).getByRole('link', {
        name: /memory/i,
      })
    ).toHaveAttribute('href', '/agent-memory');
    expect(
      within(contextSection as HTMLElement).getByRole('link', {
        name: /souls/i,
      })
    ).toHaveAttribute('href', '/agent-souls');
    expect(await screen.findByLabelText('Enable Agent')).toBeVisible();
    expect(screen.getByText('Turn on the AI assistant feature.')).toBeVisible();
    expect(screen.getByText('Configure model access.')).toBeVisible();
    expect(
      screen.getByText('Configure web search and tool policy.')
    ).toBeVisible();
    expect(screen.getByText('Manage persistent context.')).toBeVisible();
    expect(screen.getByText('Manage reusable personas.')).toBeVisible();
  });

  it('saves the Enable Agent switch from the agent home page', async () => {
    renderPage();

    const enableSwitch = await screen.findByLabelText('Enable Agent');
    fireEvent.click(enableSwitch);

    await waitFor(() => {
      expect(patchMock).toHaveBeenCalledWith('/settings/agent', {
        params: { query: { remoteNode: 'local' } },
        body: { enabled: false },
      });
    });
    expect(updateConfigMock).toHaveBeenCalledWith({ agentEnabled: false });
  });

  it('does not let the legacy agent route redirect to workflow design', () => {
    const appSource = fs.readFileSync(
      path.resolve(__dirname, '../../../App.tsx'),
      'utf8'
    );

    expect(appSource).not.toMatch(
      /path="\/agent"[\s\S]*?<Navigate to="\/design" replace \/>/
    );
  });
});
