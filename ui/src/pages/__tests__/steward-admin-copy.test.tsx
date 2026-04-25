// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

function read(relativePath: string): string {
  return readFileSync(resolve(process.cwd(), relativePath), 'utf8');
}

describe('steward admin copy', () => {
  it('renames settings copy to steward language', () => {
    const source = read('src/pages/agent-settings/index.tsx');

    expect(source).toContain('Steward Settings');
    expect(source).toContain('Enable Steward');
    expect(source).toContain('Default Profile');
    expect(source).toContain("steward&apos;s identity");
  });

  it('renames memory copy to steward language', () => {
    const source = read('src/pages/agent-memory/index.tsx');

    expect(source).toContain('Steward Memory');
    expect(source).toContain("steward&apos;s persistent memory");
    expect(source).toContain('The steward will write here when it learns something.');
  });

  it('renames souls copy to profiles language', () => {
    const listSource = read('src/pages/agent-souls/index.tsx');
    const editorSource = read('src/pages/agent-souls/SoulEditorPage.tsx');

    expect(listSource).toContain('Profiles');
    expect(listSource).toContain('Create Profile');
    expect(listSource).toContain('Search profiles...');
    expect(listSource).toContain('No profiles configured. Create a profile to get started.');
    expect(editorSource).toContain('Create Profile');
    expect(editorSource).toContain('Edit Profile:');
    expect(editorSource).toContain('What this profile defines');
  });
});
