import { describe, expect, it } from 'vitest';
import { whenEnabled } from '../queryUtils';

describe('whenEnabled', () => {
  it('returns the init object when enabled', () => {
    const init = { params: { query: { remoteNode: 'local' } } };

    expect(whenEnabled(true, init)).toBe(init);
  });

  it('returns null when disabled', () => {
    expect(whenEnabled(false, { params: { query: { remoteNode: 'local' } } })).toBeNull();
  });
});
