// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { normalizeDocPathFromURL } from '../doc-url';

describe('normalizeDocPathFromURL', () => {
  it('strips a markdown extension from URL paths', () => {
    expect(normalizeDocPathFromURL('runbooks/deploy.md')).toBe(
      'runbooks/deploy'
    );
    expect(normalizeDocPathFromURL('runbooks/DEPLOY.MD')).toBe(
      'runbooks/DEPLOY'
    );
  });

  it('keeps leading-underscore names visible after stripping the extension', () => {
    expect(normalizeDocPathFromURL('_index.md')).toBe('_index');
    expect(normalizeDocPathFromURL('guides/_partial.md')).toBe(
      'guides/_partial'
    );
  });

  it('does not strip md text from non-markdown suffixes', () => {
    expect(normalizeDocPathFromURL('notes.md.backup')).toBe('notes.md.backup');
  });
});
