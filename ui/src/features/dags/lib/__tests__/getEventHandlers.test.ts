import { describe, expect, it } from 'vitest';
import { getEventHandlers } from '../getEventHandlers';

describe('getEventHandlers', () => {
  it('renames onCancel lifecycle hook to onAbort for display', () => {
    const dagRun = {
      onCancel: {
        step: { name: 'onCancel' },
      },
    } as any;

    const handlers = getEventHandlers(dagRun);

    expect(handlers).toHaveLength(1);
    expect(handlers[0].step.name).toBe('onAbort');
    expect(dagRun.onCancel.step.name).toBe('onCancel');
  });

  it('preserves non-cancel handlers as-is', () => {
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
