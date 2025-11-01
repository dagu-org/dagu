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
import { Checkbox } from '@/components/ui/checkbox';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import dayjs from '@/lib/dayjs';
import StatusChip from '@/ui/StatusChip';
import { Play, RefreshCw, Square } from 'lucide-react'; // Import lucide icons
import React from 'react';
import { components } from '../../../../api/v2/schema';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useConfig } from '../../../../contexts/ConfigContext';
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
    | components['schemas']['DAGRunSummary']
    | components['schemas']['DAGRunDetails'];
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
  const config = useConfig();
  const [isEnqueueModal, setIsEnqueueModal] = React.useState(false);
  const [isStopModal, setIsStopModal] = React.useState(false);
  const [isRetryModal, setIsRetryModal] = React.useState(false);
  const [retryDagRunId, setRetryDagRunId] = React.useState<string>('');
  const [stopAllRunning, setStopAllRunning] = React.useState(false);

  // Retry-as-new modal state
  const [retryAsNew, setRetryAsNew] = React.useState(false);
  const [newRunId, setNewRunId] = React.useState('');
  const [dagNameOverride, setDagNameOverride] = React.useState('');

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
    enqueue: true, // Always allow enqueuing
    stop: status?.status == 1,
    retry: status?.status != 1 && status?.status != 5 && status?.dagRunId != '', // Disable when running (1) or queued (5)
  };

  if (!dag || !config.permissions.runDags) {
    return <></>;
  }

  return (
    <TooltipProvider delayDuration={300}>
      <div
        className={`flex items-center ${displayMode === 'compact' ? 'space-x-1' : 'space-x-2'}`}
      >
        {/* Enqueue Button */}
        <Tooltip>
          <TooltipTrigger asChild>
            {displayMode === 'compact' ? (
              <Button
                variant="ghost"
                size="icon"
                disabled={!buttonState['enqueue']}
                onClick={() => setIsEnqueueModal(true)}
                className="h-8 w-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
              >
                <Play className="h-4 w-4" />
                <span className="sr-only">Enqueue</span>
              </Button>
            ) : (
              <Button
                variant="outline"
                size="sm"
                disabled={!buttonState['enqueue']}
                onClick={() => setIsEnqueueModal(true)}
                className="h-8 disabled:text-gray-400 dark:disabled:text-gray-600 cursor-pointer"
              >
                <Play className="mr-2 h-4 w-4" />
                Enqueue
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

                  // Default to current status dagRunId
                  let dagRunIdToUse = status?.dagRunId || '';

                  // If we're in the history page or modal history tab with a specific run selected
                  const isInHistoryPage =
                    window.location.pathname.includes('/history');
                  const isInModalHistoryTab =
                    document.querySelector(
                      '.dag-modal-content [data-tab="history"]'
                    ) !== null;

                  if (
                    (isInHistoryPage || isInModalHistoryTab) &&
                    idxParam !== null
                  ) {
                    try {
                      // Get all dag-runs for this DAG to find the correct dagRunId
                      const { data } = await client.GET(
                        '/dags/{fileName}/dag-runs',
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

                      if (data?.dagRuns && data.dagRuns.length > 0) {
                        // Convert idx to integer
                        const selectedIdx = parseInt(idxParam);

                        // Get the dag-run at the selected index (reversed order)
                        const selectedDagRun = [...data.dagRuns].reverse()[
                          selectedIdx
                        ];

                        if (selectedDagRun && selectedDagRun.dagRunId) {
                          dagRunIdToUse = selectedDagRun.dagRunId;
                        }
                      }
                    } catch (err) {
                      console.error('Error fetching dag-runs for retry:', err);
                    }
                  }

                  // Set the dagRunId to use for retry
                  setRetryDagRunId(dagRunIdToUse);

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

                  // Default to current status dagRunId
                  let dagRunIdToUse = status?.dagRunId || '';

                  // If we're in the history page or modal history tab with a specific run selected
                  const isInHistoryPage =
                    window.location.pathname.includes('/history');
                  const isInModalHistoryTab =
                    document.querySelector(
                      '.dag-modal-content [data-tab="history"]'
                    ) !== null;

                  if (
                    (isInHistoryPage || isInModalHistoryTab) &&
                    idxParam !== null
                  ) {
                    try {
                      // Get all dag-runs for this DAG to find the correct dagRunId
                      const { data } = await client.GET(
                        '/dags/{fileName}/dag-runs',
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

                      if (data?.dagRuns && data.dagRuns.length > 0) {
                        // Convert idx to integer
                        const selectedIdx = parseInt(idxParam);

                        // Get the dag-run at the selected index (reversed order)
                        const selectedDagRun = [...data.dagRuns].reverse()[
                          selectedIdx
                        ];

                        if (selectedDagRun && selectedDagRun.dagRunId) {
                          dagRunIdToUse = selectedDagRun.dagRunId;
                        }
                      }
                    } catch (err) {
                      console.error('Error fetching dag-runs for retry:', err);
                    }
                  }

                  // Set the dagRunId to use for retry
                  setRetryDagRunId(dagRunIdToUse);

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
          dismissModal={() => {
            setIsStopModal(false);
            setStopAllRunning(false);
          }}
          onSubmit={async () => {
            setIsStopModal(false);

            // If stopAllRunning is checked, use the stop-all endpoint
            if (stopAllRunning) {
              const { error } = await client.POST('/dags/{fileName}/stop-all', {
                params: {
                  path: { fileName },
                  query: {
                    remoteNode: appBarContext.selectedRemoteNode || 'local',
                  },
                },
              });
              if (error) {
                console.error('Stop all API error:', error);
                alert(
                  error.message ||
                    'An error occurred while stopping all instances'
                );
                return;
              }
              setStopAllRunning(false);
              reloadData();
            } else {
              // Use dag-run API - requires DAG name and ID
              if (status?.name && status?.dagRunId) {
                const { error } = await client.POST(
                  '/dag-runs/{name}/{dagRunId}/stop',
                  {
                    params: {
                      query: {
                        remoteNode: appBarContext.selectedRemoteNode || 'local',
                      },
                      path: {
                        name: status.name,
                        dagRunId: status.dagRunId,
                      },
                    },
                  }
                );
                if (error) {
                  console.error('Stop dag-run API error:', error);
                  alert(error.message || 'An error occurred');
                  return;
                }
                reloadData();
              } else {
                console.error('Cannot stop DAG: missing DAG name or run ID');
                alert('Cannot stop DAG: missing DAG name or run ID');
              }
            }
          }}
        >
          <div>
            <p className="mb-2">
              {stopAllRunning
                ? `Do you really want to stop all running instances of this DAG?`
                : status?.name && status?.dagRunId
                  ? `Do you really want to stop the dag-run "${status.name}"?`
                  : 'Do you really want to cancel the DAG?'}
            </p>
            {!stopAllRunning && status?.name && (
              <LabeledItem label="DAG-Run-Name">
                <span className="font-mono text-sm">{status.name}</span>
              </LabeledItem>
            )}
            {!stopAllRunning && status?.dagRunId && (
              <LabeledItem label="DAG-Run-ID">
                <span className="font-mono text-sm">{status.dagRunId}</span>
              </LabeledItem>
            )}
            {!stopAllRunning && status?.startedAt && (
              <LabeledItem label="Started At">
                <span className="text-sm">
                  {dayjs(status.startedAt).format('YYYY-MM-DD HH:mm:ss Z')}
                </span>
              </LabeledItem>
            )}
            {!stopAllRunning && status?.status !== undefined && (
              <LabeledItem label="Status">
                <StatusChip status={status.status} size="sm">
                  {status.statusLabel || ''}
                </StatusChip>
              </LabeledItem>
            )}
            <div className="mt-4 flex items-center space-x-2 p-2 bg-amber-50 dark:bg-amber-900/20 rounded border border-amber-200 dark:border-amber-800">
              <Checkbox
                id="stop-all"
                checked={stopAllRunning}
                onCheckedChange={(checked) =>
                  setStopAllRunning(checked as boolean)
                }
                className="border-amber-600 dark:border-amber-400 data-[state=checked]:bg-amber-600 data-[state=checked]:border-amber-600 data-[state=checked]:text-white dark:data-[state=checked]:text-black"
              />
              <label
                htmlFor="stop-all"
                className="text-sm font-medium leading-none peer-disabled:cursor-not-allowed peer-disabled:opacity-70 text-amber-700 dark:text-amber-300"
              >
                Stop all running instances
              </label>
            </div>
          </div>
        </ConfirmModal>
        <ConfirmModal
          title={retryAsNew ? "Retry DAG Run as New" : "Confirmation"}
          buttonText="Rerun"
          visible={isRetryModal}
          dismissModal={() => {
            setIsRetryModal(false);
            setRetryAsNew(false);
            setNewRunId('');
            setDagNameOverride('');
          }}
          onSubmit={async () => {
            setIsRetryModal(false);

            // Use dag-run API - requires DAG name and ID
            if (status?.name && retryDagRunId) {
              if (retryAsNew) {
                // Use reschedule endpoint for retry-as-new
                const { error, data } = await client.POST(
                  '/dag-runs/{name}/{dagRunId}/reschedule',
                  {
                    params: {
                      path: {
                        name: status.name,
                        dagRunId: retryDagRunId,
                      },
                      query: {
                        remoteNode: appBarContext.selectedRemoteNode || 'local',
                      },
                    },
                    body: {
                      dagRunId: newRunId || undefined, // Auto-generate if empty
                      ...(dagNameOverride ? { dagName: dagNameOverride } : {}),
                      singleton: false,
                    },
                  }
                );
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
                const { error } = await client.POST(
                  '/dag-runs/{name}/{dagRunId}/retry',
                  {
                    params: {
                      path: {
                        name: status.name,
                        dagRunId: retryDagRunId,
                      },
                      query: {
                        remoteNode: appBarContext.selectedRemoteNode || 'local',
                      },
                    },
                    body: {
                      dagRunId: retryDagRunId,
                    },
                  }
                );
                if (error) {
                  alert(error.message || 'An error occurred');
                  return;
                }
              }
              reloadData();
            } else {
              console.error('Cannot retry DAG: missing DAG name or run ID');
              alert('Cannot retry DAG: missing DAG name or run ID');
            }
          }}
        >
          {/* Keep modal content structure */}
          <div className="space-y-3">
            <p className="mb-2">
              {status?.name && retryDagRunId
                ? `Do you really want to retry the dag-run "${status.name}"?`
                : 'Do you really want to rerun the following execution?'}
            </p>
            <LabeledItem label="DAG-Run-Name">
              <span className="font-mono text-sm">{status?.name || 'N/A'}</span>
            </LabeledItem>
            <LabeledItem label="DAG-Run-ID">
              <span className="font-mono text-sm">
                {retryDagRunId || status?.dagRunId || 'N/A'}
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

            {/* Retry as new checkbox */}
            <div className="flex items-center space-x-2 pt-2 border-t">
              <Checkbox
                id="retry-as-new-dag"
                checked={retryAsNew}
                onCheckedChange={(checked) => setRetryAsNew(checked as boolean)}
                className="border-gray-400 dark:border-gray-500"
              />
              <Label htmlFor="retry-as-new-dag" className="cursor-pointer text-sm">
                Retry as new DAG-run
              </Label>
            </div>

            {/* Conditional inputs when retry-as-new is checked */}
            {retryAsNew && (
              <div className="space-y-3 pt-2">
                <div className="space-y-1.5">
                  <Label htmlFor="new-dagrun-id-dag" className="text-sm">
                    New DAG-Run ID (optional)
                  </Label>
                  <Input
                    id="new-dagrun-id-dag"
                    placeholder="Auto-generated if empty"
                    value={newRunId}
                    onChange={(e) => setNewRunId(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="dag-name-override-dag" className="text-sm">
                    DAG Name Override (optional)
                  </Label>
                  <Input
                    id="dag-name-override-dag"
                    placeholder={`Leave empty to use: ${status?.name || 'original'}`}
                    value={dagNameOverride}
                    onChange={(e) => setDagNameOverride(e.target.value)}
                    className="h-8 text-sm"
                  />
                </div>
              </div>
            )}
          </div>
        </ConfirmModal>
        <StartDAGModal
          dag={dag}
          visible={isEnqueueModal}
          action="enqueue"
          onSubmit={async (params, dagRunId, immediate) => {
            setIsEnqueueModal(false);
            const body: { params: string; dagRunId?: string } = { params };
            if (dagRunId) {
              body.dagRunId = dagRunId;
            }

            // Use /start endpoint if immediate is true, otherwise use /enqueue
            const { error } = await (immediate
              ? client.POST('/dags/{fileName}/start', {
                  params: {
                    path: {
                      fileName: fileName,
                    },
                    query: {
                      remoteNode: appBarContext.selectedRemoteNode || 'local',
                    },
                  },
                  body,
                })
              : client.POST('/dags/{fileName}/enqueue', {
                  params: {
                    path: {
                      fileName: fileName,
                    },
                    query: {
                      remoteNode: appBarContext.selectedRemoteNode || 'local',
                    },
                  },
                  body,
                }));
            if (error) {
              alert(error.message || 'An error occurred');
              return;
            }

            // Just refresh the current page data
            reloadData();
            // Navigate to status tab after execution (if available)
            if (navigateToStatusTab) {
              navigateToStatusTab();
            }
          }}
          dismissModal={() => {
            setIsEnqueueModal(false);
          }}
        />
      </div>
    </TooltipProvider>
  );
}

export default DAGActions;
