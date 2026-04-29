import React from 'react';
import { Bot, PauseCircle, PlayCircle, Plus, Waypoints } from 'lucide-react';
import { ControllerDisplayStatus, components, Status } from '@/api/v1/schema';
import { AppBarContext } from '@/contexts/AppBarContext';
import { Button } from '@/components/ui/button';
import StatusChip from '@/components/ui/status-chip';
import Title from '@/components/ui/title';
import { useQuery } from '@/hooks/api';
import { cn } from '@/lib/utils';
import dayjs from '@/lib/dayjs';
import { ControllerAvatar } from '@/features/controller/components/ControllerAvatar';
import { ControllerCreateModal } from '@/features/controller/components/ControllerCreateModal';
import {
  filterControllerBySelectedWorkspace,
  workspaceTagForControllerSelection,
} from '@/features/controller/workspace';
import { ControllerDetailsModal } from './ControllerDetailsModal';

type ControllerSummary = components['schemas']['ControllerSummary'];
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
    case ControllerDisplayStatus.running:
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case ControllerDisplayStatus.paused:
      return 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100';
    case ControllerDisplayStatus.finished:
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

function extractControllerName(tags?: string[]): string | null {
  const match = (tags || []).find((tag) => tag.startsWith('controller='));
  if (!match) {
    return null;
  }
  return match.slice('controller='.length) || null;
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
    if (run.triggerType !== 'controller') {
      continue;
    }
    const controllerName = extractControllerName(run.tags);
    if (!controllerName) {
      continue;
    }
    const current = activity.get(controllerName);
    if (!current) {
      activity.set(controllerName, { count: 1, latestRun: run });
      continue;
    }
    const nextLatest =
      !current.latestRun || runSortTime(run) >= runSortTime(current.latestRun)
        ? run
        : current.latestRun;
    activity.set(controllerName, {
      count: current.count + 1,
      latestRun: nextLatest,
    });
  }
  return activity;
}

function sortController(
  items: ControllerSummary[],
  workspaceActivity: Map<string, WorkspaceActivity>
): ControllerSummary[] {
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

function controllerDisplayName(item: {
  name: string;
  nickname?: string | null;
}): string {
  return item.nickname?.trim() || item.name;
}

export function ControllerCockpit({
  selectedWorkspace,
  initialControllerName,
  onControllerSelectionChange,
}: {
  selectedWorkspace: string;
  initialControllerName?: string;
  onControllerSelectionChange?: (name: string | null) => void;
}): React.ReactElement {
  const appBar = React.useContext(AppBarContext);
  const remoteNode = appBar.selectedRemoteNode || 'local';
  const [showCreateDialog, setShowCreateDialog] = React.useState(false);
  const [selectedControllerName, setSelectedControllerName] = React.useState<
    string | null
  >(initialControllerName || null);

  React.useEffect(() => {
    setSelectedControllerName(initialControllerName || null);
  }, [initialControllerName]);

  const {
    data: controllerData,
    error: controllerError,
    mutate: retryController,
  } = useQuery(
    '/controller',
    {},
    {
      refreshInterval: 5000,
      revalidateOnFocus: false,
      revalidateOnReconnect: true,
    }
  );

  const workspaceTag = selectedWorkspace
    ? workspaceTagForControllerSelection(selectedWorkspace)
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

  const controller = React.useMemo(
    () =>
      selectedWorkspace && !workspaceTag
        ? []
        : filterControllerBySelectedWorkspace(
            controllerData?.controller || [],
            selectedWorkspace
          ),
    [controllerData?.controller, selectedWorkspace, workspaceTag]
  );

  const selectController = React.useCallback(
    (name: string | null) => {
      setSelectedControllerName(name);
      onControllerSelectionChange?.(name);
    },
    [onControllerSelectionChange]
  );

  React.useEffect(() => {
    if (!controllerData || !selectedControllerName) {
      return;
    }
    if (!controller.some((item) => item.name === selectedControllerName)) {
      selectController(null);
    }
  }, [controller, controllerData, selectController, selectedControllerName]);

  const workspaceActivity = React.useMemo(
    () => buildWorkspaceActivity(workspaceRunsData?.dagRuns),
    [workspaceRunsData?.dagRuns]
  );

  const stateBuckets = React.useMemo(() => {
    const buckets: Record<LifecycleState, ControllerSummary[]> = {
      running: [],
      paused: [],
      idle: [],
      finished: [],
    };
    for (const item of controller) {
      const state = (item.displayStatus || item.state) as LifecycleState;
      if (state in buckets) {
        buckets[state].push(item);
      }
    }
    for (const state of STATE_ORDER) {
      buckets[state] = sortController(buckets[state], workspaceActivity);
    }
    return buckets;
  }, [controller, workspaceActivity]);

  const workspaceControllerCount = React.useMemo(() => {
    let count = 0;
    for (const name of workspaceActivity.keys()) {
      if (controller.some((item) => item.name === name)) {
        count += 1;
      }
    }
    return count;
  }, [controller, workspaceActivity]);

  const combinedError = controllerError || workspaceRunsError;
  const isLoading = !controllerData && !controllerError;

  if (combinedError) {
    const message =
      combinedError instanceof Error
        ? combinedError.message
        : 'Failed to load Controller cockpit';
    return (
      <div className="flex flex-1 min-h-0 items-center justify-center">
        <div className="rounded-lg border bg-card p-6 text-center">
          <div className="text-base font-semibold">
            Failed to load Controller cockpit
          </div>
          <div className="mt-2 text-sm text-muted-foreground">{message}</div>
          <div className="mt-4 flex justify-center gap-2">
            <Button size="sm" onClick={() => void retryController()}>
              Retry Controller
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
          <Title>Controller Cockpit</Title>
          <p className="mt-1 text-sm text-muted-foreground">
            {selectedWorkspace
              ? `Showing Controller tagged for workspace ${selectedWorkspace}, with workspace-tagged activity overlaid on their lifecycle state.`
              : 'Idle, running, paused, and finished Controller across the workspace environment.'}
          </p>
        </div>
        <Button
          size="sm"
          variant="outline"
          onClick={() => setShowCreateDialog(true)}
        >
          <Plus className="h-4 w-4" />
          Create Controller
        </Button>
      </div>

      {selectedWorkspace ? (
        <div className="rounded-lg border border-dashed bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
          Showing Controller tagged with
          <span className="mx-1 font-mono text-foreground">
            {workspaceTag || 'workspace=<invalid>'}
          </span>
          . Workspace activity is derived from Controller-triggered DAG runs
          carrying the same tag on
          <span className="mx-1 font-mono text-foreground">{remoteNode}</span>.
          {workspaceControllerCount > 0 ? (
            <span className="ml-1">
              {workspaceControllerCount} Controller have workspace-tagged activity.
            </span>
          ) : null}
        </div>
      ) : (
        <div className="rounded-lg border border-dashed bg-muted/20 px-4 py-3 text-sm text-muted-foreground">
          Select a workspace to overlay workspace-tagged Controller activity on
          top of the lifecycle board.
        </div>
      )}

      {isLoading ? (
        <div className="rounded-lg border bg-card p-6 text-sm text-muted-foreground">
          Loading Controller cockpit…
        </div>
      ) : controller.length === 0 ? (
        <div className="rounded-lg border bg-card p-6 text-sm text-muted-foreground">
          <div>
            {selectedWorkspace
              ? 'No Controller are tagged for the selected workspace.'
              : 'No Controller defined yet.'}
          </div>
          <div className="mt-4">
            <Button
              size="sm"
              variant="outline"
              onClick={() => setShowCreateDialog(true)}
            >
              <Plus className="h-4 w-4" />
              Create Controller
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
                      No Controller in this state.
                    </div>
                  ) : (
                    items.map((item) => {
                      const activity = workspaceActivity.get(item.name);
                      return (
                        <button
                          type="button"
                          key={item.name}
                          onClick={() => selectController(item.name)}
                          className="block w-full rounded-md border p-3 text-left transition hover:bg-muted/40"
                        >
                          <div className="flex items-start justify-between gap-2">
                            <div className="flex min-w-0 items-start gap-3">
                              <ControllerAvatar
                                name={item.name}
                                nickname={item.nickname}
                                iconUrl={item.iconUrl}
                                className="h-12 w-12 rounded-2xl"
                              />
                              <div className="min-w-0">
                                <div className="truncate font-medium">
                                  {controllerDisplayName(item)}
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
      {selectedControllerName ? (
        <ControllerDetailsModal
          name={selectedControllerName}
          isOpen={!!selectedControllerName}
          onClose={() => selectController(null)}
          onUpdated={async () => {
            await retryController();
            if (selectedWorkspace) {
              await retryWorkspaceRuns();
            }
          }}
          onSelectedNameChange={(nextName) => selectController(nextName)}
          onDeleted={() => selectController(null)}
          selectedWorkspace={selectedWorkspace}
          remoteNode={remoteNode}
        />
      ) : null}
      <ControllerCreateModal
        open={showCreateDialog}
        onClose={() => setShowCreateDialog(false)}
        selectedWorkspace={selectedWorkspace}
        remoteNode={remoteNode}
        onCreated={async (name) => {
          await retryController();
          if (selectedWorkspace) {
            await retryWorkspaceRuns();
          }
          selectController(name);
        }}
      />
    </div>
  );
}
