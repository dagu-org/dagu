/**
 * DAGSpecReadOnly component displays a DAG specification in readonly mode.
 * Used in status views to show the DAG YAML without editing capabilities.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { cn } from '@/lib/utils';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import DAGEditorWithDocs from './DAGEditorWithDocs';

/**
 * Props for the DAGSpecReadOnly component
 */
type DAGSpecReadOnlyProps = {
  /** DAG name to fetch the spec for */
  dagName: string;
  /** DAG run ID */
  dagRunId: string;
  /** Optional sub-DAG run ID for viewing subdag specs */
  subDAGRunId?: string;
  /** Additional class name for the container */
  className?: string;
};

/**
 * Skeleton placeholder for the editor while loading
 */
function EditorSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        'flex flex-col bg-surface border border-border rounded-lg overflow-hidden min-h-[300px] max-h-[70vh]',
        className
      )}
    >
      <div className="flex-shrink-0 flex justify-between items-center p-2 border-b border-border">
        <div className="h-6 w-16 bg-muted animate-pulse rounded" />
      </div>
      <div className="flex-1 p-4 space-y-2">
        <div className="h-4 w-3/4 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/2 bg-muted animate-pulse rounded" />
        <div className="h-4 w-2/3 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/3 bg-muted animate-pulse rounded" />
        <div className="h-4 w-3/4 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/2 bg-muted animate-pulse rounded" />
      </div>
    </div>
  );
}

/**
 * DAGSpecReadOnly fetches and displays a DAG specification in readonly mode
 * with the Schema Documentation sidebar available for reference.
 */
function DAGSpecReadOnly({ dagName, dagRunId, subDAGRunId, className }: DAGSpecReadOnlyProps) {
  const appBarContext = React.useContext(AppBarContext);

  // Select endpoint based on whether this is a subdag
  const endpoint = subDAGRunId
    ? '/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/spec' as const
    : '/dag-runs/{name}/{dagRunId}/spec' as const;

  // Build path params conditionally
  const pathParams = subDAGRunId
    ? { name: dagName, dagRunId: dagRunId, subDAGRunId: subDAGRunId }
    : { name: dagName, dagRunId: dagRunId };

  // Fetch DAG specification data using the appropriate endpoint
  const { data, isLoading, error } = useQuery(endpoint, {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      path: pathParams as any,
    },
  });

  if (isLoading) {
    return <EditorSkeleton className={className} />;
  }

  if (error) {
    return (
      <div className="text-sm text-danger p-4">
        Failed to load DAG spec: {error.message ?? 'Unknown error'}
      </div>
    );
  }

  if (!data?.spec) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No DAG spec available for this DAG.
      </div>
    );
  }

  return (
    <DAGEditorWithDocs
      value={data.spec}
      readOnly={true}
      className={className}
    />
  );
}

export default DAGSpecReadOnly;
