/**
 * DAGRunActions component provides action buttons for DAGRun operations (stop, retry).
 *
 * @module features/dagRuns/components/common
 */
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import dayjs from '@/lib/dayjs';
import { RefreshCw, Square, X } from 'lucide-react';
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
import { useClient } from '../../../../hooks/api';
import ConfirmModal from '../../../../ui/ConfirmModal';
import LabeledItem from '../../../../ui/LabeledItem';
import StatusChip from '../../../../ui/StatusChip';

/**
 * Props for the DAGRunActions component
 */
type Props = {
  /** Current status of the DAGRun */
  dagRun?:
    | components['schemas']['DAGRunSummary']
    | components['schemas']['DAGRunDetails'];
  /** Name of the DAGRun */
  name: string;
  /** Whether to show text labels on buttons */
  label?: boolean;
  /** Function to refresh data after actions */
  refresh?: () => void;
  /** Display mode: 'compact' for icon-only, 'full' for text+icon buttons */
  displayMode?: 'compact' | 'full';
  /** Whether this is a root level dagRun (controls availability of retry/stop actions) */
  isRootLevel?: boolean;
};

/**
 * DAGRunActions component provides buttons to stop and retry DAGRun executions
 */
function DAGRunActions({
  dagRun,
  name,
  refresh,
  displayMode = 'compact',
  isRootLevel = true,
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);
  const [isDequeueModal, setIsDequeueModal] = React.useState(false);

  // Retry-as-new modal state
  const [retryAsNew, setRetryAsNew] = React.useState(false);
  const [newRunId, setNewRunId] = React.useState('');
  const [dagNameOverride, setDagNameOverride] = React.useState('');

  const client = useClient();

  /**
   * Reload DAGRun data after an action is performed
   */
  const reloadData = () => {
    if (refresh) {
      refresh();
    }
  };

  // Determine which buttons should be enabled based on current status and root level
  const buttonState = {
    stop: isRootLevel && dagRun?.status === Status.Running, // Running and at root level
    retry: isRootLevel && dagRun?.status !== Status.Running && dagRun?.status !== Status.Queued && dagRun?.dagRunId !== '', // Not running, not queued, has dagRunId, and at root level
    dequeue: isRootLevel && dagRun?.status === Status.Queued, // Queued and at root level
  };

  if (!dagRun || !config.permissions.runDags) {
    return <></>;
  }

  return (
    <TooltipProvider delayDuration={300}>
      <div
        className={`flex items-center ${displayMode === 'compact' ? 'space-x-1' : 'space-x-2'}`}
      >
        {/* Stop Button */}
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              {displayMode === 'compact' ? (
                <Button
                  variant="ghost"
                  size="icon"
                  disabled={!buttonState['stop']}
                  onClick={() => setIsStopModal(true)}
                  className="h-8 w-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <Square className="h-4 w-4" />
                  <span className="sr-only">Stop</span>
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!buttonState['stop']}
                  onClick={() => setIsStopModal(true)}
                  className="h-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <Square className="mr-2 h-4 w-4" />
                  Stop
                </Button>
              )}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{isRootLevel ? 'Stop DAGRun execution' : 'Stop action only available at root dagRun level'}</p>
          </TooltipContent>
        </Tooltip>

        {/* Retry Button */}
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              {displayMode === 'compact' ? (
                <Button
                  variant="ghost"
                  size="icon"
                  disabled={!buttonState['retry']}
                  onClick={() => setIsRetryModal(true)}
                  className="h-8 w-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <RefreshCw className="h-4 w-4" />
                  <span className="sr-only">Retry</span>
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!buttonState['retry']}
                  onClick={() => setIsRetryModal(true)}
                  className="h-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <RefreshCw className="mr-2 h-4 w-4" />
                  Retry
                </Button>
              )}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{isRootLevel ? 'Retry DAGRun execution' : 'Retry action only available at root dagRun level'}</p>
          </TooltipContent>
        </Tooltip>

        {/* Dequeue Button */}
        <Tooltip>
          <TooltipTrigger asChild>
            <span>
              {displayMode === 'compact' ? (
                <Button
                  variant="ghost"
                  size="icon"
                  disabled={!buttonState['dequeue']}
                  onClick={() => setIsDequeueModal(true)}
                  className="h-8 w-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <X className="h-4 w-4" />
                  <span className="sr-only">Dequeue</span>
                </Button>
              ) : (
                <Button
                  variant="outline"
                  size="sm"
                  disabled={!buttonState['dequeue']}
                  onClick={() => setIsDequeueModal(true)}
                  className="h-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
                >
                  <X className="mr-2 h-4 w-4" />
                  Dequeue
                </Button>
              )}
            </span>
          </TooltipTrigger>
          <TooltipContent>
            <p>{isRootLevel ? 'Remove DAGRun from queue' : 'Dequeue action only available at root dagRun level'}</p>
          </TooltipContent>
        </Tooltip>

        {/* Stop Confirmation Modal */}
        <ConfirmModal
          title="Confirmation"
          buttonText="Stop"
          visible={isStopModal}
          dismissModal={() => setIsStopModal(false)}
          onSubmit={async () => {
            setIsStopModal(false);
            const { error } = await client.POST('/dag-runs/{name}/{dagRunId}/stop', {
              params: {
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
                path: {
                  name: name,
                  dagRunId: dagRun.dagRunId,
                },
              },
            });
            if (error) {
              console.error('Stop API error:', error);
              alert(error.message || 'An error occurred');
              return;
            }
            reloadData();
          }}
        >
          <div>Do you really want to stop this dagRun?</div>
        </ConfirmModal>

        {/* Retry Confirmation Modal */}
        <ConfirmModal
          title={retryAsNew ? "Retry DAG Run as New" : "Confirmation"}
          buttonText="Retry"
          visible={isRetryModal}
          dismissModal={() => {
            setIsRetryModal(false);
            setRetryAsNew(false);
            setNewRunId('');
            setDagNameOverride('');
          }}
          onSubmit={async () => {
            setIsRetryModal(false);

            if (retryAsNew) {
              // Use reschedule endpoint for retry-as-new
              const { error, data } = await client.POST('/dag-runs/{name}/{dagRunId}/reschedule', {
                params: {
                  path: {
                    name: name,
                    dagRunId: dagRun.dagRunId,
                  },
                  query: {
                    remoteNode: appBarContext.selectedRemoteNode || 'local',
                  },
                },
                body: {
                  dagRunId: newRunId || undefined, // Auto-generate if empty
                  ...(dagNameOverride ? { dagName: dagNameOverride } : {}), // Use original if empty
                  singleton: false,
                },
              });
              if (error) {
                alert(error.message || 'An error occurred');
                // Reset state on error
                setRetryAsNew(false);
                setNewRunId('');
                setDagNameOverride('');
                return;
              }
              // Show success message with new run ID
              if (data?.dagRunId) {
                alert(`New DAG run created with ID: ${data.dagRunId}${data.queued ? ' (queued)' : ''}`);
              }
              // Reset state after success
              setRetryAsNew(false);
              setNewRunId('');
              setDagNameOverride('');
            } else {
              // Use retry endpoint for regular retry
              const { error } = await client.POST('/dag-runs/{name}/{dagRunId}/retry', {
                params: {
                  path: {
                    name: name,
                    dagRunId: dagRun.dagRunId,
                  },
                  query: {
                    remoteNode: appBarContext.selectedRemoteNode || 'local',
                  },
                },
                body: {
                  dagRunId: dagRun.dagRunId,
                },
              });
              if (error) {
                alert(error.message || 'An error occurred');
                return;
              }
            }
            reloadData();
          }}
        >
          {/* Modal content structure */}
          <div className="space-y-3">
            <p className="mb-2">
              Do you really want to retry the following execution?
            </p>
            <LabeledItem label="DAGRun-Name">
              <span className="font-mono text-sm">
                {dagRun?.name || 'N/A'}
              </span>
            </LabeledItem>
            <LabeledItem label="DAGRun-ID">
              <span className="font-mono text-sm">
                {dagRun?.dagRunId || 'N/A'}
              </span>
            </LabeledItem>
            {dagRun?.startedAt && (
              <LabeledItem label="Started At">
                <span className="text-sm">
                  {dayjs(dagRun.startedAt).format('YYYY-MM-DD HH:mm:ss Z')}
                </span>
              </LabeledItem>
            )}
            {dagRun?.status !== undefined && (
              <LabeledItem label="Status">
                <StatusChip status={dagRun.status} size="sm">
                  {dagRun.statusLabel || ''}
                </StatusChip>
              </LabeledItem>
            )}

            {/* Retry as new checkbox */}
            <div className="flex items-center space-x-2 pt-2 border-t">
              <Checkbox
                id="retry-as-new"
                checked={retryAsNew}
                onCheckedChange={(checked) => setRetryAsNew(checked as boolean)}
                className="border-gray-400 dark:border-gray-500"
              />
              <Label htmlFor="retry-as-new" className="cursor-pointer text-sm">
                Retry as new DAG-run
              </Label>
            </div>

            {/* Conditional inputs when retry-as-new is checked */}
            {retryAsNew && (
              <div className="space-y-3 pt-2">
                <div className="space-y-1.5">
                  <Label htmlFor="new-dagrun-id" className="text-sm">
                    New DAG-Run ID (optional)
                  </Label>
                  <Input
                    id="new-dagrun-id"
                    placeholder="Auto-generated if empty"
                    value={newRunId}
                    onChange={(e) => setNewRunId(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="dag-name-override" className="text-sm">
                    DAG Name Override (optional)
                  </Label>
                  <Input
                    id="dag-name-override"
                    placeholder={`Leave empty to use: ${dagRun?.name || 'original'}`}
                    value={dagNameOverride}
                    onChange={(e) => setDagNameOverride(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
              </div>
            )}
          </div>
        </ConfirmModal>

        {/* Dequeue Confirmation Modal */}
        <ConfirmModal
          title="Confirmation"
          buttonText="Dequeue"
          visible={isDequeueModal}
          dismissModal={() => setIsDequeueModal(false)}
          onSubmit={async () => {
            setIsDequeueModal(false);

            const { error } = await client.GET('/dag-runs/{name}/{dagRunId}/dequeue', {
              params: {
                path: {
                  name: name,
                  dagRunId: dagRun.dagRunId,
                },
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
              },
            });
            if (error) {
              alert(error.message || 'An error occurred');
              return;
            }
            reloadData();
          }}
        >
          <div>
            <p className="mb-2">
              Do you really want to dequeue the following dagRun?
            </p>
            <LabeledItem label="DAGRun-Name">
              <span className="font-mono text-sm">
                {dagRun?.name || 'N/A'}
              </span>
            </LabeledItem>
            <LabeledItem label="DAGRun-ID">
              <span className="font-mono text-sm">
                {dagRun?.dagRunId || 'N/A'}
              </span>
            </LabeledItem>
            {dagRun?.status !== undefined && (
              <LabeledItem label="Status">
                <StatusChip status={dagRun.status} size="sm">
                  {dagRun.statusLabel || ''}
                </StatusChip>
              </LabeledItem>
            )}
          </div>
        </ConfirmModal>
      </div>
    </TooltipProvider>
  );
}

export default DAGRunActions;
