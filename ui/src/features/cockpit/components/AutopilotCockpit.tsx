import React from 'react';
import { Bot, PauseCircle, PlayCircle, Plus, Waypoints } from 'lucide-react';
import { AutopilotDisplayStatus, components, Status } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Button } from '@/components/ui/button';
import StatusChip from '@/components/ui/status-chip';
import Title from '@/components/ui/title';
import { useQuery } from '@/hooks/api';
import { cn } from '@/lib/utils';
import dayjs from '@/lib/dayjs';
import { AutopilotAvatar } from '@/features/autopilot/components/AutopilotAvatar';
import { AutopilotCreateModal } from '@/features/autopilot/components/AutopilotCreateModal';
import {
  filterAutopilotBySelectedWorkspace,
  workspaceTagForAutopilotSelection,
} from '@/features/autopilot/workspace';
import { AutopilotDetailsModal } from './AutopilotDetailsModal';

type AutopilotSummary = components['schemas']['AutopilotSummary'];
type DAGRunSummary = components['schemas']['DAGRunSummary'];

type LifecycleState = 'running' | 'paused' | 'idle' | 'finished';

const STATE_ORDER: LifecycleState[] = ['idle', 'running', 'paused', 'finished'];

const STATE_META: Record<
  LifecycleState,
  { label: string; description: string; icon: React.ReactNode }
> = {
  running: {
    label: 'Running',
    description: 'Live cycles and active work.',
    icon: <PlayCircle size={16} />,
  },
  paused: {
    label: 'Paused',
    description: 'Temporarily frozen by an operator.',
    icon: <PauseCircle size={16} />,
  },
  idle: {
    label: 'Idle',
    description: 'No active task assigned.',
    icon: <Waypoints size={16} />,
  },
  finished: {
    label: 'Finished',
    description: 'Completed the current task.',
    icon: <Bot size={16} />,
  },
};

function getLifecycleClass(state: string): string {
  switch (state) {
    case AutopilotDisplayStatus.running:
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case AutopilotDisplayStatus.paused:
      return 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100';
    case AutopilotDisplayStatus.finished:
      return 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function dagRunStatusToStatus(status: string): Status | undefined {
  switch (status) {
    case 'not_started':
      return Status.NotStarted;
    case 'running':
      return Status.Running;
    case 'failed':
      return Status.Failed;
    case 'aborted':
      return Status.Aborted;
    case 'succeeded':
      return Status.Success;
    case 'queued':
      return Status.Queued;
    case 'partially_succeeded':
      return Status.PartialSuccess;
    case 'waiting':
      return Status.Waiting;
    case 'rejected':
      return Status.Rejected;
    default:
      return undefined;
  }
}

function extractAutopilotName(tags?: string[]): string | null {
  const match = (tags || []).find(
    (tag) => tag.startsWith('autopilot=') || tag.startsWith('automata=')
  );
  if (!match) {
    return null;
  }
  if (match.startsWith('autopilot=')) {
    return match.slice('autopilot='.length) || null;
  }
  return match.slice('automata='.length) || null;
}

function runSortTime(run: DAGRunSummary): number {
  const candidates = [run.startedAt, run.finishedAt, run.queuedAt];
  for (const value of candidates) {
    if (value && value !== '-') {
      const parsed = dayjs(value);
      if (parsed.isValid()) {
        return parsed.valueOf();
      }
    }
  }
  return 0;
}

function formatTimestamp(value?: string): string {
  if (!value || value === '-') {
    return 'n/a';
  }
  const parsed = dayjs(value);
  if (!parsed.isValid()) {
    return 'n/a';
  }
  return parsed.fromNow();
}

type WorkspaceActivity = {
  count: number;
  latestRun?: DAGRunSummary;
};

function buildWorkspaceActivity(
  runs: DAGRunSummary[] | undefined
): Map<string, WorkspaceActivity> {
  const activity = new Map<string, WorkspaceActivity>();
  for (const run of runs || []) {
    if (run.triggerType !== 'autopilot') {
      continue;
    }
    const autopilotName = extractAutopilotName(run.tags);
    if (!autopilotName) {
      continue;
    }
    const current = activity.get(autopilotName);
    if (!current) {
      activity.set(autopilotName, { count: 1, latestRun: run });
      continue;
    }
    const nextLatest =
      !current.latestRun || runSortTime(run) >= runSortTime(current.latestRun)
        ? run
        : current.latestRun;
    activity.set(autopilotName, {
      count: current.count + 1,
      latestRun: nextLatest,
    });
  }
  return activity;
}

function sortAutopilot(
  items: AutopilotSummary[],
  workspaceActivity: Map<string, WorkspaceActivity>
): AutopilotSummary[] {
  return [...items].sort((a, b) => {
    const aActivity = workspaceActivity.get(a.name);
    const bActivity = workspaceActivity.get(b.name);
    if (!!aActivity !== !!bActivity) {
      return aActivity ? -1 : 1;
    }
    if (aActivity?.latestRun && bActivity?.latestRun) {
      return (
        runSortTime(bActivity.latestRun) - runSortTime(aActivity.latestRun)
      );
    }
    return a.name.localeCompare(b.name);
  });
}

function autopilotDisplayName(item: {
  name: string;
  nickname?: string | null;
}): string {
  return item.nickname?.trim() || item.name;
}

export function AutopilotCockpit({
  selectedWorkspace,
  initialAutopilotName,
  onAutopilotSelectionChange,
}: {
  selectedWorkspace: string;
  initialAutopilotName?: string;
  onAutopilotSelectionChange?: (name: string | null) => void;
}): React.ReactElement {
  const appBar = React.useContext(AppBarContext);
  const remoteNode = appBar.selectedRemoteNode || 'local';
  const [showCreateDialog, setShowCreateDialog] = React.useState(false);
  const [selectedAutopilotName, setSelectedAutopilotName] = React.useState<
    string | null
  >(initialAutopilotName || null);

  React.useEffect(() => {
    setSelectedAutopilotName(initialAutopilotName || null);
  }, [initialAutopilotName]);

  const {
    data: autopilotData,
    error: autopilotError,
    mutate: retryAutopilot,
  } = useQuery(
    '/autopilot',
    {},
    {
      refreshInterval: 5000,
      revalidateOnFocus: false,
      revalidateOnReconnect: true,
    }
  );

  const workspaceTag = selectedWorkspace
    ? workspaceTagForAutopilotSelection(selectedWorkspace)
    : undefined;
  const {
    data: workspaceRunsData,
    error: workspaceRunsError,
    mutate: retryWorkspaceRuns,
  } = useQuery(
    '/dag-runs',
    selectedWorkspace && workspaceTag
      ? {
          params: {
            query: {
              remoteNode,
              tags: workspaceTag,
            },
          },
        }
      : null,
    {
      refreshInterval: 5000,
      revalidateOnFocus: false,
      revalidateOnReconnect: true,
    }
  );

  const autopilot = React.useMemo(
    () =>
      selectedWorkspace && !workspaceTag
        ? []
        : filterAutopilotBySelectedWorkspace(
            autopilotData?.autopilot || [],
            selectedWorkspace
          ),
    [autopilotData?.autopilot, selectedWorkspace, workspaceTag]
  );

  const selectAutopilot = React.useCallback(
    (name: string | null) => {
      setSelectedAutopilotName(name);
      onAutopilotSelectionChange?.(name);
    },
    [onAutopilotSelectionChange]
  );

  React.useEffect(() => {
    if (!autopilotData || !selectedAutopilotName) {
      return;
    }
    if (!autopilot.some((item) => item.name === selectedAutopilotName)) {
      selectAutopilot(null);
    }
  }, [autopilot, autopilotData, selectAutopilot, selectedAutopilotName]);

  const workspaceActivity = React.useMemo(
    () => buildWorkspaceActivity(workspaceRunsData?.dagRuns),
    [workspaceRunsData?.dagRuns]
  );

  const stateBuckets = React.useMemo(() => {
    const buckets: Record<LifecycleState, AutopilotSummary[]> = {
      running: [],
      paused: [],
      idle: [],
      finished: [],
    };
    for (const item of autopilot) {
      const state = (item.displayStatus || item.state) as LifecycleState;
      if (state in buckets) {
        buckets[state].push(item);
      }
    }
    for (const state of STATE_ORDER) {
      buckets[state] = sortAutopilot(buckets[state], workspaceActivity);
    }
    return buckets;
  }, [autopilot, workspaceActivity]);

  const workspaceAutopilotCount = React.useMemo(() => {
    let count = 0;
    for (const name of workspaceActivity.keys()) {
      if (autopilot.some((item) => item.name === name)) {
        count += 1;
      }
    }
    return count;
  }, [autopilot, workspaceActivity]);

  const combinedError = autopilotError || workspaceRunsError;
  const isLoading = !autopilotData && !autopilotError;

  if (combinedError) {
    const message =
      combinedError instanceof Error
        ? combinedError.message
        : 'Failed to load Autopilot cockpit';
    return (
      <div className="flex flex-1 min-h-0 items-center justify-center">
        <div className="rounded-lg border bg-card p-6 text-center">
          <div className="text-base font-semibold">
            Failed to load Autopilot cockpit
          </div>
          <div className="mt-2 text-sm text-muted-foreground">{message}</div>
          <div className="mt-4 flex justify-center gap-2">
            <Button size="sm" onClick={() => void retryAutopilot()}>
              Retry Autopilot
            </Button>
            {selectedWorkspace ? (
              <Button
                size="sm"
                variant="outline"
                onClick={() => void retryWorkspaceRuns()}
              >
                Retry Workspace Activity
              </Button>
            ) : null}
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-1 min-h-0 flex-col gap-4 overflow-y-auto p-1">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <Title>Autopilot Cockpit</Title>
          <p className="mt-1 text-sm text-muted-foreground">
            {selectedWorkspace
              ? `Showing Autopilot tagged for workspace ${selectedWorkspace}, with workspace-tagged activity overlaid on their lifecycle state.`
              : 'Idle, running, paused, and finished Autopilot across the workspace environment.'}
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={() => setShowCreateDialog(true)}
        >
          <Plus className="h-4 w-4" />
          Create Autopilot
        </Button>
      </div>

      {selectedWorkspace ? (
        <div className="rounded-lg border border-dashed bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
          Showing Autopilot tagged with
          <span className="mx-1 font-mono text-foreground">
            {workspaceTag || 'workspace=<invalid>'}
          </span>
          . Workspace activity is derived from Autopilot-triggered DAG runs
          carrying the same tag on
          <span className="mx-1 font-mono text-foreground">{remoteNode}</span>.
          {workspaceAutopilotCount > 0 ? (
            <span className="ml-1">
              {workspaceAutopilotCount} Autopilot have workspace-tagged activity.
            </span>
          ) : null}
        </div>
      ) : (
        <div className="rounded-lg border border-dashed bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
          Select a workspace to overlay workspace-tagged Autopilot activity on
          top of the lifecycle board.
        </div>
      )}

      {isLoading ? (
        <div className="rounded-lg border bg-card p-6 text-sm text-muted-foreground">
          Loading Autopilot cockpit…
        </div>
      ) : autopilot.length === 0 ? (
        <div className="rounded-lg border bg-card p-6 text-sm text-muted-foreground">
          <div>
            {selectedWorkspace
              ? 'No Autopilot are tagged for the selected workspace.'
              : 'No Autopilot defined yet.'}
          </div>
          <div className="mt-4">
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="h-4 w-4" />
              Create Autopilot
            </Button>
          </div>
        </div>
      ) : (
        <div className="grid min-h-0 gap-4 xl:grid-cols-5 md:grid-cols-2">
          {STATE_ORDER.map((state) => {
            const items = stateBuckets[state];
            return (
              <section
                key={state}
                className="min-w-0 rounded-lg border bg-card p-3"
              >
                <div className="mb-3 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-sm font-semibold">
                    <span className="text-muted-foreground">
                      {STATE_META[state].icon}
                    </span>
                    <span>{STATE_META[state].label}</span>
                  </div>
                  <span
                    className={cn(
                      'rounded-full px-2 py-1 text-[11px] font-medium',
                      getLifecycleClass(state)
                    )}
                  >
                    {items.length}
                  </span>
                </div>
                <div className="space-y-3">
                  {items.length === 0 ? (
                    <div className="rounded-md border border-dashed px-3 py-4 text-sm text-muted-foreground">
                      No Autopilot in this state.
                    </div>
                  ) : (
                    items.map((item) => {
                      const activity = workspaceActivity.get(item.name);
                      return (
                        <button
                          type="button"
                          key={item.name}
                          onClick={() => selectAutopilot(item.name)}
                          className="block w-full rounded-md border p-3 text-left transition hover:bg-muted/40"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex min-w-0 items-start gap-3">
                              <AutopilotAvatar
                                name={item.name}
                                nickname={item.nickname}
                                iconUrl={item.iconUrl}
                                className="h-12 w-12 rounded-2xl"
                              />
                              <div className="min-w-0">
                                <div className="truncate font-medium">
                                  {autopilotDisplayName(item)}
                                </div>
                                {item.nickname ? (
                                  <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                                    {item.name}
                                  </div>
                                ) : null}
                                <div className="mt-1 break-words text-xs text-muted-foreground">
                                  {item.instruction || item.goal}
                                </div>
                              </div>
                            </div>
                            <span
                              className={cn(
                                'rounded-full px-2 py-1 text-[11px] font-medium',
                                getLifecycleClass(
                                  item.displayStatus || item.state
                                )
                              )}
                            >
                              {item.displayStatus || item.state}
                            </span>
                          </div>

                          <div className="mt-3 flex flex-wrap gap-2 text-xs">
                            {item.busy ? (
                              <span className="rounded-full bg-amber-100 px-2 py-1 font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-200">
                                busy
                              </span>
                            ) : null}
                            {item.needsInput ? (
                              <span className="rounded-full bg-rose-100 px-2 py-1 font-medium text-rose-900 dark:bg-rose-900/40 dark:text-rose-200">
                                needs input
                              </span>
                            ) : null}
                            <span className="rounded-full border px-2 py-1 text-muted-foreground">
                              Tasks: {item.doneTaskCount || 0}/
                              {(item.doneTaskCount || 0) +
                                (item.openTaskCount || 0)}
                            </span>
                            {item.disabled ? (
                              <span className="rounded-full border px-2 py-1 text-muted-foreground">
                                disabled
                              </span>
                            ) : null}
                            {item.tags?.map((tag) => (
                              <span
                                key={`${item.name}-${tag}`}
                                className="rounded-full border px-2 py-1 text-muted-foreground"
                              >
                                {tag}
                              </span>
                            ))}
                            {activity ? (
                              <span className="rounded-full border px-2 py-1 text-foreground">
                                Workspace runs: {activity.count}
                              </span>
                            ) : null}
                          </div>

                          {item.currentRun ? (
                            <div className="mt-3 rounded-md bg-muted/50 p-2">
                              <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                                Current child DAG
                              </div>
                              <div className="mt-1 flex items-center justify-between gap-2">
                                <span className="truncate text-sm font-medium">
                                  {item.currentRun.name}
                                </span>
                                <StatusChip
                                  status={dagRunStatusToStatus(
                                    item.currentRun.status
                                  )}
                                  size="xs"
                                >
                                  {item.currentRun.status}
                                </StatusChip>
                              </div>
                            </div>
                          ) : null}

                          {item.nextTaskDescription ? (
                            <div className="mt-3 text-xs text-muted-foreground">
                              Next task: {item.nextTaskDescription}
                            </div>
                          ) : null}

                          {activity?.latestRun ? (
                            <div className="mt-3 rounded-md bg-muted/30 p-2">
                              <div className="text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                                Latest workspace run
                              </div>
                              <div className="mt-1 flex items-center justify-between gap-2">
                                <span className="truncate text-sm">
                                  {activity.latestRun.name}
                                </span>
                                <StatusChip
                                  status={dagRunStatusToStatus(
                                    activity.latestRun.statusLabel
                                  )}
                                  size="xs"
                                >
                                  {activity.latestRun.statusLabel}
                                </StatusChip>
                              </div>
                              <div className="mt-1 text-xs text-muted-foreground">
                                {formatTimestamp(
                                  activity.latestRun.startedAt ||
                                    activity.latestRun.finishedAt ||
                                    activity.latestRun.queuedAt
                                )}
                              </div>
                            </div>
                          ) : null}

                          <div className="mt-3 text-xs text-muted-foreground">
                            Updated {formatTimestamp(item.lastUpdatedAt)}
                          </div>
                        </button>
                      );
                    })
                  )}
                </div>
              </section>
            );
          })}
        </div>
      )}
      {selectedAutopilotName ? (
        <AutopilotDetailsModal
          name={selectedAutopilotName}
          isOpen={!!selectedAutopilotName}
          onClose={() => selectAutopilot(null)}
          onUpdated={async () => {
            await retryAutopilot();
            if (selectedWorkspace) {
              await retryWorkspaceRuns();
            }
          }}
          onSelectedNameChange={(nextName) => selectAutopilot(nextName)}
          onDeleted={() => selectAutopilot(null)}
          selectedWorkspace={selectedWorkspace}
          remoteNode={remoteNode}
        />
      ) : null}
      <AutopilotCreateModal
        open={showCreateDialog}
        onClose={() => setShowCreateDialog(false)}
        selectedWorkspace={selectedWorkspace}
        remoteNode={remoteNode}
        onCreated={async (name) => {
          await retryAutopilot();
          if (selectedWorkspace) {
            await retryWorkspaceRuns();
          }
          selectAutopilot(name);
        }}
      />
    </div>
  );
}
