import React, { useEffect, useState } from 'react';
import { components } from '../../../../api/v2/schema';
import { useChildWorkflowService } from '../../api/childWorkflowService';
import { useWorkflowHierarchy } from '../../contexts/WorkflowHierarchyContext';
import DAGStatus from '../DAGStatus';
import WorkflowBreadcrumb from './WorkflowBreadcrumb';

type ChildWorkflowDetailsProps = {
  className?: string;
};

/**
 * ChildWorkflowDetails component displays details of a child workflow
 * with the same level of information as parent workflows
 */
const ChildWorkflowDetails: React.FC<ChildWorkflowDetailsProps> = ({
  className = '',
}) => {
  const { currentWorkflow, rootName } = useWorkflowHierarchy();

  const { getChildWorkflowDetails } = useChildWorkflowService();

  // State for workflow details
  const [workflowDetails, setWorkflowDetails] = useState<
    components['schemas']['WorkflowDetails'] | null
  >(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Fetch child workflow details when the current workflow changes
  useEffect(() => {
    const fetchChildWorkflowDetails = async () => {
      if (!currentWorkflow || !currentWorkflow.parentWorkflowId || !rootName) {
        return;
      }

      setLoading(true);
      setError(null);

      try {
        const result = await getChildWorkflowDetails(
          rootName,
          currentWorkflow.parentWorkflowId,
          currentWorkflow.workflowId
        );

        if (result.error) {
          setError(
            result.error.message || 'Failed to fetch child workflow details'
          );
        } else if (result.data) {
          setWorkflowDetails(result.data);

          // Update the current workflow with status information
          if (result.data.status !== undefined) {
            currentWorkflow.status = result.data.status;
            currentWorkflow.statusLabel = result.data.statusLabel;
            currentWorkflow.details = result.data;
          }
        }
      } catch (err) {
        setError('An unexpected error occurred');
        console.error(err);
      } finally {
        setLoading(false);
      }
    };

    fetchChildWorkflowDetails();
  }, [currentWorkflow, rootName, getChildWorkflowDetails]);

  // If there's no current workflow or it's the root workflow, don't render anything
  if (!currentWorkflow || !currentWorkflow.parentWorkflowId) {
    return null;
  }

  return (
    <div className={`space-y-4 ${className}`}>
      {/* Breadcrumb navigation */}
      <WorkflowBreadcrumb className="mb-4" />

      {/* Loading state */}
      {loading && (
        <div className="flex justify-center items-center h-32">
          <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-primary"></div>
        </div>
      )}

      {/* Error state */}
      {error && (
        <div className="bg-red-50 dark:bg-red-900/10 border border-red-200 dark:border-red-800 rounded-md p-4 text-red-700 dark:text-red-400">
          <p className="font-medium">Error loading child workflow details</p>
          <p className="text-sm mt-1">{error}</p>
        </div>
      )}

      {/* Child workflow details */}
      {!loading && !error && workflowDetails && (
        <>
          <div className="bg-white dark:bg-slate-900 rounded-xl shadow-md p-4 overflow-hidden">
            <h2 className="text-lg font-semibold mb-2 text-slate-800 dark:text-slate-200">
              Child Workflow: {currentWorkflow.parentStepName || 'Unknown'}
            </h2>
            <p className="text-sm text-slate-600 dark:text-slate-400 mb-4">
              ID:{' '}
              <span className="font-mono">{currentWorkflow.workflowId}</span>
            </p>

            {/* Reuse the DAGStatus component to display workflow details */}
            <DAGStatus workflow={workflowDetails} fileName={rootName || ''} />
          </div>
        </>
      )}
    </div>
  );
};

export default ChildWorkflowDetails;
