import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

import { describe, expect, it } from 'vitest';

const designPageSource = readFileSync(
  resolve(process.cwd(), 'src/pages/design/index.tsx'),
  'utf8'
);

describe('workflow design steward copy', () => {
  it('uses steward-facing labels for workflow design actions', () => {
    expect(designPageSource).toContain('Steward is disabled');
    expect(designPageSource).toContain(
      'Enable Steward before starting a steward-guided workflow design session.'
    );
    expect(designPageSource).toContain(
      'Steward-guided workflow authoring and DAG preview'
    );
    expect(designPageSource).toContain('Ask Steward');
  });

  it('removes the old agent-facing labels from the design UI', () => {
    expect(designPageSource).not.toContain('Agent is disabled');
    expect(designPageSource).not.toContain(
      'Enable the agent before starting an agentic workflow design session.'
    );
    expect(designPageSource).not.toContain('Send to Agent');
    expect(designPageSource).not.toContain(
      'Enable the agent to send workflow change requests.'
    );
  });
});
