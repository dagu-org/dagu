// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { components } from '@/api/v1/schema';
import { dispatchHandledEscape } from '@/test/keyboard';
import { StepDetailsDrawer } from '../StepDetailsDrawer';

// The drawer's log sections import the API client, which needs runtime config.
vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(() => ({ data: undefined })),
}));

type Step = components['schemas']['Step'];

describe('StepDetailsDrawer', () => {
  it('closes on Escape unless a dialog inside it handled the key', () => {
    const onClose = vi.fn();
    render(
      <StepDetailsDrawer
        isOpen
        onClose={onClose}
        step={{ name: 'build' } as Step}
      />
    );

    dispatchHandledEscape();
    expect(onClose).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onClose).toHaveBeenCalledTimes(1);
  });
});
