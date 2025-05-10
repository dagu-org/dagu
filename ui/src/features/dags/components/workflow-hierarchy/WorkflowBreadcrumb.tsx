import { ChevronRight, Home } from 'lucide-react';
import React from 'react';
import { useWorkflowHierarchy } from '../../contexts/WorkflowHierarchyContext';

type WorkflowBreadcrumbProps = {
  className?: string;
};

/**
 * WorkflowBreadcrumb component displays the current position in the workflow hierarchy
 * and allows navigation back to parent workflows
 */
const WorkflowBreadcrumb: React.FC<WorkflowBreadcrumbProps> = ({
  className = '',
}) => {
  const { getBreadcrumbPath, navigateToWorkflow, rootName, rootWorkflowId } =
    useWorkflowHierarchy();

  const breadcrumbPath = getBreadcrumbPath();

  if (breadcrumbPath.length <= 1) {
    return null; // Don't show breadcrumbs for root workflow
  }

  return (
    <nav
      className={`flex items-center text-sm ${className}`}
      aria-label="Workflow hierarchy"
    >
      {/* Root workflow (home) */}
      <button
        onClick={() =>
          rootWorkflowId &&
          rootName &&
          navigateToWorkflow(rootWorkflowId, rootName)
        }
        className="flex items-center text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 transition-colors"
        aria-label="Navigate to root workflow"
      >
        <Home className="h-3.5 w-3.5 mr-1" />
        <span className="font-medium">Root</span>
      </button>

      {/* Breadcrumb items */}
      {breadcrumbPath.map((item, index) => {
        // Skip the root item as we already have the home button
        if (index === 0) return null;

        const isLast = index === breadcrumbPath.length - 1;

        return (
          <React.Fragment key={item.workflowId}>
            <ChevronRight className="h-3 w-3 mx-2 text-slate-400" />

            {isLast ? (
              // Current workflow (not clickable)
              <span className="font-medium text-slate-900 dark:text-slate-200">
                {item.parentStepName || 'Child'}
              </span>
            ) : (
              // Parent workflow (clickable)
              <button
                onClick={() => navigateToWorkflow(item.workflowId, item.name)}
                className="font-medium text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-slate-200 transition-colors"
              >
                {item.parentStepName || 'Child'}
              </button>
            )}
          </React.Fragment>
        );
      })}
    </nav>
  );
};

export default WorkflowBreadcrumb;
