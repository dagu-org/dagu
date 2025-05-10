/**
 * DAGActions component provides action buttons for DAG operations (start, stop, retry).
 *
 * @module features/dags/components/common
 */
import { Button } from '@/components/ui/button'; // Import Shadcn Button
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'; // Import Shadcn Tooltip
import dayjs from '@/lib/dayjs';
import StatusChip from '@/ui/StatusChip';
import { Play, RefreshCw, Square } from 'lucide-react'; // Import lucide icons
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient } from '../../../../hooks/api';
import ConfirmModal from '../../../../ui/ConfirmModal';
import LabeledItem from '../../../../ui/LabeledItem';
import { StartDAGModal } from '../dag-execution';

/**
 * Props for the DAGActions component
 */
type Props = {
  /** Current status of the DAG */
  status?:
    | components['schemas']['WorkflowSummary']
    | components['schemas']['WorkflowDetails'];
  /** File ID of the DAG */
  fileName: string;
  /** DAG definition */
  dag?: components['schemas']['DAG'] | components['schemas']['DAGDetails'];
  /** Whether to show text labels on buttons */
  label?: boolean;
  /** Function to refresh data after actions */
  refresh?: () => void;
  /** Display mode: 'compact' for icon-only, 'full' for text+icon buttons */
  displayMode?: 'compact' | 'full';
  /** Function to navigate to status tab after execution */
  navigateToStatusTab?: () => void;
};

/**
 * DAGActions component provides buttons to start, stop, and retry DAG executions
 */
function DAGActions({
  status,
  fileName,
  dag,
  refresh,
  displayMode = 'compact',
  navigateToStatusTab,
}: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const [isStartModal, setIsStartModal] = React.useState(false);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);
  const [retryWorkflowId, setRetryWorkflowId] = React.useState<string>('');

  const client = useClient();

  /**
   * Reload DAG data after an action is performed
   */
  const reloadData = () => {
    if (refresh) {
      refresh();
    }
  };

  // Determine which buttons should be enabled based on current status
  const buttonState = {
    start: status?.status != 1,
    stop: status?.status == 1,
    retry: status?.status != 1 && status?.workflowId != '',
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
                onClick={async () => {
                  // Get the current URL parameters
                  const urlParams = new URLSearchParams(window.location.search);
                  const idxParam = urlParams.get('idx');

                  // Default to current status workflowId
                  let workflowIdToUse = status?.workflowId || '';

                  // If we're in the history page or modal history tab with a specific run selected
                  const isInHistoryPage =
                    window.location.pathname.includes('/history');
                  const isInModalHistoryTab =
                    document.querySelector(
                      '.dag-modal-content [data-tab="history"]'
                    ) !== null;

                  console.log(
                    'Retry check - isInHistoryPage:',
                    isInHistoryPage
                  );
                  console.log(
                    'Retry check - isInModalHistoryTab:',
                    isInModalHistoryTab
                  );
                  console.log('Retry check - idxParam:', idxParam);
                  console.log(
                    'Retry check - current status workflowId:',
                    status?.workflowId
                  );

                  if (
                    (isInHistoryPage || isInModalHistoryTab) &&
                    idxParam !== null
                  ) {
                    try {
                      // Get all workflows for this DAG to find the correct workflowId
                      const { data } = await client.GET(
                        '/dags/{fileName}/workflows',
                        {
                          params: {
                            path: {
                              fileName: fileName,
                            },
                            query: {
                              remoteNode:
                                appBarContext.selectedRemoteNode || 'local',
                            },
                          },
                        }
                      );

                      if (data?.workflows && data.workflows.length > 0) {
                        // Convert idx to integer
                        const selectedIdx = parseInt(idxParam);

                        // Get the workflow at the selected index (reversed order)
                        const selectedWorkflow = [...data.workflows].reverse()[
                          selectedIdx
                        ];

                        console.log('Selected workflow:', selectedWorkflow);
                        console.log(
                          'Selected workflow workflowId:',
                          selectedWorkflow?.workflowId
                        );

                        if (selectedWorkflow && selectedWorkflow.workflowId) {
                          workflowIdToUse = selectedWorkflow.workflowId;
                          console.log(
                            'Using workflowId from selected workflow:',
                            workflowIdToUse
                          );
                        }
                      }
                    } catch (err) {
                      console.error('Error fetching workflows for retry:', err);
                    }
                  }

                  // Set the workflowId to use for retry
                  setRetryWorkflowId(workflowIdToUse);

                  // Show the modal
                  setIsRetryModal(true);
                }}
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
                onClick={async () => {
                  // Get the current URL parameters
                  const urlParams = new URLSearchParams(window.location.search);
                  const idxParam = urlParams.get('idx');

                  // Default to current status workflowId
                  let workflowIdToUse = status?.workflowId || '';

                  // If we're in the history page or modal history tab with a specific run selected
                  const isInHistoryPage =
                    window.location.pathname.includes('/history');
                  const isInModalHistoryTab =
                    document.querySelector(
                      '.dag-modal-content [data-tab="history"]'
                    ) !== null;

                  console.log(
                    'Retry check (full) - isInHistoryPage:',
                    isInHistoryPage
                  );
                  console.log(
                    'Retry check (full) - isInModalHistoryTab:',
                    isInModalHistoryTab
                  );
                  console.log('Retry check (full) - idxParam:', idxParam);
                  console.log(
                    'Retry check (full) - current status workflowId:',
                    status?.workflowId
                  );

                  if (
                    (isInHistoryPage || isInModalHistoryTab) &&
                    idxParam !== null
                  ) {
                    try {
                      // Get all workflows for this DAG to find the correct workflowId
                      const { data } = await client.GET(
                        '/dags/{fileName}/workflows',
                        {
                          params: {
                            path: {
                              fileName: fileName,
                            },
                            query: {
                              remoteNode:
                                appBarContext.selectedRemoteNode || 'local',
                            },
                          },
                        }
                      );

                      if (data?.workflows && data.workflows.length > 0) {
                        // Convert idx to integer
                        const selectedIdx = parseInt(idxParam);

                        // Get the workflow at the selected index (reversed order)
                        const selectedWorkflow = [...data.workflows].reverse()[
                          selectedIdx
                        ];

                        console.log(
                          'Selected workflow (full):',
                          selectedWorkflow
                        );
                        console.log(
                          'Selected workflow workflowId (full):',
                          selectedWorkflow?.workflowId
                        );

                        if (selectedWorkflow && selectedWorkflow.workflowId) {
                          workflowIdToUse = selectedWorkflow.workflowId;
                          console.log(
                            'Using workflowId from selected workflow (full):',
                            workflowIdToUse
                          );
                        }
                      }
                    } catch (err) {
                      console.error('Error fetching workflows for retry:', err);
                    }
                  }

                  // Set the workflowId to use for retry
                  setRetryWorkflowId(workflowIdToUse);

                  // Show the modal
                  setIsRetryModal(true);
                }}
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
            const { error } = await client.POST('/dags/{fileName}/stop', {
              params: {
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
                path: {
                  fileName: fileName,
                },
              },
            });
            if (error) {
              console.error('Retry API error:', error);
              alert(error.message || 'An error occurred');
              return;
            }
            console.log('Retry successful');
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

            console.log('Submitting retry with workflowId:', retryWorkflowId);

            const { error } = await client.POST('/dags/{fileName}/retry', {
              params: {
                path: {
                  fileName: fileName,
                },
                query: {
                  remoteNode: appBarContext.selectedRemoteNode || 'local',
                },
              },
              body: {
                workflowId: retryWorkflowId,
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
            <LabeledItem label="Workflow-Name">
              <span className="font-mono text-sm">{status?.name || 'N/A'}</span>
            </LabeledItem>
            <LabeledItem label="Workflow-ID">
              <span className="font-mono text-sm">
                {retryWorkflowId || status?.workflowId || 'N/A'}
              </span>
            </LabeledItem>
            {status?.startedAt && (
              <LabeledItem label="Started At">
                <span className="text-sm">
                  {dayjs(status.startedAt).format('YYYY-MM-DD HH:mm:ss Z')}
                </span>
              </LabeledItem>
            )}
            {status?.status !== undefined && (
              <LabeledItem label="Status">
                <StatusChip status={status.status} size="sm">
                  {status.statusLabel || ''}
                </StatusChip>
              </LabeledItem>
            )}
          </div>
        </ConfirmModal>
        <StartDAGModal
          dag={dag}
          visible={isStartModal}
          onSubmit={async (params) => {
            setIsStartModal(false);
            const { error } = await client.POST('/dags/{fileName}/start', {
              params: {
                path: {
                  fileName: fileName,
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
            // Navigate to status tab after execution
            if (navigateToStatusTab) {
              navigateToStatusTab();
            }
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
