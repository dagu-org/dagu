// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { MemoryRouter } from 'react-router-dom';
import { describe, expect, it, vi } from 'vitest';
import { ConfigContext, type Config } from '@/contexts/ConfigContext';
import Layout from '../Layout';

vi.mock('@/components/LicenseBanner', () => ({
  LicenseBanner: () => null,
}));

vi.mock('@/components/UpdateBanner', () => ({
  UpdateBanner: () => null,
}));

vi.mock('@/features/agent', () => ({
  useAgentChatContext: () => ({ toggleChat: vi.fn() }),
}));

vi.mock('../../menu', () => ({
  mainListItems: () => <div data-testid="sidebar-menu">Sidebar</div>,
}));

const config = {
  title: 'Dagu',
  navbarColor: '',
  agentEnabled: true,
} as Config;

function renderLayout(path: string): void {
  render(
    <MemoryRouter initialEntries={[path]}>
      <ConfigContext.Provider value={config}>
        <Layout>
          <div>Page Content</div>
        </Layout>
      </ConfigContext.Provider>
    </MemoryRouter>
  );
}

describe('Layout', () => {
  it('keeps the app sidebar visible on the agent home page', () => {
    renderLayout('/agent');

    expect(screen.getByTestId('sidebar-menu')).toBeVisible();
    expect(screen.getByText('Page Content')).toBeVisible();
  });

  it('keeps workflow design fullscreen without the app sidebar', () => {
    renderLayout('/design');

    expect(screen.queryByTestId('sidebar-menu')).toBeNull();
    expect(screen.getByText('Page Content')).toBeVisible();
  });
});
