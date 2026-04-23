import { describe, expect, it } from 'vitest';
import type { components } from '@/api/v1/schema';
import { getEventHandlers } from '../getEventHandlers';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

describe('getEventHandlers', () => {
  it('includes the onAbort lifecycle hook as-is', () => {
    const dagRun = {
      onAbort: {
        step: { name: 'onAbort' },
      },
    } as DAGRunDetails;

    const handlers = getEventHandlers(dagRun);
    const handler = handlers[0]!;

    expect(handlers).toHaveLength(1);
    expect(handler.step.name).toBe('onAbort');
    expect(handler).toBe(dagRun.onAbort);
  });

  it('preserves non-abort handlers as-is', () => {
    const dagRun = {
      onSuccess: {
        step: { name: 'onSuccess' },
      },
      onFailure: {
        step: { name: 'onFailure' },
      },
      onExit: {
        step: { name: 'onExit' },
      },
    } as DAGRunDetails;

    const handlers = getEventHandlers(dagRun);

    expect(handlers.map((h) => h.step.name)).toEqual([
      'onSuccess',
      'onFailure',
      'onExit',
    ]);
  });
});
