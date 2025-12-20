import {
  BarChart3,
  ChevronDown,
  ChevronRight,
  Clock,
  GitBranch,
  Play,
  Settings,
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
  const [isExpanded, setIsExpanded] = React.useState(false);
  const [isClearing, setIsClearing] = React.useState(false);
  const [showClearConfirm, setShowClearConfirm] = React.useState(false);

  const toggleExpanded = () => setIsExpanded(!isExpanded);

  const handleClearQueue = async () => {
    setIsClearing(true);
    try {
      // Get all queued DAG runs from this specific queue
      const queuedRuns = queue.queued || [];

      // Dequeue all queued DAG runs
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

      // Notify parent component to refresh
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

  // Calculate utilization for global queues
  const utilization = React.useMemo(() => {
    if (queue.type !== 'global' || !queue.maxConcurrency) return null;
    const running = queue.running?.length || 0;
    return Math.round((running / queue.maxConcurrency) * 100);
  }, [queue]);

  // Format datetime with timezone
  const formatDateTime = (datetime: string) => {
    if (!datetime) return 'N/A';

    if (config.tzOffsetInSec !== undefined) {
      return dayjs(datetime)
        .utcOffset(config.tzOffsetInSec / 60)
        .format('MMM D, HH:mm:ss [GMT]Z');
    }
    return dayjs(datetime).format('MMM D, HH:mm:ss');
  };

  // DAG Run row component
  const DAGRunRow: React.FC<{
    dagRun: components['schemas']['DAGRunSummary'];
    showQueuedAt?: boolean;
  }> = ({ dagRun, showQueuedAt = false }) => (
    <tr
      onClick={() => onDAGRunClick(dagRun)}
      className="cursor-pointer hover:bg-muted/30 transition-colors"
    >
      <td className="py-1 px-2 text-xs font-medium">{dagRun.name}</td>
      <td className="py-1 px-2">
        <StatusChip status={dagRun.status} size="xs">
          {dagRun.statusLabel}
        </StatusChip>
      </td>
      <td className="py-1 px-2 text-xs text-muted-foreground">
        {showQueuedAt
          ? dagRun.queuedAt
            ? formatDateTime(dagRun.queuedAt)
            : 'N/A'
          : dagRun.startedAt
            ? formatDateTime(dagRun.startedAt)
            : 'N/A'}
      </td>
      <td className="py-1 px-2 text-xs text-muted-foreground">
        {dagRun.dagRunId}
      </td>
    </tr>
  );

  return (
    <div
      className={cn(
        'border rounded-lg bg-card transition-all duration-200',
        isSelected && 'ring-2 ring-primary/20 bg-muted/10'
      )}
    >
      {/* Queue Header */}
      <div
        className="p-3 cursor-pointer hover:bg-muted/10 transition-colors"
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
              <div className="flex items-center gap-2">
                {queue.type === 'global' ? (
                  <Settings className="h-4 w-4 text-primary" />
                ) : (
                  <GitBranch className="h-4 w-4 text-muted-foreground" />
                )}
                <span className="font-semibold text-sm">{queue.name}</span>
                <span className="text-xs px-2 py-0.5 rounded-full bg-muted text-muted-foreground">
                  {queue.type}
                </span>
              </div>
            </div>

            {/* Utilization bar for global queues */}
            {queue.type === 'global' && queue.maxConcurrency && (
              <div className="flex items-center gap-2">
                <div className="w-16 h-2 bg-muted rounded-full overflow-hidden">
                  <div
                    className={cn(
                      'h-full transition-all duration-300',
                      utilization && utilization > 80
                        ? 'bg-warning'
                        : utilization && utilization > 60
                          ? 'bg-warning'
                          : 'bg-success'
                    )}
                    style={{ width: `${utilization || 0}%` }}
                  />
                </div>
                <span className="text-xs text-muted-foreground">
                  {queue.running?.length || 0}/{queue.maxConcurrency}
                </span>
              </div>
            )}
          </div>

          {/* Summary counts */}
          <div className="flex items-center gap-4">
            <div className="flex items-center gap-1">
              <Play className="h-3 w-3 text-success" />
              <span className="text-sm font-medium">
                {queue.running?.length || 0}
              </span>
              <span className="text-xs text-muted-foreground">running</span>
            </div>
            <div className="flex items-center gap-1">
              <Clock className="h-3 w-3 text-info" />
              <span className="text-sm font-medium">
                {queue.queued?.length || 0}
              </span>
              <span className="text-xs text-muted-foreground">queued</span>
            </div>
            {utilization !== null && (
              <div className="flex items-center gap-1">
                <BarChart3 className="h-3 w-3 text-warning" />
                <span className="text-sm font-medium">{utilization}%</span>
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Expanded Content */}
      {isExpanded && (
        <div className="border-t">
          {/* Queue Details */}
          <div className="p-3 bg-muted/20">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 text-xs">
              <div>
                <span className="text-muted-foreground">Type:</span>
                <div className="font-medium">{queue.type}</div>
              </div>
              {queue.maxConcurrency && (
                <div>
                  <span className="text-muted-foreground">
                    Max Concurrency:
                  </span>
                  <div className="font-medium">{queue.maxConcurrency}</div>
                </div>
              )}
              <div>
                <span className="text-muted-foreground">Total Active:</span>
                <div className="font-medium">
                  {(queue.running?.length || 0) + (queue.queued?.length || 0)}
                </div>
              </div>
              {utilization !== null && (
                <div>
                  <span className="text-muted-foreground">Utilization:</span>
                  <div className="font-medium">{utilization}%</div>
                </div>
              )}
            </div>
          </div>

          {/* Running DAGs */}
          {queue.running && queue.running.length > 0 && (
            <div className="border-t">
              <div className="p-2 bg-success-muted">
                <div className="flex items-center gap-2 mb-2">
                  <Play className="h-4 w-4 text-success" />
                  <h4 className="text-sm font-semibold text-success">
                    Running DAGs ({queue.running.length})
                  </h4>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-success/30">
                        <th className="text-left py-1 px-2 font-medium text-success">
                          DAG Name
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-success">
                          Status
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-success">
                          Started At
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-success">
                          Run ID
                        </th>
                      </tr>
                    </thead>
                    <tbody>
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
          <div className="border-t">
            <div className="p-2 bg-info-muted">
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  <Clock className="h-4 w-4 text-info" />
                  <h4 className="text-sm font-semibold text-info">
                    Queued DAGs ({queue.queued?.length || 0})
                  </h4>
                </div>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={(e) => {
                        e.stopPropagation();
                        setShowClearConfirm(true);
                      }}
                      disabled={
                        isClearing || !queue.queued || queue.queued.length === 0
                      }
                      className="h-6 px-2"
                    >
                      <Trash2
                        className={cn('h-3 w-3', isClearing && 'animate-pulse')}
                      />
                      <span className="ml-1 text-xs">Clear</span>
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>
                    <p>Remove all queued DAG runs from this queue</p>
                  </TooltipContent>
                </Tooltip>
              </div>
              {queue.queued && queue.queued.length > 0 ? (
                <div className="overflow-x-auto">
                  <table className="w-full text-xs">
                    <thead>
                      <tr className="border-b border-info/30">
                        <th className="text-left py-1 px-2 font-medium text-info">
                          DAG Name
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-info">
                          Status
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-info">
                          Queued At
                        </th>
                        <th className="text-left py-1 px-2 font-medium text-info">
                          Run ID
                        </th>
                      </tr>
                    </thead>
                    <tbody>
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
              ) : (
                <div className="text-center py-4 text-muted-foreground">
                  <p className="text-sm">No queued DAG runs</p>
                </div>
              )}
            </div>
          </div>

          {/* Empty state */}
          {(!queue.running || queue.running.length === 0) &&
            (!queue.queued || queue.queued.length === 0) && (
              <div className="p-4 text-center text-muted-foreground">
                <p className="text-sm">
                  No DAGs currently running or queued in this queue.
                </p>
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
            This will remove all queued DAG runs from the "{queue.name}" queue.
            This action cannot be undone.
          </p>
          <p className="text-xs text-muted-foreground">
            Currently {queue.queued?.length || 0} DAG runs are queued in this
            queue.
          </p>
        </div>
      </ConfirmModal>
    </div>
  );
}

export default QueueCard;
