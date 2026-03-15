import { describe, expect, it } from 'vitest';
import {
  base64ToBytes,
  binaryStringToBase64,
  stringToBase64,
} from '@/pages/terminal/encoding';

describe('terminal encoding helpers', () => {
  it('round-trips multi-byte UTF-8 output through base64ToBytes', () => {
    const value = '╭─────╮ 日本 🙂';
    const encoded = stringToBase64(value);
    const decoded = new TextDecoder().decode(base64ToBytes(encoded));

    expect(decoded).toBe(value);
  });

  it('stringToBase64 handles code points above U+00FF without throwing', () => {
    const value = 'hello 世界🙂';

    expect(() => stringToBase64(value)).not.toThrow();
    expect(new TextDecoder().decode(base64ToBytes(stringToBase64(value)))).toBe(
      value
    );
  });

  it('binaryStringToBase64 preserves raw byte values from xterm onBinary', () => {
    const rawBytes = new Uint8Array([0x00, 0x7f, 0x80, 0xff]);
    const binary = String.fromCharCode(...rawBytes);

    expect(base64ToBytes(binaryStringToBase64(binary))).toEqual(rawBytes);
  });
});
