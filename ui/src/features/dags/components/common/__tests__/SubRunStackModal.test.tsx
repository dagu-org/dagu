// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useBoundedDAGRunDetails } from '@/features/dag-runs/hooks/useBoundedDAGRunDetails';
import { dispatchHandledEscape } from '@/test/keyboard';
import { SubRunStackModal } from '../SubRunStackModal';

vi.mock('@/features/dag-runs/hooks/useBoundedDAGRunDetails', () => ({
  useBoundedDAGRunDetails: vi.fn(),
}));

describe('SubRunStackModal', () => {
  it('pops a level on Escape unless a dialog inside it handled the key', () => {
    vi.mocked(useBoundedDAGRunDetails).mockReturnValue({
      data: undefined,
      isValidating: false,
      refresh: vi.fn(),
    } as unknown as ReturnType<typeof useBoundedDAGRunDetails>);
    const onChange = vi.fn();
    const stack = [
      { name: 'child', dagRunId: 'child-run' },
      { name: 'grandchild', dagRunId: 'grandchild-run' },
    ];
    render(
      <SubRunStackModal
        rootName="root"
        rootDAGRunId="root-run"
        rootLabel="root"
        stack={stack}
        onChange={onChange}
      />
    );

    dispatchHandledEscape();
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.keyDown(window, { key: 'Escape' });
    expect(onChange).toHaveBeenCalledWith(stack.slice(0, 1));
  });
});
