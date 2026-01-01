import {
  ChevronDown,
  ChevronRight,
  Trash2,
} from 'lucide-react';
import React from 'react';
import type { components } from '../../../api/v2/schema';
import { Button } from '../../../components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '../../../components/ui/tooltip';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useConfig } from '../../../contexts/ConfigContext';
import { useClient } from '../../../hooks/api';
import dayjs from '../../../lib/dayjs';
import { cn } from '../../../lib/utils';
import ConfirmModal from '../../../ui/ConfirmModal';
import StatusChip from '../../../ui/StatusChip';

interface QueueCardProps {
  queue: components['schemas']['Queue'];
  isSelected?: boolean;
  onDAGRunClick: (dagRun: components['schemas']['DAGRunSummary']) => void;
  onQueueCleared?: () => void;
}

function QueueCard({
  queue,
  isSelected,
  onDAGRunClick,
  onQueueCleared,
}: QueueCardProps) {
  const config = useConfig();
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const [isExpanded, setIsExpanded] = React.useState(true);
  const [isClearing, setIsClearing] = React.useState(false);
  const [showClearConfirm, setShowClearConfirm] = React.useState(false);

  const toggleExpanded = () => setIsExpanded(!isExpanded);

  const handleClearQueue = async () => {
    setIsClearing(true);
    try {
      const queuedRuns = queue.queued || [];
      await Promise.all(
        queuedRuns.map(async (dagRun) => {
          try {
            await client.GET('/dag-runs/{name}/{dagRunId}/dequeue', {
              params: {
                path: {
                  name: dagRun.name,
                  dagRunId: dagRun.dagRunId,
                },
                query: {
                  remoteNode: appBarContext?.selectedRemoteNode || 'local',
                },
              },
            });
          } catch (error) {
            console.error(
              `Failed to dequeue ${dagRun.name}:${dagRun.dagRunId}:`,
              error
            );
          }
        })
      );
      if (onQueueCleared) {
        onQueueCleared();
      }
    } catch (error) {
      console.error('Failed to clear queue:', error);
    } finally {
      setIsClearing(false);
      setShowClearConfirm(false);
    }
  };

  const utilization = React.useMemo(() => {
    if (queue.type !== 'global' || !queue.maxConcurrency) return null;
    const running = queue.running?.length || 0;
    return Math.round((running / queue.maxConcurrency) * 100);
  }, [queue]);

  const formatDateTime = (datetime: string) => {
    if (!datetime) return 'N/A';
    if (config.tzOffsetInSec !== undefined) {
      return dayjs(datetime)
        .utcOffset(config.tzOffsetInSec / 60)
        .format('MMM D, HH:mm:ss');
    }
    return dayjs(datetime).format('MMM D, HH:mm:ss');
  };

  const DAGRunRow: React.FC<{
    dagRun: components['schemas']['DAGRunSummary'];
    showQueuedAt?: boolean;
  }> = ({ dagRun, showQueuedAt = false }) => (
    <tr
      onClick={() => onDAGRunClick(dagRun)}
      className="cursor-pointer hover:bg-muted/30 transition-colors"
    >
      <td className="py-1.5 px-2 text-xs font-medium">{dagRun.name}</td>
      <td className="py-1.5 px-2">
        <StatusChip status={dagRun.status} size="xs">
          {dagRun.statusLabel}
        </StatusChip>
      </td>
      <td className="py-1.5 px-2 text-xs text-muted-foreground tabular-nums">
        {showQueuedAt
          ? dagRun.queuedAt
            ? formatDateTime(dagRun.queuedAt)
            : 'N/A'
          : dagRun.startedAt
            ? formatDateTime(dagRun.startedAt)
            : 'N/A'}
      </td>
      <td className="py-1.5 px-2 text-xs text-muted-foreground font-mono">
        {dagRun.dagRunId}
      </td>
    </tr>
  );

  return (
    <div
      className={cn(
        'border rounded-lg bg-card transition-all duration-200',
        isSelected && 'ring-1 ring-foreground/20'
      )}
    >
      {/* Queue Header */}
      <div
        className="px-3 py-2 cursor-pointer hover:bg-muted/30 transition-colors"
        onClick={toggleExpanded}
      >
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex items-center gap-2">
              {isExpanded ? (
                <ChevronDown className="h-4 w-4 text-muted-foreground" />
              ) : (
                <ChevronRight className="h-4 w-4 text-muted-foreground" />
              )}
              <span className="font-medium text-sm">{queue.name}</span>
              <span className="text-[10px] px-1.5 py-0.5 rounded bg-muted text-muted-foreground">
                {queue.type}
              </span>
            </div>

            {/* Utilization bar for global queues */}
            {queue.type === 'global' && queue.maxConcurrency && (
              <div className="flex items-center gap-2">
                <div className="w-12 h-1 bg-muted rounded-full overflow-hidden">
                  <div
                    className="h-full transition-all duration-300 bg-foreground/40"
                    style={{ width: `${utilization || 0}%` }}
                  />
                </div>
                <span className="text-xs text-muted-foreground tabular-nums">
                  {queue.running?.length || 0}/{queue.maxConcurrency}
                </span>
              </div>
            )}
          </div>

          {/* Summary counts */}
          <div className="flex items-center gap-4 text-xs text-muted-foreground">
            <div className="flex items-baseline gap-1">
              <span className="text-sm font-light tabular-nums text-foreground">
                {queue.running?.length || 0}
              </span>
              <span>running</span>
            </div>
            <div className="flex items-baseline gap-1">
              <span className={`text-sm font-light tabular-nums ${(queue.queued?.length || 0) > 0 ? 'text-foreground' : 'text-muted-foreground/50'}`}>
                {queue.queued?.length || 0}
              </span>
              <span>queued</span>
            </div>
            {utilization !== null && (
              <div className="flex items-baseline gap-1">
                <span className="text-sm font-light tabular-nums text-foreground">
                  {utilization}%
                </span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t">
          {/* Running DAGs */}
          {queue.running && queue.running.length > 0 && (
            <div>
              <div className="px-3 py-2 bg-muted/20">
                <div className="flex items-center gap-2 mb-2">
                  <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Running ({queue.running.length})
                  </span>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-border">
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          DAG
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Status
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Started
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Run ID
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border/50">
                      {queue.running.map((dagRun) => (
                        <DAGRunRow key={dagRun.dagRunId} dagRun={dagRun} />
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}

          {/* Queued DAGs */}
          {queue.queued && queue.queued.length > 0 && (
            <div className={queue.running && queue.running.length > 0 ? 'border-t' : ''}>
              <div className="px-3 py-2 bg-muted/10">
                <div className="flex items-center justify-between mb-2">
                  <span className="text-xs font-medium text-muted-foreground uppercase tracking-wide">
                    Queued ({queue.queued.length})
                  </span>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={(e) => {
                          e.stopPropagation();
                          setShowClearConfirm(true);
                        }}
                        disabled={isClearing}
                        className="h-6 px-2 text-muted-foreground hover:text-foreground"
                      >
                        <Trash2
                          className={cn('h-3 w-3', isClearing && 'animate-pulse')}
                        />
                        <span className="ml-1 text-xs">Clear</span>
                      </Button>
                    </TooltipTrigger>
                    <TooltipContent>
                      <p>Remove all queued DAG runs</p>
                    </TooltipContent>
                  </Tooltip>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-border">
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          DAG
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Status
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Queued
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-muted-foreground">
                          Run ID
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-border/50">
                      {queue.queued.map((dagRun) => (
                        <DAGRunRow
                          key={dagRun.dagRunId}
                          dagRun={dagRun}
                          showQueuedAt={true}
                        />
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            </div>
          )}

          {/* Empty state */}
          {(!queue.running || queue.running.length === 0) &&
            (!queue.queued || queue.queued.length === 0) && (
              <div className="px-3 py-4 text-center text-muted-foreground text-xs">
                No DAGs running or queued
              </div>
            )}
        </div>
      )}

      {/* Clear Queue Confirmation Modal */}
      <ConfirmModal
        title="Clear Queue"
        buttonText="Clear Queue"
        visible={showClearConfirm}
        dismissModal={() => setShowClearConfirm(false)}
        onSubmit={handleClearQueue}
      >
        <div className="space-y-2">
          <p className="text-sm">
            Remove all queued DAG runs from "{queue.name}"?
          </p>
          <p className="text-xs text-muted-foreground">
            {queue.queued?.length || 0} DAG runs will be removed. This cannot be undone.
          </p>
        </div>
      </ConfirmModal>
    </div>
  );
}

export default QueueCard;
