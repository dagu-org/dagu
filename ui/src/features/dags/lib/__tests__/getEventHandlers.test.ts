import { describe, expect, it } from 'vitest';
import { getEventHandlers } from '../getEventHandlers';

describe('getEventHandlers', () => {
  it('includes the onAbort lifecycle hook as-is', () => {
    const dagRun = {
      onAbort: {
        step: { name: 'onAbort' },
      },
    } as any;

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
    } as any;

    const handlers = getEventHandlers(dagRun);

    expect(handlers.map((h: any) => h.step.name)).toEqual([
      'onSuccess',
      'onFailure',
      'onExit',
    ]);
  });
});
