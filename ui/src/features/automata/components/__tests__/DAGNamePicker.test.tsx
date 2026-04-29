// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

import { DAGNamePicker } from '@/features/automata/components/AutomataCreateModal';

describe('DAGNamePicker', () => {
  it('removes a selected DAG through an explicit remove button', () => {
    const onChange = vi.fn();

    render(
      <DAGNamePicker
        availableDAGs={[]}
        selectedNames={['build-app']}
        onChange={onChange}
      />
    );

    fireEvent.click(
      screen.getByRole('button', { name: 'Remove build-app' })
    );

    expect(onChange).toHaveBeenCalledWith([]);
  });
});
