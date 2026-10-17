// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { act, fireEvent, render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { useStoredValue } from '../useStoredValue';

function Reader({ storageKey }: { storageKey: string }) {
  const [value, setValue] = useStoredValue(storageKey);
  return (
    <button type="button" onClick={() => setValue('dismissed')}>
      {value ?? 'unset'}
    </button>
  );
}

function values(): (string | null)[] {
  return screen.getAllByRole('button').map((button) => button.textContent);
}

describe('useStoredValue', () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it('updates every reader of the key at once', () => {
    render(
      <>
        <Reader storageKey="offer:local" />
        <Reader storageKey="offer:local" />
        <Reader storageKey="offer:edge" />
      </>
    );

    fireEvent.click(screen.getAllByRole('button')[0]!);

    expect(values()).toEqual(['dismissed', 'dismissed', 'unset']);
    expect(localStorage.getItem('offer:local')).toBe('dismissed');
  });

  it('follows a value stored in another tab', () => {
    render(<Reader storageKey="offer:local" />);

    act(() => {
      localStorage.setItem('offer:local', 'dismissed');
      window.dispatchEvent(new StorageEvent('storage', { key: 'offer:local' }));
    });

    expect(values()).toEqual(['dismissed']);
  });
});
