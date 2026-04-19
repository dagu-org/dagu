export type BuildWorkflowDesignPromptInput = {
  mode: 'create' | 'update';
  dagFile?: string;
  newDagName?: string;
  stepName?: string;
  remoteNode: string;
  userPrompt: string;
  draftSpec?: string;
  validationErrors?: string[];
};

const MAX_DRAFT_SPEC_LENGTH = 6000;

export function buildWorkflowDesignPrompt({
  mode,
  dagFile,
  newDagName,
  stepName,
  remoteNode,
  userPrompt,
  draftSpec,
  validationErrors,
}: BuildWorkflowDesignPromptInput): string {
  const target = mode === 'update' ? dagFile : newDagName;
  const lines = [
    'Use the workflow design workspace to author or update a Dagu DAG.',
    '',
    `Mode: ${mode === 'update' ? 'Update existing DAG' : 'Create new DAG'}`,
    `Remote node: ${remoteNode}`,
    `Target DAG: ${target || '(not selected)'}`,
  ];

  if (stepName) {
    lines.push(`Target step: ${stepName}`);
  }

  lines.push('', 'User request:', userPrompt.trim(), '');

  if (draftSpec && draftSpec.trim()) {
    lines.push(
      'Current draft YAML shown in the design pane:',
      truncateDraftSpec(draftSpec),
      ''
    );
  }

  if (validationErrors && validationErrors.length > 0) {
    lines.push('Current validation errors:');
    for (const error of validationErrors) {
      lines.push(`- ${error}`);
    }
    lines.push('');
  }

  lines.push(
    'Instructions:',
    '- Inspect the referenced DAG file before editing an existing DAG.',
    '- Make the file changes directly with the available tools; do not only suggest changes.',
    '- Keep edits focused on the requested workflow behavior and selected step when provided.',
    '- Validate the DAG after editing and fix validation errors before stopping.',
    '- When finished, summarize the changes and navigate to /design?dag=<dag-file> so the user can review.'
  );

  return lines.join('\n');
}

function truncateDraftSpec(spec: string): string {
  if (spec.length <= MAX_DRAFT_SPEC_LENGTH) {
    return spec;
  }
  return `${spec.slice(0, MAX_DRAFT_SPEC_LENGTH)}\n... [draft truncated]`;
}
