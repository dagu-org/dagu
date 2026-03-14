// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect } from 'vitest';
import { parseParams, stringifyParams, Parameter } from '../parseParams';

describe('parseParams', () => {
  it('parses basic named param', () => {
    expect(parseParams('key=value')).toEqual([{ Name: 'key', Value: 'value' }]);
  });

  it('parses quoted value', () => {
    expect(parseParams('msg="hello world"')).toEqual([
      { Name: 'msg', Value: 'hello world' },
    ]);
  });

  it('parses escaped quotes', () => {
    expect(parseParams('msg="say \\"hello\\""')).toEqual([
      { Name: 'msg', Value: 'say "hello"' },
    ]);
  });

  it('parses multiline escape sequence', () => {
    expect(parseParams('msg="line1\\nline2"')).toEqual([
      { Name: 'msg', Value: 'line1\nline2' },
    ]);
  });

  it('parses escaped backslash', () => {
    expect(parseParams('path="C:\\\\Users"')).toEqual([
      { Name: 'path', Value: 'C:\\Users' },
    ]);
  });

  it('parses literal backslash-n (not newline)', () => {
    expect(parseParams('val="a\\\\nb"')).toEqual([
      { Name: 'val', Value: 'a\\nb' },
    ]);
  });

  it('parses tab escape', () => {
    expect(parseParams('data="col1\\tcol2"')).toEqual([
      { Name: 'data', Value: 'col1\tcol2' },
    ]);
  });

  it('parses positional param', () => {
    expect(parseParams('"hello"')).toEqual([{ Value: 'hello' }]);
  });

  it('parses multiple params', () => {
    expect(parseParams('a=1 b=2')).toEqual([
      { Name: 'a', Value: '1' },
      { Name: 'b', Value: '2' },
    ]);
  });

  it('passes through literal LF bytes unchanged (backward compat)', () => {
    const input = 'msg="line1\nline2"';
    const result = parseParams(input);
    // The literal LF in the input passes through unescapeValue unchanged
    expect(result[0]!.Value).toContain('\n');
  });
});

describe('stringifyParams', () => {
  it('produces JSON array with objects for named params', () => {
    const params: Parameter[] = [{ Name: 'key', Value: 'val' }];
    const result = stringifyParams(params);
    expect(result).toBe('[{"key":"val"}]');
    expect(JSON.parse(result)).toEqual([{ key: 'val' }]);
  });

  it('handles multiline value with JSON escaping', () => {
    const params: Parameter[] = [{ Name: 'MSG', Value: 'line1\nline2' }];
    const result = stringifyParams(params);
    expect(result).toContain('\\n');
    const parsed = JSON.parse(result);
    expect(parsed[0].MSG).toBe('line1\nline2');
  });

  it('produces JSON array with strings for positional params', () => {
    const params: Parameter[] = [{ Value: 'val1' }];
    const result = stringifyParams(params);
    expect(result).toBe('["val1"]');
  });

  it('handles mixed named and positional params', () => {
    const params: Parameter[] = [
      { Name: 'MSG', Value: 'hello' },
      { Value: 'positional' },
    ];
    const result = stringifyParams(params);
    expect(JSON.parse(result)).toEqual([{ MSG: 'hello' }, 'positional']);
  });

  it('returns empty string for empty params array', () => {
    expect(stringifyParams([])).toBe('');
  });

  it('normalizes CR+LF to LF', () => {
    const params: Parameter[] = [{ Name: 'MSG', Value: 'line1\r\nline2' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].MSG).toBe('line1\nline2');
  });

  it('properly escapes literal backslash in value', () => {
    const params: Parameter[] = [{ Name: 'path', Value: 'C:\\Users' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].path).toBe('C:\\Users');
  });
});

describe('edge cases', () => {
  it('handles value containing only newline', () => {
    const params: Parameter[] = [{ Name: 'x', Value: '\n' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].x).toBe('\n');
  });

  it('handles value containing double quote', () => {
    const params: Parameter[] = [{ Name: 'x', Value: 'say "hi"' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].x).toBe('say "hi"');
  });

  it('handles value containing literal backslash-n (not newline)', () => {
    const params: Parameter[] = [{ Name: 'x', Value: 'a\\nb' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].x).toBe('a\\nb');
  });

  it('handles value containing JSON-like characters', () => {
    const params: Parameter[] = [{ Name: 'x', Value: '{"key": [1,2]}' }];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].x).toBe('{"key": [1,2]}');
  });

  it('handles unicode characters in values', () => {
    const params: Parameter[] = [
      { Name: 'x', Value: 'hello \u00e9\u00e8\u00ea' },
    ];
    const result = stringifyParams(params);
    const parsed = JSON.parse(result);
    expect(parsed[0].x).toBe('hello \u00e9\u00e8\u00ea');
  });
});
