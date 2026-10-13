// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Status, StatusLabel } from '@/api/v1/schema';
import { StatusChip } from '../status-chip';

describe('StatusChip', () => {
  it('shows API status labels as readable text', () => {
    render(
      <StatusChip status={Status.NotStarted}>
        {StatusLabel.not_started}
      </StatusChip>
    );

    expect(screen.getByText('Not started')).toBeVisible();
    expect(screen.queryByText('not_started')).toBeNull();
  });

  it('renders custom content unchanged', () => {
    render(
      <StatusChip status={Status.Failed}>
        <span>exit 3</span>
      </StatusChip>
    );

    expect(screen.getByText('exit 3')).toBeVisible();
  });
});
