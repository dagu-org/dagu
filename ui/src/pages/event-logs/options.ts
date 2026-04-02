import type { EventKindFilter } from './types';

export const EVENT_KIND_OPTIONS = [
  { value: 'all', label: 'All kinds' },
  { value: 'dag_run', label: 'DAG runs' },
  { value: 'automata', label: 'Automata' },
  { value: 'llm_usage', label: 'LLM usage' },
] as const;

const DAG_RUN_EVENT_TYPE_OPTIONS = [
  { value: 'all', label: 'All event types' },
  { value: 'dag.run.succeeded', label: 'Succeeded' },
  { value: 'dag.run.failed', label: 'Failed' },
  { value: 'dag.run.aborted', label: 'Aborted' },
  { value: 'dag.run.waiting', label: 'Waiting' },
  { value: 'dag.run.rejected', label: 'Rejected' },
] as const;

const AUTOMATA_EVENT_TYPE_OPTIONS = [
  { value: 'all', label: 'All event types' },
  { value: 'automata.needs_input', label: 'Needs input' },
  { value: 'automata.error', label: 'Automata error' },
  { value: 'automata.finished', label: 'Automata finished' },
] as const;

const LLM_USAGE_EVENT_TYPE_OPTIONS = [
  { value: 'all', label: 'All event types' },
  { value: 'llm.usage.recorded', label: 'LLM usage' },
] as const;

const ALL_EVENT_TYPE_OPTIONS = [
  { value: 'all', label: 'All event types' },
  ...DAG_RUN_EVENT_TYPE_OPTIONS.slice(1),
  ...AUTOMATA_EVENT_TYPE_OPTIONS.slice(1),
  ...LLM_USAGE_EVENT_TYPE_OPTIONS.slice(1),
] as const;

const EVENT_TYPE_OPTIONS_BY_KIND: Record<
  EventKindFilter,
  ReadonlyArray<{ value: string; label: string }>
> = {
  all: ALL_EVENT_TYPE_OPTIONS,
  dag_run: DAG_RUN_EVENT_TYPE_OPTIONS,
  automata: AUTOMATA_EVENT_TYPE_OPTIONS,
  llm_usage: LLM_USAGE_EVENT_TYPE_OPTIONS,
};

export function isEventKindFilter(
  value: string | null | undefined
): value is EventKindFilter {
  return (
    value === 'all' ||
    value === 'dag_run' ||
    value === 'automata' ||
    value === 'llm_usage'
  );
}

export function getEventTypeOptions(kind: EventKindFilter) {
  return EVENT_TYPE_OPTIONS_BY_KIND[kind];
}

export function sanitizeEventTypeForKind(
  kind: EventKindFilter,
  type: string
): string {
  if (!type || type === 'all') {
    return 'all';
  }
  const allowedTypes = EVENT_TYPE_OPTIONS_BY_KIND[kind];
  return allowedTypes.some((option) => option.value === type) ? type : 'all';
}
