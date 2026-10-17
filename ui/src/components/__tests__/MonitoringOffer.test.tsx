// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MonitoringOffer } from '@/components/MonitoringOffer';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useIsAdmin } from '@/contexts/AuthContext';
import type { LicenseStatus } from '@/contexts/ConfigContext';
import { LicenseContext } from '@/contexts/LicenseContext';

vi.mock('@/contexts/AuthContext', () => ({ useIsAdmin: vi.fn() }));

const useIsAdminMock = vi.mocked(useIsAdmin);

const community: LicenseStatus = {
  valid: false,
  plan: '',
  expiry: '',
  features: [],
  gracePeriod: false,
  graceEndsAt: '',
  community: true,
  source: '',
  warningCode: '',
  error: '',
};

const heading = 'Get an email if this server stops';

function renderOffer(
  state: { license?: LicenseStatus; error?: unknown } = { license: community },
  remoteNode = 'local'
) {
  return render(
    <MemoryRouter>
      <AppBarContext.Provider
        value={{ selectedRemoteNode: remoteNode } as never}
      >
        <LicenseContext.Provider value={{ ...state, mutate: vi.fn() }}>
          <MonitoringOffer />
        </LicenseContext.Provider>
      </AppBarContext.Provider>
    </MemoryRouter>
  );
}

describe('MonitoringOffer', () => {
  beforeEach(() => {
    localStorage.clear();
    useIsAdminMock.mockReturnValue(true);
  });

  it('offers a Community admin free monitoring', () => {
    renderOffer();

    expect(screen.getByRole('region', { name: heading })).toBeVisible();
    expect(
      screen.getByRole('link', { name: 'Set up for free' })
    ).toHaveAttribute('href', '/license#monitoring');
  });

  it.each([
    ['a non-admin', false, { license: community }],
    [
      'a licensed server',
      true,
      {
        license: { ...community, community: false, valid: true, plan: 'team' },
      },
    ],
    ['a license still loading', true, {}],
    ['an unreadable license', true, { error: new Error('unavailable') }],
    [
      'an environment license',
      true,
      { license: { ...community, source: 'env' } },
    ],
  ])('stays hidden for %s', (_, admin, state) => {
    useIsAdminMock.mockReturnValue(admin);

    renderOffer(state);

    expect(screen.queryByText(heading)).not.toBeInTheDocument();
  });

  it('stays dismissed for that server only', () => {
    renderOffer();
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss' }));
    expect(screen.queryByText(heading)).not.toBeInTheDocument();
    cleanup();

    renderOffer();
    expect(screen.queryByText(heading)).not.toBeInTheDocument();
    cleanup();

    renderOffer(undefined, 'edge');
    expect(screen.getByText(heading)).toBeVisible();
  });
});
