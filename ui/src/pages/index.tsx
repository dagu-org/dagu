// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { RefreshButton } from '@/components/ui/refresh-button';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { Activity, Search } from 'lucide-react';
import { cn } from '@/lib/utils';
import React from 'react';
import { Link } from 'react-router-dom';
import { AppBarContext } from '../contexts/AppBarContext';
import { useConfig } from '../contexts/ConfigContext';
import { ActivityLanes } from '../features/activity/components/ActivityLanes';
import {
  ActivityCounts,
  InProgress,
  NeedsAttention,
  UpNext,
} from '../features/activity/components/ActivityPanels';
import {
  type ActivityWindow,
  useDAGRunActivity,
} from '../features/activity/hooks/useDAGRunActivity';
import {
  attentionItems,
  fitRange,
  inProgressRuns,
  type TimeRange,
} from '../features/activity/lib/activityModel';
import { CreateDAGModal } from '../features/dags/components/common';
import PathsCard from '../features/system-status/components/PathsCard';
import { useQuery } from '../hooks/api';
import dayjs from '../lib/dayjs';
import { workspaceSelectionQuery } from '../lib/workspace';
import { I18nText } from '@/i18n/I18nText';
import { I18nTemplate } from '@/i18n/I18nTemplate';
import { useI18n } from '@/i18n/I18nProvider';

/** How often the "now" line and attention timings move forward. */
const NOW_TICK_MS = 30_000;
/** Workflows read to tell an empty install from one with only examples. */
const EMPTY_STATE_DAG_SAMPLE = 5;

type LaneScale = 'fit' | 'window';

function getDayBounds(
  date: dayjs.Dayjs,
  tzOffsetInSec: number | undefined
): { startOfDay: dayjs.Dayjs; endOfDay: dayjs.Dayjs } {
  const adjusted =
    tzOffsetInSec !== undefined ? date.utcOffset(tzOffsetInSec / 60) : date;
  return {
    startOfDay: adjusted.startOf('day'),
    endOfDay: adjusted.endOf('day'),
  };
}

function useNowSeconds(): number {
  const [now, setNow] = React.useState(() => Math.floor(Date.now() / 1000));
  React.useEffect(() => {
    const timer = setInterval(
      () => setNow(Math.floor(Date.now() / 1000)),
      NOW_TICK_MS
    );
    return () => clearInterval(timer);
  }, []);
  return now;
}

function GettingStartedPanel(): React.ReactElement {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-3 rounded-xl border border-border bg-surface p-8 text-center">
      <h2 className="text-xl font-semibold text-foreground">
        <I18nText text={'Create your first workflow'} />
      </h2>
      <p className="max-w-md text-base text-muted-foreground">
        <I18nText
          text={
            'Dagu runs workflows defined in YAML. Create one from scratch or start from the documentation examples.'
          }
        />
      </p>
      <CreateDAGModal />
      <div className="flex flex-wrap items-center justify-center gap-4 text-sm">
        <a
          href="https://docs.dagu.sh"
          target="_blank"
          rel="noreferrer"
          className="text-primary hover:underline"
        >
          <I18nText text={'Documentation'} />
        </a>
        <a
          href="https://docs.dagu.sh/writing-workflows/examples"
          target="_blank"
          rel="noreferrer"
          className="text-primary hover:underline"
        >
          <I18nText text={'Example workflows'} />
        </a>
      </div>
    </div>
  );
}

function NoRunsNotice({
  live,
  hasExampleDAGs,
}: {
  live: boolean;
  hasExampleDAGs: boolean;
}): React.ReactElement {
  return (
    <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-border p-10 text-center">
      <Activity
        className="h-8 w-8 text-muted-foreground/50"
        aria-hidden="true"
      />
      <h2 className="text-base font-medium text-foreground">
        {live ? (
          <I18nText text={'No runs in the last 24 hours'} />
        ) : (
          <I18nText text={'No runs in this range'} />
        )}
      </h2>
      <p className="text-sm text-muted-foreground">
        <I18nTemplate
          text={
            hasExampleDAGs
              ? 'Run one of the example workflows from the {link} to see activity here.'
              : 'Start a workflow from the {link} to see activity here.'
          }
          values={{
            link: (
              <Link to="/dags" className="text-primary hover:underline">
                <I18nText text="Workflows page" />
              </Link>
            ),
          }}
        />
      </p>
    </div>
  );
}

function UnavailableNotice({
  onRetry,
}: {
  onRetry: () => void;
}): React.ReactElement {
  return (
    <div className="flex flex-col items-center gap-3 rounded-lg border border-border p-10 text-center">
      <h2 className="text-base font-medium text-foreground">
        <I18nText text={'Activity is not available from this node'} />
      </h2>
      <p className="max-w-md text-sm text-muted-foreground">
        <I18nText
          text={
            'The node may run an older Dagu version. Its runs are still listed on the Executions page.'
          }
        />
      </p>
      <div className="flex gap-2">
        <Button variant="outline" onClick={onRetry}>
          <I18nText text={'Try again'} />
        </Button>
        <Button asChild variant="outline">
          <Link to="/dag-runs">
            <I18nText text={'Open Executions'} />
          </Link>
        </Button>
      </div>
    </div>
  );
}

/** Overview of recent runs: what needs attention, what runs, what is next. */
function Dashboard(): React.ReactElement {
  const { ts } = useI18n();
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const workspaceQuery = React.useMemo(
    () => workspaceSelectionQuery(appBarContext.workspaceSelection),
    [appBarContext.workspaceSelection]
  );
  const tzOffset = config.tzOffsetInSec ?? 0;
  const [activityWindow, setActivityWindow] = React.useState<ActivityWindow>({
    kind: 'live',
  });
  const [filter, setFilter] = React.useState('');
  const [scale, setScale] = React.useState<LaneScale>('fit');
  const nowSeconds = useNowSeconds();
  const live = activityWindow.kind === 'live';

  React.useEffect(() => {
    appBarContext.setTitle('Activity');
  }, [appBarContext]);

  const { activity, isLoading, unavailable, refresh } = useDAGRunActivity(
    activityWindow,
    workspaceQuery,
    remoteNode
  );

  const filteredActivity = React.useMemo(() => {
    const query = filter.trim().toLowerCase();
    if (!activity || !query) return activity;
    const matches = (name?: string) =>
      (name ?? '').toLowerCase().includes(query);
    return {
      ...activity,
      workflows: activity.workflows.filter((workflow) =>
        matches(workflow.name)
      ),
      active: activity.active.filter((run) => matches(run.name)),
      upcoming: activity.upcoming.filter((item) => matches(item.name)),
    };
  }, [activity, filter]);

  // Telling "no workflows" from "no runs" costs one small request, sent only
  // when the window is empty. A few names are enough to spot the examples.
  const isEmpty =
    !!activity &&
    activity.workflows.length === 0 &&
    activity.active.length === 0;
  const { data: dagList } = useQuery(
    '/dags',
    isEmpty
      ? {
          params: {
            query: {
              perPage: EMPTY_STATE_DAG_SAMPLE,
              remoteNode,
              ...workspaceQuery,
            },
          },
        }
      : null,
    { revalidateOnFocus: false }
  );
  const hasNoWorkflows = isEmpty && dagList?.pagination?.totalRecords === 0;
  const hasExampleDAGs = !!dagList?.dags.some((dag) =>
    dag.dag.name.startsWith('example-')
  );

  let range: TimeRange | null = null;
  if (filteredActivity) {
    range =
      scale === 'fit' && live
        ? fitRange(filteredActivity)
        : { from: filteredActivity.from, to: filteredActivity.to };
  }

  const selectDay = (value: string) => {
    if (!value) return;
    const date = dayjs(value);
    if (!date.isValid()) return;
    const { startOfDay, endOfDay } = getDayBounds(date, config.tzOffsetInSec);
    setActivityWindow({
      kind: 'fixed',
      from: startOfDay.unix(),
      to: endOfDay.unix(),
    });
  };

  const dayValue =
    activityWindow.kind === 'fixed'
      ? dayjs
          .unix(activityWindow.from)
          .utcOffset(tzOffset / 60)
          .format('YYYY-MM-DD')
      : '';

  let body: React.ReactNode;
  if (unavailable) {
    body = <UnavailableNotice onRetry={() => void refresh()} />;
  } else if (isLoading || !filteredActivity || !range) {
    body = (
      <div
        role="status"
        aria-label={ts('Loading...')}
        className="h-40 animate-pulse rounded-lg bg-muted/50"
      />
    );
  } else if (hasNoWorkflows) {
    body = <GettingStartedPanel />;
  } else if (isEmpty) {
    body = <NoRunsNotice live={live} hasExampleDAGs={hasExampleDAGs} />;
  } else {
    body = (
      <>
        <ActivityCounts activity={filteredActivity} />
        <div
          className={cn(
            'grid min-w-0 gap-6',
            live && 'xl:grid-cols-[minmax(0,1fr)_18rem]'
          )}
        >
          <div className="flex min-w-0 flex-col gap-6">
            {live && (
              <NeedsAttention
                items={attentionItems(filteredActivity, nowSeconds)}
                remoteNode={remoteNode}
                onChanged={() => void refresh()}
              />
            )}
            <section className="flex min-w-0 flex-col gap-2">
              <div className="flex items-center justify-between gap-2">
                <h2 className="text-sm font-medium text-foreground">
                  <I18nText text={'Runs over time'} />
                </h2>
                {live && (
                  <ToggleGroup aria-label={ts('Time range')} className="p-0.5">
                    <ToggleButton
                      value="fit"
                      groupValue={scale}
                      onClick={() => setScale('fit')}
                      className="h-7 px-2.5"
                    >
                      <I18nText text={'Fit'} />
                    </ToggleButton>
                    <ToggleButton
                      value="window"
                      groupValue={scale}
                      onClick={() => setScale('window')}
                      className="h-7 px-2.5"
                    >
                      <I18nText text={'24h'} />
                    </ToggleButton>
                  </ToggleGroup>
                )}
              </div>
              {filteredActivity.workflows.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                  <I18nText text={'No workflows match the filter.'} />
                </p>
              ) : (
                <div className="rounded-lg border border-border bg-card px-3">
                  <ActivityLanes
                    activity={filteredActivity}
                    range={range}
                    nowSeconds={nowSeconds}
                    remoteNode={remoteNode}
                    tzOffsetSeconds={tzOffset}
                    onZoom={(zoomed) =>
                      setActivityWindow({ kind: 'fixed', ...zoomed })
                    }
                  />
                </div>
              )}
              <p className="text-xs text-muted-foreground">
                {filteredActivity.truncated ? (
                  <I18nText
                    text={
                      'Some routine runs are not marked one by one; the bars still count every run.'
                    }
                  />
                ) : (
                  <I18nText
                    text={
                      'Drag across a lane to zoom in. Click a bar for its runs or a mark for one run.'
                    }
                  />
                )}
              </p>
            </section>
          </div>
          {live && (
            <aside className="flex flex-col gap-6">
              <InProgress
                runs={inProgressRuns(filteredActivity)}
                remoteNode={remoteNode}
              />
              <UpNext
                upcoming={filteredActivity.upcoming}
                remoteNode={remoteNode}
                tzOffsetSeconds={tzOffset}
              />
            </aside>
          )}
        </div>
      </>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col gap-4 overflow-auto p-1">
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            value={filter}
            onChange={(event) => setFilter(event.target.value)}
            placeholder={ts('Filter workflows...')}
            aria-label={ts('Filter workflows')}
            className="h-9 w-56 pl-8"
          />
        </div>
        <Input
          type="date"
          aria-label={ts('Date')}
          value={dayValue}
          onChange={(event) => selectDay(event.target.value)}
          className="h-9 w-[150px]"
        />
        {live ? (
          <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
            <Activity className="h-3.5 w-3.5 text-success" aria-hidden="true" />
            <I18nText text={'Live, last 24 hours'} />
          </span>
        ) : (
          <Button
            variant="outline"
            onClick={() => setActivityWindow({ kind: 'live' })}
          >
            <I18nText text={'Back to live'} />
          </Button>
        )}
        <div className="flex-1" />
        <PathsCard />
        <RefreshButton
          onRefresh={async () => {
            await refresh();
          }}
        />
      </div>
      {body}
    </div>
  );
}

export default Dashboard;
