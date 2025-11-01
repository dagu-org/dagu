/**
 * DAGRunActions component provides action buttons for DAGRun operations (stop, retry).
 *
 * @module features/dagRuns/components/common
 */
import { Button } from '@/components/ui/button';
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
          title="Confirmation"
          buttonText="Retry"
          visible={isRetryModal}
          dismissModal={() => setIsRetryModal(false)}
          onSubmit={async () => {
            setIsRetryModal(false);

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
            reloadData();
          }}
        >
          {/* Modal content structure */}
          <div>
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
