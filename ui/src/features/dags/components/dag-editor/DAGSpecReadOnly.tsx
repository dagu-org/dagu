/**
 * DAGSpecReadOnly component displays a DAG specification in readonly mode.
 * Used in status views to show the DAG YAML without editing capabilities.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useQuery } from '../../../../hooks/api';
import LoadingIndicator from '../../../../ui/LoadingIndicator';
import DAGEditorWithDocs from './DAGEditorWithDocs';

/**
 * Props for the DAGSpecReadOnly component
 */
type DAGSpecReadOnlyProps = {
  /** DAG name to fetch the spec for */
  dagName: string;
  /** DAG run ID */
  dagRunId: string;
  /** Additional class name for the container */
  className?: string;
};

/**
 * DAGSpecReadOnly fetches and displays a DAG specification in readonly mode
 * with the Schema Documentation sidebar available for reference.
 */
function DAGSpecReadOnly({ dagName, dagRunId, className }: DAGSpecReadOnlyProps) {
  const appBarContext = React.useContext(AppBarContext);

  // Fetch DAG specification data using the dag-runs spec endpoint
  const { data, isLoading, error } = useQuery('/dag-runs/{name}/{dagRunId}/spec', {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
      path: {
        name: dagName,
        dagRunId: dagRunId,
      },
    },
  });

  if (isLoading) {
    return <LoadingIndicator />;
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
