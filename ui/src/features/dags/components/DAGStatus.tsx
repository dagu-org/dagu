import { useErrorModal } from '@/components/ui/error-modal';
import { Tab, Tabs } from '@/components/ui/tabs';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  ActivitySquare,
  FileCode,
  GanttChart,
  MessageSquare,
  MousePointerClick,
  Package,
} from 'lucide-react';
import React, { useState, useEffect } from 'react';
import { useCookies } from 'react-cookie';
import { useNavigate } from 'react-router-dom';
import { components, NodeStatus, Status, Stream } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useConfig } from '../../../contexts/ConfigContext';
import { useClient } from '../../../hooks/api';
import { toMermaidNodeId } from '../../../lib/utils';
import BorderedBox from '../../../ui/BorderedBox';
import { DAGRunOutputs } from '../../dag-runs/components/dag-run-details';
import { DAGContext } from '../contexts/DAGContext';
import { getEventHandlers } from '../lib/getEventHandlers';
import { DAGStatusOverview, NodeStatusTable } from './dag-details';
import {
  LogViewer,
  ParallelExecutionModal,
  StatusUpdateModal,
} from './dag-execution';
import { DAGSpecReadOnly } from './dag-editor';
import { FlowchartType, Graph, TimelineChart } from './visualization';
import { ChatHistoryTab } from './chat-history';

type Props = {
  dagRun: components['schemas']['DAGRunDetails'];
  fileName: string;
};

type StatusTab = 'status' | 'timeline' | 'outputs' | 'chat' | 'spec';

function DAGStatus({ dagRun, fileName }: Props) {
  const appBarContext = React.useContext(AppBarContext);
  const config = useConfig();
  const navigate = useNavigate();
  const { showError } = useErrorModal();
  const [modal, setModal] = useState(false);
  const [activeTab, setActiveTab] = useState<StatusTab>('status');

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = useState<FlowchartType>(cookie['flowchart']);


  const [selectedStep, setSelectedStep] = useState<
    components['schemas']['Step'] | undefined
  >(undefined);
  // State for log viewer
  const [logViewer, setLogViewer] = useState<{
    isOpen: boolean;
    logType: 'execution' | 'step';
    stepName: string;
    dagRunId: string;
    stream: Stream;
    node?: components['schemas']['Node'];
  }>({
    isOpen: false,
    logType: 'step',
    stepName: '',
    dagRunId: '',
    stream: Stream.stdout,
  });
  // State for parallel execution modal
  const [parallelExecutionModal, setParallelExecutionModal] = useState<{
    isOpen: boolean;
    node?: components['schemas']['Node'];
  }>({
    isOpen: false,
  });
  const client = useClient();
  const dismissModal = () => setModal(false);

  /**
   * Handle flowchart direction change and save preference to cookie
   */
  const onChangeFlowchart = (value: FlowchartType) => {
    if (!value) {
      return;
    }
    setCookie('flowchart', value, { path: '/' });
    setFlowchart(value);
  };

  const onUpdateStatus = async (
    step: components['schemas']['Step'],
    status: NodeStatus
  ) => {
    // Check if this is a sub DAG-run by checking if rootDAGRunId and rootDAGRunName exist
    // and are different from the current DAG-run's ID and name
    const isSubDAGRun =
      dagRun.rootDAGRunId &&
      dagRun.rootDAGRunName &&
      dagRun.rootDAGRunId !== dagRun.dagRunId;

    // Define path parameters with proper typing
    const pathParams = {
      name: isSubDAGRun ? dagRun.rootDAGRunName : dagRun.name,
      dagRunId: isSubDAGRun ? dagRun.rootDAGRunId : dagRun.dagRunId,
      stepName: step.name,
      ...(isSubDAGRun ? { subDAGRunId: dagRun.dagRunId } : {}),
    };

    // Use the appropriate endpoint based on whether this is a sub DAG-run
    const endpoint = isSubDAGRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/status'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/status';

    const { error } = await client.PATCH(endpoint, {
      params: {
        path: pathParams,
        query: {
          remoteNode: appBarContext.selectedRemoteNode || 'local',
        },
      },
      body: {
        status,
      },
    });
    if (error) {
      showError(
        error.message || 'Failed to update status',
        'Please try again or check the server connection.'
      );
      return;
    }
    dismissModal();
  };
  // Handle double-click on graph node (navigate to sub dagRun)
  const onSelectStepOnGraph = React.useCallback(
    async (id: string) => {
      // find the clicked step
      const n = dagRun.nodes?.find((n) => toMermaidNodeId(n.step.name) == id);

      const subDAGName = n?.step?.call;
      if (n && subDAGName) {
        // Combine both regular children and repeated children
        const allSubRuns = [...(n.subRuns || []), ...(n.subRunsRepeated || [])];

        // Check if there are multiple sub runs (parallel execution or repeated)
        if (allSubRuns.length > 1) {
          // Show modal to select which execution to view
          setParallelExecutionModal({
            isOpen: true,
            node: n,
          });
        } else if (allSubRuns.length === 1) {
          // Single sub dagRun - navigate directly
          navigateToSubDagRun(n, 0);
        }
      }
    },
    [dagRun, navigate, fileName]
  );

  // Helper function to navigate to a specific sub DAG run
  const navigateToSubDagRun = React.useCallback(
    (
      node: components['schemas']['Node'],
      childIndex: number,
      openInNewTab?: boolean
    ) => {
      // Combine both regular children and repeated children
      const allSubRuns = [
        ...(node.subRuns || []),
        ...(node.subRunsRepeated || []),
      ];
      const subDAGRun = allSubRuns[childIndex];

      if (subDAGRun && subDAGRun.dagRunId) {
        // Navigate to the sub DAG-run status page
        const dagRunId = dagRun.rootDAGRunId || dagRun.dagRunId;

        // Check if we're in a dagRun context or a DAG context
        const currentPath = window.location.pathname;
        const isModal =
          document.querySelector('.dagRun-modal-content') !== null;
        const isDAGRunContext = currentPath.startsWith('/dag-runs/') || isModal;

        let url: string;
        if (isDAGRunContext) {
          // For DAG runs, use query parameters to navigate to the DAG-run details page
          const searchParams = new URLSearchParams();
          searchParams.set('subDAGRunId', subDAGRun.dagRunId);

          // Use root DAG-run information
          if (dagRun.rootDAGRunId) {
            searchParams.set('dagRunId', dagRun.rootDAGRunId);
            searchParams.set('dagRunName', dagRun.rootDAGRunName);
          } else {
            searchParams.set('dagRunId', dagRun.dagRunId);
            searchParams.set('dagRunName', dagRun.name);
          }

          searchParams.set('step', node.step.name);

          // Determine root DAG name
          const rootDAGName = dagRun.rootDAGRunName || dagRun.name;
          url = `/dag-runs/${rootDAGName}/${dagRunId}?${searchParams.toString()}`;
        } else {
          // For DAGs, use the existing approach with query parameters
          url = `/dags/${fileName}?subDAGRunId=${subDAGRun.dagRunId}&dagRunId=${dagRunId}&step=${node.step.name}&dagRunName=${encodeURIComponent(dagRun.rootDAGRunName || dagRun.name)}`;
        }

        if (openInNewTab) {
          window.open(url, '_blank');
        } else {
          navigate(url);
        }
      }
    },
    [dagRun, navigate, fileName]
  );

  // Handle right-click on graph node (show status update modal)
  const onRightClickStepOnGraph = React.useCallback(
    (id: string) => {
      // Check if user has permission to run DAGs
      if (!config.permissions.runDags) {
        return;
      }

      const status = dagRun.status;

      // Only allow status updates for completed DAG runs
      if (status !== Status.Running && status !== Status.NotStarted) {
        // find the right-clicked step
        const n = dagRun.nodes?.find((n) => toMermaidNodeId(n.step.name) == id);

        if (n) {
          // Show the modal (it will be centered by default)
          setSelectedStep(n.step);
          setModal(true);
        }
      }
    },
    [dagRun, config.permissions.runDags]
  );

  const handlers = getEventHandlers(dagRun);

  // Handler for opening log viewer
  const handleViewLog = (
    stepName: string,
    dagRunId: string,
    node?: components['schemas']['Node']
  ) => {
    // Check if this is a stderr log (indicated by _stderr suffix)
    const isStderr = stepName.endsWith('_stderr');
    const actualStepName = isStderr ? stepName.slice(0, -7) : stepName; // Remove '_stderr' suffix

    setLogViewer({
      isOpen: true,
      logType: 'step',
      stepName: actualStepName,
      dagRunId: dagRunId || dagRun.dagRunId,
      stream: isStderr ? Stream.stderr : Stream.stdout,
      node,
    });
  };

  // Check if timeline should be shown (any status except not started)
  const showTimeline = dagRun.status !== Status.NotStarted;

  // Check if there are any chat steps
  const hasChatSteps = dagRun.nodes?.some(
    (node) => node.step.executorConfig?.type === 'chat'
  );

  // Reset to status tab if selected tab is not available
  useEffect(() => {
    if (activeTab === 'timeline' && !showTimeline) {
      setActiveTab('status');
    }
    if (activeTab === 'chat' && !hasChatSteps) {
      setActiveTab('status');
    }
  }, [showTimeline, hasChatSteps, activeTab]);

  return (
    <div className="space-y-4">
      {/* Status Detail Tabs */}
      <Tabs className="whitespace-nowrap">
        <Tab
          isActive={activeTab === 'status'}
          onClick={() => setActiveTab('status')}
          className="flex items-center gap-2 cursor-pointer"
        >
          <ActivitySquare className="h-4 w-4" />
          Status
        </Tab>
        {showTimeline && (
          <Tab
            isActive={activeTab === 'timeline'}
            onClick={() => setActiveTab('timeline')}
            className="flex items-center gap-2 cursor-pointer"
          >
            <GanttChart className="h-4 w-4" />
            Timeline
          </Tab>
        )}
        <Tab
          isActive={activeTab === 'outputs'}
          onClick={() => setActiveTab('outputs')}
          className="flex items-center gap-2 cursor-pointer"
        >
          <Package className="h-4 w-4" />
          Outputs
        </Tab>
        {hasChatSteps && (
          <Tab
            isActive={activeTab === 'chat'}
            onClick={() => setActiveTab('chat')}
            className="flex items-center gap-2 cursor-pointer"
          >
            <MessageSquare className="h-4 w-4" />
            Chat
          </Tab>
        )}
        <Tab
          isActive={activeTab === 'spec'}
          onClick={() => setActiveTab('spec')}
          className="flex items-center gap-2 cursor-pointer"
        >
          <FileCode className="h-4 w-4" />
          Spec
        </Tab>
      </Tabs>

      {/* Status Tab Content */}
      {activeTab === 'status' && (
        <div className="space-y-6">
          {/* DAG Graph Visualization */}
          {dagRun.nodes && dagRun.nodes.length > 0 && (
            <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
              <div className="flex justify-end mb-2">
                <Tooltip>
                  <TooltipTrigger asChild>
                    <div className="flex items-center text-xs text-muted-foreground bg-muted px-2 py-1 rounded cursor-help">
                      <MousePointerClick className="h-3 w-3 mr-1" />
                      {config.permissions.runDags
                        ? 'Double-click to navigate / Right-click to change status'
                        : 'Double-click to navigate'}
                    </div>
                  </TooltipTrigger>
                  <TooltipContent>
                    <div className="space-y-1">
                      <p>Double-click: Navigate to sub dagRun</p>
                      {config.permissions.runDags && (
                        <p>Right-click: Update node status</p>
                      )}
                    </div>
                  </TooltipContent>
                </Tooltip>
              </div>
              <div className="overflow-x-auto">
                <Graph
                  steps={dagRun.nodes}
                  type="status"
                  flowchart={flowchart}
                  onChangeFlowchart={onChangeFlowchart}
                  onClickNode={onSelectStepOnGraph}
                  onRightClickNode={
                    config.permissions.runDags ? onRightClickStepOnGraph : undefined
                  }
                  showIcons={dagRun.status > Status.NotStarted}
                  animate={dagRun.status == Status.Running}
                />
              </div>
            </BorderedBox>
          )}

          <DAGContext.Consumer>
            {(props) => (
              <>
                <div className="grid grid-cols-1 gap-6">
                  {/* Status Overview */}
                  <div className="bg-surface border border-border rounded-lg p-4">
                    <DAGStatusOverview
                      status={dagRun}
                      onViewLog={(dagRunId) => {
                        setLogViewer({
                          isOpen: true,
                          logType: 'execution',
                          stepName: '',
                          dagRunId,
                          stream: Stream.stdout,
                        });
                      }}
                    />
                  </div>

                  {/* Steps Table */}
                  <NodeStatusTable
                    nodes={dagRun.nodes}
                    status={dagRun}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                </div>

                {/* Lifecycle Hooks */}
                {handlers?.length ? (
                  <NodeStatusTable
                    nodes={handlers}
                    status={dagRun}
                    {...props}
                    onViewLog={handleViewLog}
                  />
                ) : null}
              </>
            )}
          </DAGContext.Consumer>
        </div>
      )}

      {/* Timeline Tab Content */}
      {activeTab === 'timeline' && showTimeline && (
        <TimelineChart status={dagRun} />
      )}

      {/* Outputs Tab Content */}
      {activeTab === 'outputs' && (
        <DAGRunOutputs dagName={dagRun.name} dagRunId={dagRun.dagRunId} />
      )}

      {/* Chat Tab Content */}
      {activeTab === 'chat' && <ChatHistoryTab dagRun={dagRun} />}

      {/* Spec Tab Content */}
      {activeTab === 'spec' && (
        <DAGSpecReadOnly dagName={dagRun.name} dagRunId={dagRun.dagRunId} />
      )}

      <StatusUpdateModal
        visible={modal}
        step={selectedStep}
        dismissModal={dismissModal}
        onSubmit={onUpdateStatus}
      />

      {/* Log viewer modal */}
      <LogViewer
        isOpen={logViewer.isOpen}
        onClose={() => setLogViewer((prev) => ({ ...prev, isOpen: false }))}
        logType={logViewer.logType}
        dagName={dagRun.name}
        dagRunId={logViewer.dagRunId}
        stepName={logViewer.stepName}
        dagRun={dagRun}
        stream={logViewer.stream}
        node={logViewer.node}
      />

      {/* Parallel execution selection modal */}
      {parallelExecutionModal.isOpen && parallelExecutionModal.node && (
        <ParallelExecutionModal
          isOpen={parallelExecutionModal.isOpen}
          onClose={() => setParallelExecutionModal({ isOpen: false })}
          stepName={parallelExecutionModal.node.step.name}
          subDAGName={parallelExecutionModal.node.step.call || ''}
          subRuns={[
            ...(parallelExecutionModal.node.subRuns || []),
            ...(parallelExecutionModal.node.subRunsRepeated || []),
          ]}
          rootDagName={dagRun.rootDAGRunName}
          rootDagRunId={dagRun.rootDAGRunId}
          parentDagRunId={dagRun.dagRunId}
          onSelectSubRun={(subRunIndex, openInNewTab) => {
            navigateToSubDagRun(
              parallelExecutionModal.node!,
              subRunIndex,
              openInNewTab
            );
            if (!openInNewTab) {
              setParallelExecutionModal({ isOpen: false });
            }
          }}
        />
      )}
    </div>
  );
}

export default DAGStatus;
