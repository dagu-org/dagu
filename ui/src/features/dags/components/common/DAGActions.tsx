/**
 * DAGActions component provides action buttons for DAG operations (start, stop, retry).
 *
 * @module features/dags/components/common
 */
import React from 'react';
import { Play, Square, RefreshCw } from 'lucide-react'; // Import lucide icons
import { StartDAGModal } from '../dag-execution';
import ConfirmModal from '../../../../ui/ConfirmModal';
import LabeledItem from '../../../../ui/LabeledItem';
import { components } from '../../../../api/v2/schema';
import { useClient } from '../../../../hooks/api';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { Button } from '@/components/ui/button'; // Import Shadcn Button
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'; // Import Shadcn Tooltip

/**
 * Props for the DAGActions component
 */
type Props = {
  /** Current status of the DAG */
  status?:
    | components['schemas']['RunSummary']
    | components['schemas']['RunDetails'];
  /** File ID of the DAG */
  fileId: string;
  /** DAG definition */
  dag?: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  /** Whether to show text labels on buttons */
  label?: boolean;
  /** Function to refresh data after actions */
  refresh?: () => void;
  /** Display mode: 'compact' for icon-only, 'full' for text+icon buttons */
  displayMode?: 'compact' | 'full';
};

/**
 * DAGActions component provides buttons to start, stop, and retry DAG executions
 */
function DAGActions({
  status,
  fileId,
  dag,
  refresh,
  displayMode = 'compact',
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [isStartModal, setIsStartModal] = React.useState(false);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);

  const client = useClient();

  /**
   * Reload DAG data after an action is performed
   */
  const reloadData = () => {
    refresh && refresh();
  };

  // Determine which buttons should be enabled based on current status
  const buttonState = {
    start: status?.status != 1,
    stop: status?.status == 1,
    retry: status?.status != 1 && status?.requestId != '',
  };

  if (!dag) {
    return <></>;
  }

  return (
    <TooltipProvider delayDuration={300}>
      <div
        className={`flex items-center ${displayMode === 'compact' ? 'space-x-1' : 'space-x-2'}`}
      >
        {/* Start Button */}
        <Tooltip>
          <TooltipTrigger asChild>
            {displayMode === 'compact' ? (
              <Button
                variant="ghost"
                size="icon"
                disabled={!buttonState['start']}
                onClick={() => setIsStartModal(true)}
                className="h-8 w-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
              >
                <Play className="h-4 w-4" />
                <span className="sr-only">Start</span>
              </Button>
            ) : (
              <Button
                variant="outline"
                size="sm"
                disabled={!buttonState['start']}
                onClick={() => setIsStartModal(true)}
                className="h-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
              >
                <Play className="mr-2 h-4 w-4" />
                Start
              </Button>
            )}
          </TooltipTrigger>
          <TooltipContent>
            <p>Start DAG execution</p>
          </TooltipContent>
        </Tooltip>

        {/* Stop Button */}
        <Tooltip>
          <TooltipTrigger asChild>
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
          </TooltipTrigger>
          <TooltipContent>
            <p>Stop DAG execution</p>
          </TooltipContent>
        </Tooltip>

        {/* Retry Button */}
        <Tooltip>
          <TooltipTrigger asChild>
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
          </TooltipTrigger>
          <TooltipContent>
            <p>Retry DAG execution</p>
          </TooltipContent>
        </Tooltip>
        <ConfirmModal
          title="Confirmation"
          buttonText="Stop"
          visible={isStopModal}
          dismissModal={() => setIsStopModal(false)}
          onSubmit={async () => {
            setIsStopModal(false);
            const { error } = await client.POST('/dags/{fileId}/stop', {
              params: {
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
                path: {
                  fileId: fileId,
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
          <div>Do you really want to cancel the DAG?</div>
        </ConfirmModal>
        <ConfirmModal
          title="Confirmation"
          buttonText="Rerun"
          visible={isRetryModal}
          dismissModal={() => setIsRetryModal(false)}
          onSubmit={async () => {
            setIsRetryModal(false);
            const { error } = await client.POST('/dags/{fileId}/retry', {
              params: {
                path: {
                  fileId: fileId,
                },
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
              },
              body: {
                requestId: status?.requestId || '',
              },
            });
            if (error) {
              alert(error.message || 'An error occurred');
              return;
            }
            reloadData();
          }}
        >
          {/* Keep modal content structure */}
          <div>
            <p className="mb-2">
              Do you really want to rerun the following execution?
            </p>
            <LabeledItem label="Request-ID">
              <span className="font-mono text-sm">
                {status?.requestId || 'N/A'}
              </span>
            </LabeledItem>
          </div>
        </ConfirmModal>
        <StartDAGModal
          dag={dag}
          visible={isStartModal}
          onSubmit={async (params) => {
            setIsStartModal(false);
            const { error } = await client.POST('/dags/{fileId}/start', {
              params: {
                path: {
                  fileId: fileId,
                },
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
              },
              body: {
                params: params,
              },
            });
            if (error) {
              alert(error.message || 'An error occurred');
              return;
            }
            reloadData();
          }}
          dismissModal={() => {
            setIsStartModal(false);
          }}
        />
      </div>
    </TooltipProvider>
  );
}

export default DAGActions;
