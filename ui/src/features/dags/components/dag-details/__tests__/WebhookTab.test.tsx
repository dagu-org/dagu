// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import WebhookTab from '../WebhookTab';

vi.mock('../../../../../contexts/ConfigContext', () => ({
  useConfig: () => ({ apiURL: '/api/v1' }),
}));

vi.mock('../../../../../contexts/RemoteNodeContext', () => ({
  useRemoteNode: () => 'local',
}));

vi.mock('../../../../../contexts/AuthContext', () => ({
  TOKEN_KEY: 'dagu_auth_token',
  useIsAdmin: () => true,
}));

vi.mock('../../../../../hooks/api', () => ({
  useClient: () => ({ GET: vi.fn(), POST: vi.fn() }),
}));

vi.mock('../WebhookProfileSelectionCard', () => ({ default: () => null }));
vi.mock('../WebhookProfileTokensCard', () => ({ default: () => null }));

const webhook = {
  id: '00000000-0000-0000-0000-000000000001',
  dagName: 'deploy',
  tokenPrefix: 'dagu_wh_',
  enabled: true,
  authMode: 'token_only',
  hmac: { enabled: false, secretConfigured: false },
  profileSelection: { allowedProfiles: [] },
  createdAt: '2026-10-01T00:00:00Z',
  updatedAt: '2026-10-01T00:00:00Z',
};

describe('WebhookTab', () => {
  const fetchMock = vi.fn();

  beforeEach(() => {
    fetchMock.mockImplementation((url: string, init?: RequestInit) => {
      if (init?.method === 'POST') {
        return Promise.resolve(
          new Response(JSON.stringify({ webhook, token: 'new-token' }))
        );
      }
      if (url.includes('/webhook?')) {
        return Promise.resolve(new Response(JSON.stringify(webhook)));
      }
      return Promise.resolve(new Response('{}', { status: 404 }));
    });
    vi.stubGlobal('fetch', fetchMock);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    fetchMock.mockReset();
  });

  it('confirms before regenerating the webhook token', async () => {
    render(<WebhookTab fileName="deploy" />);

    fireEvent.click(
      await screen.findByRole('button', { name: 'Regenerate token' })
    );

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent(
      'Callers using the current token lose access'
    );
    expect(
      fetchMock.mock.calls.some(([, init]) => init?.method === 'POST')
    ).toBe(false);

    fireEvent.click(within(dialog).getByRole('button', { name: 'Regenerate' }));

    expect(
      await screen.findByText('new-token', undefined, { timeout: 2000 })
    ).toBeVisible();
    expect(fetchMock).toHaveBeenCalledWith(
      '/api/v1/dags/deploy/webhook/regenerate?remoteNode=local',
      expect.objectContaining({ method: 'POST' })
    );
  });
});
