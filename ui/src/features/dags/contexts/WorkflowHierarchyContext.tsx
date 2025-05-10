import React, { createContext, useCallback, useContext, useState } from 'react';

import { components } from '../../../api/v2/schema';

// Define the structure for a workflow in the hierarchy
export type WorkflowHierarchyItem = {
  workflowId: string;
  name: string;
  parentWorkflowId?: string;
  parentName?: string;
  parentStepName?: string;
  level: number;
  childWorkflowIds?: string[];
  isLoaded: boolean;
  status?: number; // Status of the workflow (from components['schemas']['Status'])
  statusLabel?: string; // Human-readable status label
  details?: components['schemas']['WorkflowDetails']; // Optional workflow details
};

// Define the context type
type WorkflowHierarchyContextType = {
  // The current workflow being viewed
  currentWorkflow: WorkflowHierarchyItem | null;
  // The complete hierarchy map (workflowId -> hierarchy item)
  hierarchyMap: Map<string, WorkflowHierarchyItem>;
  // The root workflow ID
  rootWorkflowId: string | null;
  // The root workflow name
  rootName: string | null;
  // Function to navigate to a specific workflow in the hierarchy
  navigateToWorkflow: (workflowId: string, name: string) => void;
  // Function to navigate to a child workflow
  navigateToChildWorkflow: (
    parentWorkflowId: string,
    parentName: string,
    childWorkflowId: string,
    stepName: string
  ) => void;
  // Function to navigate back to the parent workflow
  navigateToParentWorkflow: () => void;
  // Function to add a child workflow to the hierarchy
  addChildWorkflow: (
    parentWorkflowId: string,
    parentName: string,
    childWorkflowId: string,
    stepName: string
  ) => void;
  // Function to set the root workflow
  setRootWorkflow: (workflowId: string, name: string) => void;
  // Function to check if a workflow has child workflows
  hasChildWorkflows: (workflowId: string) => boolean;
  // Function to get child workflows for a specific workflow
  getChildWorkflows: (workflowId: string) => WorkflowHierarchyItem[];
  // Function to get the breadcrumb path to the current workflow
  getBreadcrumbPath: () => WorkflowHierarchyItem[];
  // Function to clear the hierarchy
  clearHierarchy: () => void;
};

// Create the context with default values
export const WorkflowHierarchyContext =
  createContext<WorkflowHierarchyContextType>({
    currentWorkflow: null,
    hierarchyMap: new Map(),
    rootWorkflowId: null,
    rootName: null,
    navigateToWorkflow: () => {},
    navigateToChildWorkflow: () => {},
    navigateToParentWorkflow: () => {},
    addChildWorkflow: () => {},
    setRootWorkflow: () => {},
    hasChildWorkflows: () => false,
    getChildWorkflows: () => [],
    getBreadcrumbPath: () => [],
    clearHierarchy: () => {},
  });

// Provider component
export const WorkflowHierarchyProvider: React.FC<{
  children: React.ReactNode;
}> = ({ children }) => {
  // State for the current workflow
  const [currentWorkflow, setCurrentWorkflow] =
    useState<WorkflowHierarchyItem | null>(null);
  // State for the hierarchy map
  const [hierarchyMap, setHierarchyMap] = useState<
    Map<string, WorkflowHierarchyItem>
  >(new Map());
  // State for the root workflow ID
  const [rootWorkflowId, setRootWorkflowId] = useState<string | null>(null);
  // State for the root workflow name
  const [rootName, setRootName] = useState<string | null>(null);

  // Function to set the root workflow
  const setRootWorkflow = useCallback((workflowId: string, name: string) => {
    const newHierarchyMap = new Map<string, WorkflowHierarchyItem>();
    const rootItem: WorkflowHierarchyItem = {
      workflowId,
      name,
      level: 0,
      childWorkflowIds: [],
      isLoaded: true,
    };

    newHierarchyMap.set(workflowId, rootItem);

    setRootWorkflowId(workflowId);
    setRootName(name);
    setHierarchyMap(newHierarchyMap);
    setCurrentWorkflow(rootItem);
  }, []);

  // Function to navigate to a specific workflow in the hierarchy
  const navigateToWorkflow = useCallback(
    (workflowId: string, name: string) => {
      const workflow = hierarchyMap.get(workflowId);

      if (workflow) {
        setCurrentWorkflow(workflow);
      } else {
        // If the workflow is not in the hierarchy, add it as the root
        setRootWorkflow(workflowId, name);
      }
    },
    [hierarchyMap, setRootWorkflow]
  );

  // Function to navigate to a child workflow
  const navigateToChildWorkflow = useCallback(
    (
      parentWorkflowId: string,
      parentName: string,
      childWorkflowId: string,
      stepName: string
    ) => {
      const newHierarchyMap = new Map(hierarchyMap);
      const parentWorkflow = newHierarchyMap.get(parentWorkflowId);

      if (!parentWorkflow) {
        // If the parent workflow is not in the hierarchy, add it first
        const newParentWorkflow: WorkflowHierarchyItem = {
          workflowId: parentWorkflowId,
          name: parentName,
          level: 0,
          childWorkflowIds: [childWorkflowId],
          isLoaded: true,
        };

        newHierarchyMap.set(parentWorkflowId, newParentWorkflow);

        // If there's no root workflow yet, set this as the root
        if (!rootWorkflowId) {
          setRootWorkflowId(parentWorkflowId);
          setRootName(parentName);
        }
      } else {
        // Add the child workflow ID to the parent's children if it's not already there
        if (!parentWorkflow.childWorkflowIds) {
          parentWorkflow.childWorkflowIds = [];
        }

        if (!parentWorkflow.childWorkflowIds.includes(childWorkflowId)) {
          parentWorkflow.childWorkflowIds.push(childWorkflowId);
        }

        newHierarchyMap.set(parentWorkflowId, parentWorkflow);
      }

      // Add or update the child workflow in the hierarchy
      const childWorkflow = newHierarchyMap.get(childWorkflowId) || {
        workflowId: childWorkflowId,
        name: parentName, // Use parent name as the DAG name is the same
        parentWorkflowId,
        parentName,
        parentStepName: stepName,
        level: (parentWorkflow?.level || 0) + 1,
        childWorkflowIds: [],
        isLoaded: false,
      };

      newHierarchyMap.set(childWorkflowId, childWorkflow);

      // Update the hierarchy map and set the current workflow to the child
      setHierarchyMap(newHierarchyMap);
      setCurrentWorkflow(childWorkflow);
    },
    [hierarchyMap, rootWorkflowId]
  );

  // Function to navigate back to the parent workflow
  const navigateToParentWorkflow = useCallback(() => {
    if (currentWorkflow?.parentWorkflowId) {
      const parentWorkflow = hierarchyMap.get(currentWorkflow.parentWorkflowId);

      if (parentWorkflow) {
        setCurrentWorkflow(parentWorkflow);
      }
    }
  }, [currentWorkflow, hierarchyMap]);

  // Function to add a child workflow to the hierarchy
  const addChildWorkflow = useCallback(
    (
      parentWorkflowId: string,
      parentName: string,
      childWorkflowId: string,
      stepName: string
    ) => {
      const newHierarchyMap = new Map(hierarchyMap);
      const parentWorkflow = newHierarchyMap.get(parentWorkflowId);

      if (!parentWorkflow) {
        // If the parent workflow is not in the hierarchy, add it first
        const newParentWorkflow: WorkflowHierarchyItem = {
          workflowId: parentWorkflowId,
          name: parentName,
          level: 0,
          childWorkflowIds: [childWorkflowId],
          isLoaded: true,
        };

        newHierarchyMap.set(parentWorkflowId, newParentWorkflow);

        // If there's no root workflow yet, set this as the root
        if (!rootWorkflowId) {
          setRootWorkflowId(parentWorkflowId);
          setRootName(parentName);
        }
      } else {
        // Add the child workflow ID to the parent's children if it's not already there
        if (!parentWorkflow.childWorkflowIds) {
          parentWorkflow.childWorkflowIds = [];
        }

        if (!parentWorkflow.childWorkflowIds.includes(childWorkflowId)) {
          parentWorkflow.childWorkflowIds.push(childWorkflowId);
        }

        newHierarchyMap.set(parentWorkflowId, parentWorkflow);
      }

      // Add the child workflow to the hierarchy if it's not already there
      if (!newHierarchyMap.has(childWorkflowId)) {
        const childWorkflow: WorkflowHierarchyItem = {
          workflowId: childWorkflowId,
          name: parentName, // Use parent name as the DAG name is the same
          parentWorkflowId,
          parentName,
          parentStepName: stepName,
          level: (parentWorkflow?.level || 0) + 1,
          childWorkflowIds: [],
          isLoaded: false,
        };

        newHierarchyMap.set(childWorkflowId, childWorkflow);
      }

      // Update the hierarchy map
      setHierarchyMap(newHierarchyMap);
    },
    [hierarchyMap, rootWorkflowId]
  );

  // Function to check if a workflow has child workflows
  const hasChildWorkflows = useCallback(
    (workflowId: string) => {
      const workflow = hierarchyMap.get(workflowId);
      return !!(
        workflow?.childWorkflowIds && workflow.childWorkflowIds.length > 0
      );
    },
    [hierarchyMap]
  );

  // Function to get child workflows for a specific workflow
  const getChildWorkflows = useCallback(
    (workflowId: string) => {
      const workflow = hierarchyMap.get(workflowId);

      if (!workflow?.childWorkflowIds) {
        return [];
      }

      return workflow.childWorkflowIds
        .map((id) => hierarchyMap.get(id))
        .filter((item): item is WorkflowHierarchyItem => !!item);
    },
    [hierarchyMap]
  );

  // Function to get the breadcrumb path to the current workflow
  const getBreadcrumbPath = useCallback(() => {
    if (!currentWorkflow) {
      return [];
    }

    const path: WorkflowHierarchyItem[] = [];
    let current: WorkflowHierarchyItem | undefined = currentWorkflow;

    while (current) {
      path.unshift(current);

      if (current.parentWorkflowId) {
        current = hierarchyMap.get(current.parentWorkflowId);
      } else {
        break;
      }
    }

    return path;
  }, [currentWorkflow, hierarchyMap]);

  // Function to clear the hierarchy
  const clearHierarchy = useCallback(() => {
    setHierarchyMap(new Map());
    setCurrentWorkflow(null);
    setRootWorkflowId(null);
    setRootName(null);
  }, []);

  // Create the context value
  const contextValue: WorkflowHierarchyContextType = {
    currentWorkflow,
    hierarchyMap,
    rootWorkflowId,
    rootName,
    navigateToWorkflow,
    navigateToChildWorkflow,
    navigateToParentWorkflow,
    addChildWorkflow,
    setRootWorkflow,
    hasChildWorkflows,
    getChildWorkflows,
    getBreadcrumbPath,
    clearHierarchy,
  };

  return (
    <WorkflowHierarchyContext.Provider value={contextValue}>
      {children}
    </WorkflowHierarchyContext.Provider>
  );
};

// Custom hook for using the workflow hierarchy context
export const useWorkflowHierarchy = () => useContext(WorkflowHierarchyContext);
