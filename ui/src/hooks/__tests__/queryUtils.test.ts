import { describe, expect, it } from 'vitest';
import { optionalPositiveInt, whenEnabled } from '../queryUtils';

describe('whenEnabled', () => {
  it('returns the init object when enabled', () => {
    const init = { params: { query: { remoteNode: 'local' } } };

    expect(whenEnabled(true, init)).toBe(init);
  });

  it('returns null when disabled', () => {
    expect(
      whenEnabled(false, { params: { query: { remoteNode: 'local' } } })
    ).toBeNull();
  });
});

describe('optionalPositiveInt', () => {
  it('returns a positive integer number', () => {
    expect(optionalPositiveInt(100)).toBe(100);
  });

  it('parses a positive integer string', () => {
    expect(optionalPositiveInt('100')).toBe(100);
  });

  it('omits blank strings', () => {
    expect(optionalPositiveInt('')).toBeUndefined();
  });

  it('omits invalid values', () => {
    expect(optionalPositiveInt('abc')).toBeUndefined();
    expect(optionalPositiveInt(0)).toBeUndefined();
    expect(optionalPositiveInt(-1)).toBeUndefined();
  });
});
