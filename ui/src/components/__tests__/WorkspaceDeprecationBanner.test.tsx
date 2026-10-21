// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkspaceDeprecationItemKind } from '@/api/v1/schema';
import { useIsAdmin } from '@/contexts/AuthContext';
import { UserPreferencesProvider } from '@/contexts/UserPreference';
import { I18nProvider } from '@/i18n/I18nProvider';
import { WorkspaceDeprecationBanner } from '../WorkspaceDeprecationBanner';

// The generated query hook's type is too deep to mock through vi.mocked.
const { useQueryMock } = vi.hoisted(() => ({ useQueryMock: vi.fn() }));
vi.mock('@/hooks/api', () => ({
  useQuery: useQueryMock,
}));

vi.mock('@/contexts/AuthContext', () => ({
  useIsAdmin: vi.fn(),
}));

vi.mock('@/contexts/RemoteNodeContext', () => ({
  useRemoteNode: () => 'local',
}));

const useIsAdminMock = vi.mocked(useIsAdmin);

function respondWith(
  items: { kind: WorkspaceDeprecationItemKind; name: string }[]
) {
  useQueryMock.mockReturnValue({ data: { items } });
}

function renderBanner() {
  return render(
    <UserPreferencesProvider>
      <I18nProvider>
        <WorkspaceDeprecationBanner />
      </I18nProvider>
    </UserPreferencesProvider>
  );
}

describe('WorkspaceDeprecationBanner', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useIsAdminMock.mockReturnValue(true);
  });

  it('lists each workspace setting for an admin', async () => {
    respondWith([
      {
        kind: WorkspaceDeprecationItemKind.baseConfig,
        name: '/dags/workspaces/billing/base.yaml',
      },
      { kind: WorkspaceDeprecationItemKind.user, name: 'alice' },
    ]);

    renderBanner();

    const banner = screen.getByRole('status');
    expect(banner).toHaveTextContent('Workspaces are removed in v3.0.');
    expect(banner).not.toHaveTextContent('alice');

    await userEvent.click(
      screen.getByRole('button', { name: 'Show 2 settings' })
    );

    expect(banner).toHaveTextContent(
      'Base config /dags/workspaces/billing/base.yaml'
    );
    expect(banner).toHaveTextContent('User alice');
  });

  it('renders nothing when no workspace setting is in use', () => {
    respondWith([]);

    const { container } = renderBanner();

    expect(container).toBeEmptyDOMElement();
  });

  it('does not ask for the list for a non-admin', () => {
    useIsAdminMock.mockReturnValue(false);
    respondWith([]);

    const { container } = renderBanner();

    expect(container).toBeEmptyDOMElement();
    expect(useQueryMock).toHaveBeenCalledWith('/workspaces/deprecation', null);
  });
});
