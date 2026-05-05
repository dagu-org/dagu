import { describe, expect, it } from 'vitest';
import { normalizeDocPathFromURL } from '../doc-url';

describe('normalizeDocPathFromURL', () => {
  it('strips a markdown extension from URL paths', () => {
    expect(normalizeDocPathFromURL('runbooks/deploy.md')).toBe(
      'runbooks/deploy'
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
