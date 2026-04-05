import { describe, expect, it } from 'vitest';
import { sseFallbackOptions } from '../useSSECacheSync';
import { buildSSEEndpoint } from '../useSSE';

describe('buildSSEEndpoint', () => {
  it('serializes array query values as repeated parameters', () => {
    expect(
      buildSSEEndpoint('/events/dag-runs', {
        status: [5, 1],
        fromDate: 100,
      })
    ).toBe('/events/dag-runs?status=5&status=1&fromDate=100');
  });
});

describe('sseFallbackOptions', () => {
  it('disables polling when SSE is connected even before the first payload', () => {
    expect(
      sseFallbackOptions({
        data: null,
        error: null,
        isConnected: true,
        isConnecting: false,
        shouldUseFallback: false,
      })
    ).toMatchObject({
      revalidateIfStale: false,
      revalidateOnFocus: false,
      refreshInterval: 0,
    });
  });
});
