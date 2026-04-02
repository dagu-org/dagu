import type { EventLogEntry, EventLogFilters, SpecificPeriod } from './types';

export function getEventTypeLabel(type: string, status?: string): string {
  switch (type) {
    case 'dag.run.succeeded':
      return 'Succeeded';
    case 'dag.run.failed':
      return 'Failed';
    case 'dag.run.aborted':
      return 'Aborted';
    case 'dag.run.waiting':
      return 'Waiting';
    case 'dag.run.rejected':
      return 'Rejected';
    case 'automata.needs_input':
      return 'Needs input';
    case 'automata.error':
      return 'Error';
    case 'automata.finished':
      return 'Finished';
    case 'llm.usage.recorded':
      return 'Usage recorded';
    default:
      if (status) {
        return status.replace(/_/g, ' ');
      }
      return type;
  }
}

export function getKindLabel(kind: string): string {
  switch (kind) {
    case 'dag_run':
      return 'DAG run';
    case 'automata':
      return 'Automata';
    case 'llm_usage':
      return 'LLM usage';
    default:
      return kind;
  }
}

export function getSubjectName(entry: EventLogEntry): string {
  return entry.dagName || entry.automataName || entry.sessionId || '-';
}

export function getContextLabel(entry: EventLogEntry): string {
  if (entry.kind === 'dag_run') {
    const parts = [entry.dagRunId, entry.attemptId].filter(Boolean);
    return parts.length > 0 ? parts.join(' / ') : '-';
  }
  if (entry.kind === 'automata') {
    const parts = [entry.automataKind, entry.automataCycleId].filter(Boolean);
    return parts.length > 0 ? parts.join(' / ') : '-';
  }
  const parts = [entry.model, entry.sessionId].filter(Boolean);
  return parts.length > 0 ? parts.join(' / ') : '-';
}

export function safeStringify(value: unknown): string {
  try {
    return JSON.stringify(value, null, 2) ?? '';
  } catch {
    return String(value);
  }
}

export function getInputTypeForPeriod(period: SpecificPeriod): string {
  switch (period) {
    case 'date':
      return 'date';
    case 'month':
      return 'month';
    case 'year':
      return 'number';
  }
}

export function buildRunPath(entry: EventLogEntry): string | null {
  if (!entry.dagName || !entry.dagRunId) {
    return null;
  }
  return `/dag-runs/${encodeURIComponent(entry.dagName)}/${encodeURIComponent(entry.dagRunId)}`;
}

export function hasQueryParams(params: URLSearchParams): boolean {
  return Array.from(params.keys()).length > 0;
}

export function appendUniqueEntries(
  current: EventLogEntry[],
  next: EventLogEntry[]
): EventLogEntry[] {
  if (next.length === 0) {
    return current;
  }
  const seen = new Set(current.map((entry) => entry.id));
  const merged = [...current];
  for (const entry of next) {
    if (seen.has(entry.id)) {
      continue;
    }
    seen.add(entry.id);
    merged.push(entry);
  }
  return merged;
}

export function mergeUniqueEntries(
  head: EventLogEntry[],
  older: EventLogEntry[]
): EventLogEntry[] {
  return appendUniqueEntries(head, older);
}

export function getClientErrorMessage(error: unknown, fallback: string): string {
  if (error && typeof error === 'object') {
    const maybeMessage = (error as { message?: unknown }).message;
    if (typeof maybeMessage === 'string' && maybeMessage) {
      return maybeMessage;
    }
  }
  return fallback;
}

export function areEventLogFiltersEqual(
  a: EventLogFilters,
  b: EventLogFilters
): boolean {
  return (
    a.kind === b.kind &&
    a.type === b.type &&
    a.dagName === b.dagName &&
    a.automataName === b.automataName &&
    a.dagRunId === b.dagRunId &&
    a.attemptId === b.attemptId &&
    a.fromDate === b.fromDate &&
    a.toDate === b.toDate &&
    a.dateRangeMode === b.dateRangeMode &&
    a.datePreset === b.datePreset &&
    a.specificPeriod === b.specificPeriod &&
    a.specificValue === b.specificValue
  );
}
