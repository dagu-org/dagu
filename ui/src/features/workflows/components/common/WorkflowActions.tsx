/**
 * WorkflowActions component provides action buttons for Workflow operations (stop, retry).
 *
 * @module features/workflows/components/common
 */
import { Button } from '@/components/ui/button';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import dayjs from '@/lib/dayjs';
import { RefreshCw, Square } from 'lucide-react';
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient } from '../../../../hooks/api';
import ConfirmModal from '../../../../ui/ConfirmModal';
import LabeledItem from '../../../../ui/LabeledItem';
import StatusChip from '../../../../ui/StatusChip';

/**
 * Props for the WorkflowActions component
 */
type Props = {
  /** Current status of the Workflow */
  workflow?:
    | components['schemas']['WorkflowSummary']
    | components['schemas']['WorkflowDetails'];
  /** Name of the Workflow */
  name: string;
  /** Whether to show text labels on buttons */
  label?: boolean;
  /** Function to refresh data after actions */
  refresh?: () => void;
  /** Display mode: 'compact' for icon-only, 'full' for text+icon buttons */
  displayMode?: 'compact' | 'full';
  /** Whether this is a root level workflow (controls availability of retry/stop actions) */
  isRootLevel?: boolean;
};

/**
 * WorkflowActions component provides buttons to stop and retry Workflow executions
 */
function WorkflowActions({
  workflow,
  name,
  refresh,
  displayMode = 'compact',
  isRootLevel = true,
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);

  const client = useClient();

  /**
   * Reload Workflow data after an action is performed
   */
  const reloadData = () => {
    if (refresh) {
      refresh();
    }
  };

  // Determine which buttons should be enabled based on current status and root level
  const buttonState = {
    stop: isRootLevel && workflow?.status === 1, // Running and at root level
    retry: isRootLevel && workflow?.status !== 1 && workflow?.workflowId !== '', // Not running, has workflowId, and at root level
  };

  if (!workflow) {
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
            <p>{isRootLevel ? 'Stop Workflow execution' : 'Stop action only available at root workflow level'}</p>
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
            <p>{isRootLevel ? 'Retry Workflow execution' : 'Retry action only available at root workflow level'}</p>
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
            const { error } = await client.POST('/workflows/{name}/{workflowId}/stop', {
              params: {
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
                path: {
                  name: name,
                  workflowId: workflow.workflowId,
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
          <div>Do you really want to stop this workflow?</div>
        </ConfirmModal>

        {/* Retry Confirmation Modal */}
        <ConfirmModal
          title="Confirmation"
          buttonText="Retry"
          visible={isRetryModal}
          dismissModal={() => setIsRetryModal(false)}
          onSubmit={async () => {
            setIsRetryModal(false);

            const { error } = await client.POST('/workflows/{name}/{workflowId}/retry', {
              params: {
                path: {
                  name: name,
                  workflowId: workflow.workflowId,
                },
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
              },
              body: {
                workflowId: workflow.workflowId,
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
            <LabeledItem label="Workflow-Name">
              <span className="font-mono text-sm">
                {workflow?.name || 'N/A'}
              </span>
            </LabeledItem>
            <LabeledItem label="Workflow-ID">
              <span className="font-mono text-sm">
                {workflow?.workflowId || 'N/A'}
              </span>
            </LabeledItem>
            {workflow?.startedAt && (
              <LabeledItem label="Started At">
                <span className="text-sm">
                  {dayjs(workflow.startedAt).format('YYYY-MM-DD HH:mm:ss Z')}
                </span>
              </LabeledItem>
            )}
            {workflow?.status !== undefined && (
              <LabeledItem label="Status">
                <StatusChip status={workflow.status} size="sm">
                  {workflow.statusLabel || ''}
                </StatusChip>
              </LabeledItem>
            )}
          </div>
        </ConfirmModal>
      </div>
    </TooltipProvider>
  );
}

export default WorkflowActions;
