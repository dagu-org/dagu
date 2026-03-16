import { renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useSSEConnection } from '../useSSEConnection';

describe('useSSEConnection', () => {
  it('always returns isSessionLive=false (agent SSE disabled, polling fallback active)', () => {
    const { result } = renderHook(() =>
      useSSEConnection('sess-1', '/api/v1', 'local', {
        onEvent: () => {},
        onNavigate: () => {},
      })
    );

    expect(result.current.isSessionLive).toBe(false);
  });
});
