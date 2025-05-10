import {
  AlertCircle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  GitBranch,
  GitMerge,
  PlayCircle,
  XCircle,
} from 'lucide-react';
import React, { useState } from 'react';
import { Status } from '../../../../api/v2/schema';
import {
  useWorkflowHierarchy,
  WorkflowHierarchyItem,
} from '../../contexts/WorkflowHierarchyContext';

type WorkflowHierarchySidebarProps = {
  className?: string;
};

/**
 * WorkflowHierarchySidebar component displays a tree view of the workflow hierarchy
 * and allows navigation between workflows
 */
const WorkflowHierarchySidebar: React.FC<WorkflowHierarchySidebarProps> = ({
  className = '',
}) => {
  const {
    hierarchyMap,
    rootWorkflowId,
    currentWorkflow,
    navigateToWorkflow,
    hasChildWorkflows,
    getChildWorkflows,
  } = useWorkflowHierarchy();

  // Track expanded nodes
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());

  // Toggle node expansion
  const toggleNode = (workflowId: string) => {
    const newExpandedNodes = new Set(expandedNodes);

    if (newExpandedNodes.has(workflowId)) {
      newExpandedNodes.delete(workflowId);
    } else {
      newExpandedNodes.add(workflowId);
    }

    setExpandedNodes(newExpandedNodes);
  };

  // If there's no root workflow, don't render anything
  if (!rootWorkflowId || !hierarchyMap.size) {
    return null;
  }

  // Get the root workflow
  const rootWorkflow = hierarchyMap.get(rootWorkflowId);

  if (!rootWorkflow) {
    return null;
  }

  return (
    <div
      className={`border border-slate-200 dark:border-slate-700 rounded-md p-2 ${className}`}
    >
      <h3 className="text-sm font-semibold mb-2 text-slate-700 dark:text-slate-300 flex items-center">
        <GitMerge className="h-4 w-4 mr-1.5" />
        Workflow Hierarchy
      </h3>

      <div className="overflow-y-auto max-h-[300px]">
        <TreeNode
          item={rootWorkflow}
          level={0}
          isExpanded={expandedNodes.has(rootWorkflow.workflowId)}
          isSelected={currentWorkflow?.workflowId === rootWorkflow.workflowId}
          onToggle={toggleNode}
          onSelect={navigateToWorkflow}
          getChildWorkflows={getChildWorkflows}
          hasChildWorkflows={hasChildWorkflows}
        />
      </div>
    </div>
  );
};

// Props for the TreeNode component
type TreeNodeProps = {
  item: WorkflowHierarchyItem;
  level: number;
  isExpanded: boolean;
  isSelected: boolean;
  onToggle: (workflowId: string) => void;
  onSelect: (workflowId: string, name: string) => void;
  getChildWorkflows: (workflowId: string) => WorkflowHierarchyItem[];
  hasChildWorkflows: (workflowId: string) => boolean;
};

/**
 * TreeNode component renders a single node in the workflow hierarchy tree
 */
const TreeNode: React.FC<TreeNodeProps> = ({
  item,
  level,
  isExpanded,
  isSelected,
  onToggle,
  onSelect,
  getChildWorkflows,
  hasChildWorkflows,
}) => {
  const hasChildren = hasChildWorkflows(item.workflowId);
  const children = isExpanded ? getChildWorkflows(item.workflowId) : [];

  // Get status icon based on workflow status
  const getStatusIcon = (status?: Status) => {
    switch (status) {
      case Status.Success:
        return <CheckCircle2 className="h-3 w-3 text-green-500" />;
      case Status.Failed:
        return <XCircle className="h-3 w-3 text-red-500" />;
      case Status.Cancelled:
        return <AlertCircle className="h-3 w-3 text-pink-500" />;
      case Status.Running:
        return <PlayCircle className="h-3 w-3 text-lime-500 animate-pulse" />;
      case Status.NotStarted:
      default:
        return <Clock className="h-3 w-3 text-slate-400" />;
    }
  };

  return (
    <div>
      <div
        className={`
          flex items-center py-1 px-1 rounded-md text-sm
          ${isSelected ? 'bg-slate-100 dark:bg-slate-800' : ''}
          ${isSelected ? 'font-medium' : ''}
        `}
        style={{ paddingLeft: `${level * 12 + 4}px` }}
      >
        {/* Expand/collapse button */}
        {hasChildren ? (
          <button
            onClick={() => onToggle(item.workflowId)}
            className="mr-1 text-slate-500 hover:text-slate-700 dark:text-slate-400 dark:hover:text-slate-300"
            aria-label={isExpanded ? 'Collapse' : 'Expand'}
          >
            {isExpanded ? (
              <ChevronDown className="h-3.5 w-3.5" />
            ) : (
              <ChevronRight className="h-3.5 w-3.5" />
            )}
          </button>
        ) : (
          <span className="w-3.5 mr-1"></span>
        )}

        {/* Workflow node */}
        <button
          onClick={() => onSelect(item.workflowId, item.name)}
          className={`
            flex items-center gap-1.5 hover:text-slate-900 dark:hover:text-slate-200
            ${isSelected ? 'text-slate-900 dark:text-slate-200' : 'text-slate-700 dark:text-slate-400'}
          `}
        >
          <GitBranch className="h-3.5 w-3.5" />
          <span className="truncate max-w-[150px]">
            {item.parentStepName || (level === 0 ? 'Root' : 'Child')}
          </span>
          {getStatusIcon(item.status)}
        </button>
      </div>

      {/* Render children if expanded */}
      {isExpanded &&
        children.map((child) => (
          <TreeNode
            key={child.workflowId}
            item={child}
            level={level + 1}
            isExpanded={false}
            isSelected={isSelected}
            onToggle={onToggle}
            onSelect={onSelect}
            getChildWorkflows={getChildWorkflows}
            hasChildWorkflows={hasChildWorkflows}
          />
        ))}
    </div>
  );
};

export default WorkflowHierarchySidebar;
