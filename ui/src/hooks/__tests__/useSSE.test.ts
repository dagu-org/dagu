import { describe, expect, it } from 'vitest';
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
