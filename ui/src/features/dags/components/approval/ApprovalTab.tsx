import { Button } from '@/components/ui/button';
import { AppBarContext } from '@/contexts/AppBarContext';
import { useClient } from '@/hooks/api';
import { Ban, Check, RotateCcw } from 'lucide-react';
import React, { useState } from 'react';
import { components, NodeStatus, Stream } from '../../../../api/v1/schema';
import { InlineLogViewer } from '../common/InlineLogViewer';
import { StepReviewModal } from '../dag-execution/StepReviewModal';

type DAGRunDetails = components['schemas']['DAGRunDetails'];
type NodeData = components['schemas']['Node'];

type WaitConfig = {
  prompt?: string;
  input?: string[];
  required?: string[];
};

interface ApprovalTabProps {
  dagRun: DAGRunDetails;
  dagName: string;
}

function ApprovalCard({
  node,
  dagRun,
  dagName,
  onAction,
}: {
  node: NodeData;
  dagRun: DAGRunDetails;
  dagName: string;
  onAction: (node: NodeData, action: 'approve' | 'retry' | 'reject') => void;
}) {
  const step = node.step;
  const approvalConfig = step.approval;
  const waitConfig: WaitConfig = approvalConfig
    ? { prompt: approvalConfig.prompt, input: approvalConfig.input, required: approvalConfig.required }
    : (step.executorConfig?.config as WaitConfig) || {};
  const prompt = waitConfig.prompt || step.description;
  const iteration = node?.approvalIteration || 0;

  return (
    <div className="bg-surface border border-border rounded-lg p-4 space-y-3">
      {/* Header + Action buttons */}
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <div className="flex items-center gap-2">
            <span className="text-sm font-semibold">{step.name}</span>
            {iteration > 0 && (
              <span className="text-xs font-normal text-muted-foreground bg-muted px-1.5 py-0.5 rounded">
                Iteration {iteration}
              </span>
            )}
          </div>
          {prompt && (
            <div className="text-base whitespace-pre-wrap">
              {prompt}
            </div>
          )}
        </div>
        <div className="flex shrink-0 gap-2">
          {step.approval && (
            <Button
              size="sm"
              variant="secondary"
              onClick={() => onAction(node, 'retry')}
            >
              <RotateCcw className="h-4 w-4" />
              Retry
            </Button>
          )}
          <Button
            size="icon"
            variant="ghost"
            className="h-8 w-8 text-muted-foreground hover:text-destructive"
            onClick={() => onAction(node, 'reject')}
          >
            <Ban className="h-4 w-4" />
          </Button>
          <Button
            size="sm"
            variant="primary"
            onClick={() => onAction(node, 'approve')}
          >
            <Check className="h-4 w-4" />
            Approve
          </Button>
        </div>
      </div>

      {/* Step Output */}
      <div>
        <div className="text-xs font-medium text-muted-foreground mb-1">Step Output</div>
        <div className="max-h-[400px] overflow-y-auto rounded border border-border">
          <InlineLogViewer
            dagName={dagName}
            dagRunId={dagRun.dagRunId}
            stepName={step.name}
            stream={Stream.stdout}
            dagRun={dagRun}
          />
        </div>
      </div>
    </div>
  );
}

export function ApprovalTab({ dagRun, dagName }: ApprovalTabProps) {
  const client = useClient();
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  const [reviewState, setReviewState] = useState<{
    node: NodeData;
    action: 'approve' | 'retry' | 'reject';
  } | null>(null);

  const waitingNodes = dagRun.nodes?.filter(
    (n) => n.status === NodeStatus.Waiting
  ) || [];

  const isSubRun = !!(
    dagRun.rootDAGRunId &&
    dagRun.rootDAGRunName &&
    dagRun.rootDAGRunId !== dagRun.dagRunId
  );

  const getPathParams = (stepName: string) => ({
    name: isSubRun ? dagRun.rootDAGRunName : dagName,
    dagRunId: isSubRun ? dagRun.rootDAGRunId : dagRun.dagRunId,
    stepName,
    ...(isSubRun ? { subDAGRunId: dagRun.dagRunId } : {}),
  });

  const handleApprove = async (inputs: Record<string, string>) => {
    if (!reviewState) return;
    const endpoint = isSubRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/approve'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/approve';
    const { error } = await client.POST(endpoint, {
      params: { path: getPathParams(reviewState.node.step.name), query: { remoteNode } },
      body: { inputs: Object.keys(inputs).length > 0 ? inputs : undefined },
    });
    if (error) throw new Error(error.message || 'Failed to approve step');
  };

  const handlePushBack = async (inputs: Record<string, string>) => {
    if (!reviewState) return;
    const endpoint = isSubRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/push-back'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/push-back';
    const { error } = await client.POST(endpoint, {
      params: { path: getPathParams(reviewState.node.step.name), query: { remoteNode } },
      body: { inputs: Object.keys(inputs).length > 0 ? inputs : undefined },
    });
    if (error) throw new Error(error.message || 'Failed to retry step');
  };

  const handleReject = async (reason: string) => {
    if (!reviewState) return;
    const endpoint = isSubRun
      ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/steps/{stepName}/reject'
      : '/dag-runs/{name}/{dagRunId}/steps/{stepName}/reject';
    const { error } = await client.POST(endpoint, {
      params: { path: getPathParams(reviewState.node.step.name), query: { remoteNode } },
      body: { reason: reason || undefined },
    });
    if (error) throw new Error(error.message || 'Failed to reject step');
  };

  if (waitingNodes.length === 0) {
    return (
      <div className="text-center text-muted-foreground py-8 text-sm">
        No steps awaiting approval.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {waitingNodes.map((node) => (
        <ApprovalCard
          key={`${node.step.name}-${node.approvalIteration || 0}`}
          node={node}
          dagRun={dagRun}
          dagName={dagName}
          onAction={(n, action) => setReviewState({ node: n, action })}
        />
      ))}

      {reviewState && (
        <StepReviewModal
          visible={!!reviewState}
          dismissModal={() => setReviewState(null)}
          step={reviewState.node.step}
          node={reviewState.node}
          dagName={dagName}
          dagRunId={dagRun.dagRunId}
          dagRun={dagRun}
          onApprove={reviewState.action === 'approve' ? handleApprove : undefined}
          onPushBack={reviewState.action === 'retry' ? handlePushBack : undefined}
          onReject={reviewState.action === 'reject' ? handleReject : undefined}
          compact
        />
      )}
    </div>
  );
}
