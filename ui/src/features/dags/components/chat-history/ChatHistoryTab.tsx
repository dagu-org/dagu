import React, { useState, useMemo } from 'react';
import { Loader2 } from 'lucide-react';
import { components, NodeStatus } from '@/api/v2/schema';
import { StepMessagesTable } from './StepMessagesTable';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

interface ChatHistoryTabProps {
  dagRun: DAGRunDetails;
}

export function ChatHistoryTab({ dagRun }: ChatHistoryTabProps) {
  // Find all chat steps (steps with type: 'chat' in executorConfig)
  const chatSteps = useMemo(() => {
    return (
      dagRun.nodes?.filter(
        (node) => node.step.executorConfig?.type === 'chat'
      ) || []
    );
  }, [dagRun.nodes]);

  // Determine default selected step: last finished chat step
  const defaultStep = useMemo(() => {
    const finishedStatuses = [
      NodeStatus.Success,
      NodeStatus.Failed,
      NodeStatus.Aborted,
    ];
    const finishedSteps = chatSteps.filter((n) =>
      finishedStatuses.includes(n.status as NodeStatus)
    );

    // Last finished = highest index among finished (assumes nodes are in execution order)
    if (finishedSteps.length > 0) {
      return finishedSteps[finishedSteps.length - 1]?.step.name;
    }

    // Fallback to first chat step if none finished
    return chatSteps[0]?.step.name;
  }, [chatSteps]);

  const [selectedStep, setSelectedStep] = useState<string | undefined>(
    defaultStep
  );

  // Get selected node info
  const selectedNode = chatSteps.find((n) => n.step.name === selectedStep);
  const isSelectedRunning = selectedNode?.status === NodeStatus.Running;

  // Determine if this is a sub-DAG run
  const isSubDAGRun =
    dagRun.rootDAGRunId &&
    dagRun.rootDAGRunName &&
    dagRun.rootDAGRunId !== dagRun.dagRunId;

  if (chatSteps.length === 0) {
    return (
      <div className="text-xs text-muted-foreground p-2">
        No chat steps in this DAG run
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {/* Step selector dropdown */}
      <div className="flex items-center gap-2 text-xs">
        <span className="text-muted-foreground">Step:</span>
        <select
          value={selectedStep || ''}
          onChange={(e) => setSelectedStep(e.target.value)}
          className="h-6 px-2 text-xs border rounded bg-background"
        >
          {chatSteps.map((node) => (
            <option key={node.step.name} value={node.step.name}>
              {node.step.name} ({node.statusLabel})
            </option>
          ))}
        </select>
        {isSelectedRunning && (
          <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
        )}
      </div>

      {/* Messages table - only for selected step */}
      {selectedStep && (
        <StepMessagesTable
          dagName={dagRun.name}
          dagRunId={dagRun.dagRunId}
          stepName={selectedStep}
          isRunning={isSelectedRunning || false}
          subDAGRunId={isSubDAGRun ? dagRun.dagRunId : undefined}
          rootDagName={isSubDAGRun ? dagRun.rootDAGRunName : undefined}
          rootDagRunId={isSubDAGRun ? dagRun.rootDAGRunId : undefined}
        />
      )}
    </div>
  );
}
