// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import OverviewPage from '..';

vi.mock('../..', () => ({
  default: () => <div>Timeline panel</div>,
}));

vi.mock('../../cockpit', () => ({
  default: () => <div>Cockpit panel</div>,
}));

describe('OverviewPage', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('defaults to the Timeline tab', () => {
    render(<OverviewPage />);

    expect(screen.getByRole('tab', { name: /timeline/i })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('Timeline panel')).toBeInTheDocument();
    expect(screen.queryByText('Cockpit panel')).not.toBeInTheDocument();
  });

  it('persists the last selected tab', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<OverviewPage />);

    await user.click(screen.getByRole('tab', { name: /cockpit/i }));

    expect(screen.getByRole('tab', { name: /cockpit/i })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(localStorage.getItem('dagu_overview_active_tab')).toBe('cockpit');

    unmount();
    render(<OverviewPage />);

    expect(screen.getByRole('tab', { name: /cockpit/i })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('Cockpit panel')).toBeInTheDocument();
  });

  it('honors an explicit route tab over stored state', () => {
    localStorage.setItem('dagu_overview_active_tab', 'cockpit');

    render(<OverviewPage initialTab="timeline" />);

    expect(screen.getByRole('tab', { name: /timeline/i })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('Timeline panel')).toBeInTheDocument();
  });
});
