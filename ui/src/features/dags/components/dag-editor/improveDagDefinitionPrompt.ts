import { components, NodeStatus } from '@/api/v1/schema';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

export type BuildImproveDAGDefinitionPromptInput = {
  dagFile: string;
  dagName?: string;
  latestDAGRun?: DAGRunDetails;
  userPrompt: string;
};

const MAX_PROBLEM_STEPS = 5;
const MAX_VALUE_LENGTH = 240;

const PROBLEMATIC_NODE_STATUSES = new Set<number>([
  NodeStatus.Running,
  NodeStatus.Failed,
  NodeStatus.Aborted,
  NodeStatus.PartialSuccess,
  NodeStatus.Waiting,
  NodeStatus.Rejected,
  NodeStatus.Retrying,
]);

export function buildImproveDAGDefinitionPrompt({
  dagFile,
  dagName,
  latestDAGRun,
  userPrompt,
}: BuildImproveDAGDefinitionPromptInput): string {
  const trimmedPrompt = userPrompt.trim();

  return [
    'Improve the referenced DAG definition in the workspace.',
    '',
    `DAG file: ${dagFile}`,
    `DAG name: ${dagName || dagFile}`,
    '',
    'User request:',
    trimmedPrompt,
    '',
    'Latest run detail:',
    formatLatestDAGRunDetail(latestDAGRun),
    '',
    'Instructions:',
    '- Use the attached DAG reference to inspect and edit the DAG definition.',
    '- Base the improvement on the latest run detail and the user request.',
    '- Make the file changes directly instead of only suggesting them.',
    '- Briefly explain the changes you made and why.',
  ].join('\n');
}

export function formatLatestDAGRunDetail(latestDAGRun?: DAGRunDetails): string {
  if (!latestDAGRun) {
    return '- No latest run details are available for this DAG yet.';
  }

  const lines = [
    `- Run ID: ${latestDAGRun.dagRunId}`,
    `- Status: ${latestDAGRun.statusLabel}`,
  ];

  if (latestDAGRun.triggerType) {
    lines.push(`- Trigger: ${latestDAGRun.triggerType}`);
  }
  if (latestDAGRun.workerId) {
    lines.push(`- Worker: ${latestDAGRun.workerId}`);
  }
  if (latestDAGRun.startedAt) {
    lines.push(`- Started at: ${latestDAGRun.startedAt}`);
  }
  if (latestDAGRun.finishedAt) {
    lines.push(`- Finished at: ${latestDAGRun.finishedAt}`);
  }
  if (latestDAGRun.queuedAt) {
    lines.push(`- Queued at: ${latestDAGRun.queuedAt}`);
  }
  if (latestDAGRun.scheduleTime) {
    lines.push(`- Schedule time: ${latestDAGRun.scheduleTime}`);
  }
  if (latestDAGRun.params) {
    lines.push(`- Params: ${formatParams(latestDAGRun.params)}`);
  }

  const problemSteps = summarizeProblemSteps(latestDAGRun);
  if (problemSteps.length === 0) {
    lines.push('- Problematic steps: none highlighted in the latest run.');
    return lines.join('\n');
  }

  lines.push('- Problematic steps:');
  for (const step of problemSteps) {
    const details = step.error ? ` - ${step.error}` : '';
    lines.push(`  - ${step.name} (${step.statusLabel})${details}`);
  }

  const omittedCount =
    countProblemSteps(latestDAGRun.nodes || []) - problemSteps.length;
  if (omittedCount > 0) {
    lines.push(`  - ... ${omittedCount} more problematic step(s) omitted`);
  }

  return lines.join('\n');
}

function summarizeProblemSteps(latestDAGRun: DAGRunDetails) {
  return (latestDAGRun.nodes || [])
    .filter((node) => PROBLEMATIC_NODE_STATUSES.has(node.status))
    .slice(0, MAX_PROBLEM_STEPS)
    .map((node) => ({
      name: node.step.name,
      statusLabel: node.statusLabel,
      error: node.error ? truncate(cleanInline(node.error)) : '',
    }));
}

function countProblemSteps(nodes: components['schemas']['Node'][]): number {
  return nodes.filter((node) => PROBLEMATIC_NODE_STATUSES.has(node.status))
    .length;
}

function formatParams(params: string): string {
  try {
    return truncate(JSON.stringify(JSON.parse(params)));
  } catch {
    return truncate(cleanInline(params));
  }
}

function cleanInline(value: string): string {
  return value.replace(/\s+/g, ' ').trim();
}

function truncate(value: string, limit: number = MAX_VALUE_LENGTH): string {
  if (value.length <= limit) {
    return value;
  }
  return `${value.slice(0, limit - 3)}...`;
}
