// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { validateDocPath } from '../doc-validation';

describe('validateDocPath', () => {
  it('accepts document path segments that start with underscores', () => {
    expect(validateDocPath('_index')).toEqual({ isValid: true });
    expect(validateDocPath('guides/_partial')).toEqual({ isValid: true });
  });

  it('continues to reject hidden dot files', () => {
    expect(validateDocPath('.hidden').isValid).toBe(false);
  });
});
