// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { WorkspaceDeprecationItemKind } from '@/api/v1/schema';
import { useIsAdmin } from '@/contexts/AuthContext';
import { UserPreferencesProvider } from '@/contexts/UserPreference';
import { I18nProvider } from '@/i18n/I18nProvider';
import { WorkspaceDeprecationNotice } from '../WorkspaceDeprecationNotice';

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

function renderNotice() {
  return render(
    <UserPreferencesProvider>
      <I18nProvider>
        <WorkspaceDeprecationNotice />
      </I18nProvider>
    </UserPreferencesProvider>
  );
}

describe('WorkspaceDeprecationNotice', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useIsAdminMock.mockReturnValue(true);
  });

  it('lists each workspace setting once an admin opens it', async () => {
    respondWith([
      {
        kind: WorkspaceDeprecationItemKind.baseConfig,
        name: '/dags/workspaces/billing/base.yaml',
      },
      { kind: WorkspaceDeprecationItemKind.user, name: 'alice' },
    ]);

    renderNotice();

    expect(screen.queryByText('alice')).not.toBeInTheDocument();

    await userEvent.click(
      screen.getByRole('button', { name: 'Workspaces are removed in v3.0' })
    );

    const dialog = screen.getByRole('dialog');
    expect(dialog).toHaveTextContent(
      'Base config /dags/workspaces/billing/base.yaml'
    );
    expect(dialog).toHaveTextContent('User alice');
    expect(
      screen.getByRole('link', {
        name: 'Have feedback? Comment on the GitHub issue.',
      })
    ).toHaveAttribute('href', 'https://github.com/dagucloud/dagu/issues/3288');
  });

  it('renders nothing when no workspace setting is in use', () => {
    respondWith([]);

    const { container } = renderNotice();

    expect(container).toBeEmptyDOMElement();
  });

  it('does not ask for the list for a non-admin', () => {
    useIsAdminMock.mockReturnValue(false);
    respondWith([]);

    const { container } = renderNotice();

    expect(container).toBeEmptyDOMElement();
    expect(useQueryMock).toHaveBeenCalledWith('/workspaces/deprecation', null);
  });
});
