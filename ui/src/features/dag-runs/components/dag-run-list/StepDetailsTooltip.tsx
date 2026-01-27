import React from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '../../../../components/ui/tooltip';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';

type DAGRunSummary = components['schemas']['DAGRunSummary'];
type Node = components['schemas']['Node'];

type StepDetailsTooltipProps = {
  dagRun: DAGRunSummary;
  children: React.ReactNode;
};

function getStepName(node: Node, index: number) {
  return node.step?.name || `Step ${index + 1}`;
}

function renderStepList(title: string, nodes: Node[], colorClass: string) {
  if (nodes.length === 0) {
    return null;
  }

  const maxVisible = 5;
  const visibleSteps = nodes.slice(0, maxVisible);
  const remaining = nodes.length - visibleSteps.length;

  return (
    <div className="space-y-1">
      <div className={`text-xs font-semibold ${colorClass}`}>{title}</div>
      <ul className="text-xs space-y-0.5">
        {visibleSteps.map((node, idx) => (
          <li key={`${node.step?.name || idx}-${idx}`}>
            {idx + 1}. {getStepName(node, idx)}
          </li>
        ))}
      </ul>
      {remaining > 0 && (
        <div className="text-xs text-muted-foreground">
          +{remaining} more step{remaining > 1 ? 's' : ''}
        </div>
      )}
    </div>
  );
}

export function StepDetailsTooltip({
  dagRun,
  children,
}: StepDetailsTooltipProps) {
  const appBarContext = React.useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';
  const [isOpen, setIsOpen] = React.useState(false);

  const canRequestDetails = Boolean(dagRun.name && dagRun.dagRunId);

  const queryKey =
    isOpen && canRequestDetails
      ? '/dag-runs/{name}/{dagRunId}'
      : (undefined as unknown as '/dag-runs/{name}/{dagRunId}');

  const { data, error, isLoading } = useQuery(
    queryKey,
    isOpen && canRequestDetails
      ? {
          params: {
            path: {
              name: dagRun.name,
              dagRunId: dagRun.dagRunId,
            },
            query: {
              remoteNode,
            },
          },
        }
      : null,
    {
      revalidateOnFocus: false,
      revalidateOnReconnect: false,
      refreshInterval: 0,
    }
  );

  const nodes = data?.dagRunDetails?.nodes || [];
  const runningSteps = nodes.filter(
    (node) => node.status === NodeStatus.Running
  );
  const failedSteps = nodes.filter((node) => node.status === NodeStatus.Failed);

  const hasStepData = runningSteps.length > 0 || failedSteps.length > 0;

  return (
    <Tooltip open={isOpen} onOpenChange={setIsOpen}>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent className="max-w-sm space-y-2">
        {!canRequestDetails && (
          <div className="text-xs text-muted-foreground">
            Step details are unavailable for this DAG run.
          </div>
        )}
        {canRequestDetails && isLoading && (
          <div className="text-xs text-muted-foreground">
            Loading step details...
          </div>
        )}
        {canRequestDetails && error && (
          <div className="text-xs text-error">
            Failed to load step details
          </div>
        )}
        {canRequestDetails && !isLoading && !error && (
          <>
            {hasStepData ? (
              <>
                {renderStepList(
                  'Running steps',
                  runningSteps,
                  'text-success'
                )}
                {renderStepList('Failed steps', failedSteps, 'text-error')}
              </>
            ) : (
              <div className="text-xs text-muted-foreground">
                No running or failed steps at the moment.
              </div>
            )}
          </>
        )}
      </TooltipContent>
    </Tooltip>
  );
}
