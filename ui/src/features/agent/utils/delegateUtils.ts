import { DelegateInfo, ToolCall } from '../types';

/**
 * Check if a list of tool calls contains a delegate tool call.
 */
export function isDelegateToolCall(toolCalls?: ToolCall[]): boolean {
  return toolCalls?.some((tc) => tc.function.name === 'delegate') ?? false;
}

/**
 * Parse delegate task descriptions from a delegate tool call's arguments.
 * Returns an array of task strings extracted from the `tasks` array.
 */
export function parseDelegateTasks(toolCall: ToolCall): string[] {
  try {
    const parsed = JSON.parse(toolCall.function.arguments) as {
      tasks?: { task?: string }[];
    };
    if (!Array.isArray(parsed.tasks)) return [];
    return parsed.tasks
      .map((t) => t.task)
      .filter((t): t is string => typeof t === 'string' && t.length > 0);
  } catch {
    return [];
  }
}

/**
 * Look up a delegate's status from the delegate statuses map.
 */
export function getDelegateStatus(
  id: string,
  statuses: Record<string, DelegateInfo>
): DelegateInfo | undefined {
  return statuses[id];
}
