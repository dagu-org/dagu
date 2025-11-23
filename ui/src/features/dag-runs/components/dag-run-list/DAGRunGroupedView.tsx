import { ChevronDown, ChevronRight } from 'lucide-react';
import { useMemo, useState, useEffect } from 'react';
import { components, Status } from '../../../../api/v2/schema';
import { Tooltip, TooltipContent, TooltipTrigger } from '../../../../components/ui/tooltip';
import { useSimpleToast } from '../../../../components/ui/simple-toast';
import dayjs from '../../../../lib/dayjs';
import StatusChip from '../../../../ui/StatusChip';
import { DAGRunDetailsModal } from '../dag-run-details';

// Component for step names with tooltip (desktop) and toast (mobile)
function StepNamesDisplay({
  stepNames,
  label,
  colorClass,
  isMobile,
  toastTimeout = 3000,
}: {
  stepNames: string[];
  label: string;
  colorClass: string;
  isMobile: boolean;
  toastTimeout?: number;
}) {
  const { showToast } = useSimpleToast();

  if (!stepNames || stepNames.length === 0) {
    return null;
  }

  const hasMultiple = stepNames.length > 1;

  // Single step - just show it
  if (!hasMultiple) {
    return (
      <div className={`text-[10px] ${colorClass}`}>
        {label}: {stepNames[0]}
      </div>
    );
  }

  // Multiple steps - show "stepname +n" with tooltip/toast
  const handleClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (isMobile) {
      // On mobile, show toast with all step names
      const stepList = stepNames.map((name, idx) => `${idx + 1}. ${name}`).join('\n');
      showToast(`${label} steps:\n${stepList}`, toastTimeout);
    }
  };

  const stepListContent = (
    <div className="space-y-1">
      <div className="font-semibold text-xs mb-1">{label} steps:</div>
      {stepNames.map((stepName, idx) => (
        <div key={idx} className="text-xs">
          {idx + 1}. {stepName}
        </div>
      ))}
    </div>
  );

  if (isMobile) {
    // Mobile: clickable to show toast
    return (
      <div className={`text-[10px] ${colorClass}`}>
        <button
          onClick={handleClick}
          className="hover:underline cursor-pointer"
        >
          {label}: {stepNames[0]} +{stepNames.length - 1}
        </button>
      </div>
    );
  }

  // Desktop: tooltip on hover
  return (
    <div className={`text-[10px] ${colorClass}`}>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="cursor-help">
            {label}: {stepNames[0]} +{stepNames.length - 1}
          </span>
        </TooltipTrigger>
        <TooltipContent className="max-w-xs">
          {stepListContent}
        </TooltipContent>
      </Tooltip>
    </div>
  );
}

interface DAGRunGroupedViewProps {
  dagRuns: components['schemas']['DAGRunSummary'][];
}

interface GroupedDAGRuns {
  [dagName: string]: components['schemas']['DAGRunSummary'][];
}

function DAGRunGroupedView({ dagRuns }: DAGRunGroupedViewProps) {
  const [expandedGroups, setExpandedGroups] = useState<Set<string>>(new Set());
  const [isMobile, setIsMobile] = useState(false);
  const [selectedDAGRun, setSelectedDAGRun] = useState<{
    name: string;
    dagRunId: string;
  } | null>(null);

  // Configurable toast timeout (default 3 seconds)
  const toastTimeout = 3000;

  // Check if mobile screen
  useEffect(() => {
    const checkScreenSize = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkScreenSize();
    window.addEventListener('resize', checkScreenSize);
    return () => window.removeEventListener('resize', checkScreenSize);
  }, []);

  // Group DAG runs by name
  const groupedDAGRuns = useMemo(() => {
    const groups: GroupedDAGRuns = {};
    dagRuns.forEach((dagRun) => {
      if (!groups[dagRun.name]) {
        groups[dagRun.name] = [];
      }
      const runsList = groups[dagRun.name];
      if (runsList) {
        runsList.push(dagRun);
      }
    });
    // Sort runs within each group by queuedAt descending (most recent first)
    Object.keys(groups).forEach((dagName) => {
      const runs = groups[dagName];
      if (runs) {
        runs.sort((a, b) => {
          return (
            dayjs(b.queuedAt || '').valueOf() -
            dayjs(a.queuedAt || '').valueOf()
          );
        });
      }
    });
    return groups;
  }, [dagRuns]);

  const toggleGroup = (dagName: string) => {
    setExpandedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(dagName)) {
        next.delete(dagName);
      } else {
        next.add(dagName);
      }
      return next;
    });
  };

  // Calculate duration between start and finish times
  const calculateDuration = (
    startedAt: string,
    finishedAt: string | null,
    status: number
  ): string => {
    if (!startedAt) {
      return '-';
    }

    if (status === Status.Running && !finishedAt) {
      const start = dayjs(startedAt);
      const now = dayjs();
      const durationMs = now.diff(start);
      return formatDuration(durationMs);
    }

    if (finishedAt) {
      const start = dayjs(startedAt);
      const end = dayjs(finishedAt);
      const durationMs = end.diff(start);
      return formatDuration(durationMs);
    }

    return '-';
  };

  const formatDuration = (durationMs: number): string => {
    const seconds = Math.floor(durationMs / 1000);

    if (seconds < 60) {
      return `${seconds}s`;
    }

    const minutes = Math.floor(seconds / 60);
    const remainingSeconds = seconds % 60;

    if (minutes < 60) {
      return `${minutes}m ${remainingSeconds}s`;
    }

    const hours = Math.floor(minutes / 60);
    const remainingMinutes = minutes % 60;

    return `${hours}h ${remainingMinutes}m ${remainingSeconds}s`;
  };

  // Get the most recent run for each group for summary display
  const getGroupSummary = (runs: components['schemas']['DAGRunSummary'][]) => {
    if (!runs || runs.length === 0) {
      return null;
    }

    const latestRun = runs[0];
    if (!latestRun) {
      return null;
    }

    const failedCount = runs.filter((r) => r.status === Status.Failed).length;
    const abortedCount = runs.filter((r) => r.status === Status.Aborted).length;
    const queuedCount = runs.filter((r) => r.status === Status.Queued).length;
    const runningCount = runs.filter((r) => r.status === Status.Running).length;

    // Check if all runs have the same status
    const firstStatus = runs[0]?.status;
    const allSameStatus = firstStatus !== undefined && runs.every((r) => r.status === firstStatus);

    return {
      latestRun,
      failedCount,
      abortedCount,
      queuedCount,
      runningCount,
      totalCount: runs.length,
      allSameStatus,
      uniformStatus: allSameStatus ? firstStatus : null,
    };
  };

  // Empty state component
  const EmptyState = () => (
    <div className="flex flex-col items-center justify-center py-12 px-4 border rounded-md bg-white dark:bg-zinc-900">
      <div className="text-6xl mb-4">🔍</div>
      <h3 className="text-lg font-normal text-gray-900 dark:text-gray-100 mb-2">
        No DAG runs found
      </h3>
      <p className="text-sm text-gray-500 dark:text-gray-400 text-center max-w-md mb-4">
        There are no DAG runs matching your current filters. Try adjusting your
        search criteria or date range.
      </p>
    </div>
  );

  if (dagRuns.length === 0) {
    return <EmptyState />;
  }

  const sortedDagNames = Object.keys(groupedDAGRuns).sort();

  return (
    <div className="border rounded-md bg-white dark:bg-zinc-900">
      <div className="divide-y divide-border">
        {sortedDagNames.map((dagName) => {
          const runs = groupedDAGRuns[dagName];
          if (!runs) return null;

          const summary = getGroupSummary(runs);
          if (!summary) return null;

          const isExpanded = expandedGroups.has(dagName);

          return (
            <div key={dagName}>
              {/* Group Header */}
              <div
                className="flex items-center justify-between p-3 cursor-pointer hover:bg-muted/30 transition-colors"
                onClick={() => toggleGroup(dagName)}
              >
                <div className="flex items-center gap-2 flex-1 min-w-0">
                  <button
                    className="flex-shrink-0 p-1 hover:bg-muted rounded"
                    onClick={(e) => {
                      e.stopPropagation();
                      toggleGroup(dagName);
                    }}
                  >
                    {isExpanded ? (
                      <ChevronDown
                        size={16}
                        className="text-muted-foreground"
                      />
                    ) : (
                      <ChevronRight
                        size={16}
                        className="text-muted-foreground"
                      />
                    )}
                  </button>
                  <div className="flex-1 min-w-0">
                    <div className="font-medium text-sm truncate">
                      {dagName}
                    </div>
                    <div className="text-xs text-muted-foreground">
                      {summary.totalCount} run
                      {summary.totalCount !== 1 ? 's' : ''}
                      {!summary.allSameStatus && (
                        <>
                          {summary.runningCount > 0 && (
                            <span className="ml-2">
                              {summary.runningCount} running
                            </span>
                          )}
                          {summary.failedCount > 0 && (
                            <span className="ml-2">
                              {summary.failedCount} failed
                            </span>
                          )}
                          {summary.abortedCount > 0 && (
                            <span className="ml-2">
                              {summary.abortedCount} aborted
                            </span>
                          )}
                          {summary.queuedCount > 0 && (
                            <span className="ml-2">
                              {summary.queuedCount} queued
                            </span>
                          )}
                        </>
                      )}
                    </div>
                  </div>
                </div>
                <div className="flex items-center gap-2 flex-shrink-0">
                  {summary.allSameStatus && summary.uniformStatus != null ? (
                    <StatusChip status={summary.uniformStatus} size="xs">
                      {summary.latestRun.statusLabel}
                    </StatusChip>
                  ) : (
                    <StatusChip status={undefined} size="xs">
                      Mixed
                    </StatusChip>
                  )}
                </div>
              </div>

              {/* Expanded Runs List */}
              {isExpanded && (
                <div className="bg-muted/10">
                  <div className="divide-y divide-border/50">
                    {runs.map((dagRun) => (
                      <div
                        key={dagRun.dagRunId}
                        className="px-3 py-2 pl-11 hover:bg-muted/20 cursor-pointer transition-colors text-xs"
                        onClick={(e) => {
                          if (e.ctrlKey || e.metaKey) {
                            window.open(
                              `/dag-runs/${dagRun.name}/${dagRun.dagRunId}`,
                              '_blank'
                            );
                          } else {
                            setSelectedDAGRun({
                              name: dagRun.name,
                              dagRunId: dagRun.dagRunId,
                            });
                          }
                        }}
                      >
                        <div className="flex items-start justify-between gap-3">
                          <div className="flex-1 min-w-0 space-y-1">
                            <div className="font-mono text-muted-foreground truncate">
                              {dagRun.dagRunId}
                            </div>
                            <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs">
                              <div className="whitespace-nowrap">
                                <span className="text-muted-foreground">
                                  Queued:{' '}
                                </span>
                                {dagRun.queuedAt || '-'}
                              </div>
                              <div className="whitespace-nowrap">
                                <span className="text-muted-foreground">
                                  Started:{' '}
                                </span>
                                {dagRun.startedAt || '-'}
                              </div>
                              <div className="flex items-center gap-1 whitespace-nowrap">
                                <span className="text-muted-foreground">
                                  Duration:{' '}
                                </span>
                                {calculateDuration(
                                  dagRun.startedAt,
                                  dagRun.finishedAt,
                                  dagRun.status
                                )}
                                {dagRun.status === Status.Running &&
                                  dagRun.startedAt && (
                                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-lime-500 animate-pulse" />
                                  )}
                              </div>
                            </div>
                          </div>
                          <div className="flex-shrink-0 mt-0.5">
                            <div className="flex flex-col items-end gap-1">
                              <StatusChip status={dagRun.status} size="xs">
                                {dagRun.statusLabel}
                              </StatusChip>
                              <StepNamesDisplay
                                stepNames={dagRun.runningStepNames || []}
                                label="Running"
                                colorClass="text-muted-foreground text-right"
                                isMobile={isMobile}
                                toastTimeout={toastTimeout}
                              />
                              <StepNamesDisplay
                                stepNames={dagRun.failedStepNames || []}
                                label="Failed"
                                colorClass="text-destructive text-right"
                                isMobile={isMobile}
                                toastTimeout={toastTimeout}
                              />
                            </div>
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* DAG Run Details Modal */}
      {selectedDAGRun && (
        <DAGRunDetailsModal
          name={selectedDAGRun.name}
          dagRunId={selectedDAGRun.dagRunId}
          isOpen={!!selectedDAGRun}
          onClose={() => {
            setSelectedDAGRun(null);
          }}
        />
      )}
    </div>
  );
}

export default DAGRunGroupedView;
