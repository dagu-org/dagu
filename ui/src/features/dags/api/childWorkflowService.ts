import { useContext } from 'react';
import { components } from '../../../api/v2/schema';
import { AppBarContext } from '../../../contexts/AppBarContext';
import { useClient } from '../../../hooks/api';

/**
 * Custom hook for child workflow API operations
 */
export const useChildWorkflowService = () => {
  const client = useClient();
  const appBarContext = useContext(AppBarContext);
  const remoteNode = appBarContext.selectedRemoteNode || 'local';

  /**
   * Fetch child workflow details
   * @param parentName Parent DAG name
   * @param parentWorkflowId Parent workflow ID
   * @param childWorkflowId Child workflow ID
   * @returns Child workflow details or error
   */
  const getChildWorkflowDetails = async (
    parentName: string,
    parentWorkflowId: string,
    childWorkflowId: string
  ) => {
    try {
      const response = await client.GET(
        '/workflows/{name}/{workflowId}/children/{childWorkflowId}',
        {
          params: {
            path: {
              name: parentName,
              workflowId: parentWorkflowId,
              childWorkflowId,
            },
            query: {
              remoteNode,
            },
          },
        }
      );

      if (response.error) {
        return { error: response.error, data: null };
      }

      return {
        data: response.data
          ?.workflowDetails as components['schemas']['WorkflowDetails'],
        error: null,
      };
    } catch (error) {
      return {
        error: {
          message: 'Failed to fetch child workflow details',
          details: error,
        },
        data: null,
      };
    }
  };

  /**
   * Fetch child workflow log
   * @param parentName Parent DAG name
   * @param parentWorkflowId Parent workflow ID
   * @param childWorkflowId Child workflow ID
   * @param options Log fetch options
   * @returns Log content or error
   */
  const getChildWorkflowLog = async (
    parentName: string,
    parentWorkflowId: string,
    childWorkflowId: string,
    options?: {
      tail?: number;
      head?: number;
      offset?: number;
      limit?: number;
    }
  ) => {
    try {
      const response = await client.GET(
        '/workflows/{name}/{workflowId}/children/{childWorkflowId}/log',
        {
          params: {
            path: {
              name: parentName,
              workflowId: parentWorkflowId,
              childWorkflowId,
            },
            query: {
              remoteNode,
              ...options,
            },
          },
        }
      );

      if (response.error) {
        return { error: response.error, data: null };
      }

      return {
        data: response.data as components['schemas']['Log'],
        error: null,
      };
    } catch (error) {
      return {
        error: {
          message: 'Failed to fetch child workflow log',
          details: error,
        },
        data: null,
      };
    }
  };

  /**
   * Fetch child workflow step log
   * @param parentName Parent DAG name
   * @param parentWorkflowId Parent workflow ID
   * @param childWorkflowId Child workflow ID
   * @param stepName Step name
   * @param options Log fetch options
   * @returns Step log content or error
   */
  const getChildWorkflowStepLog = async (
    parentName: string,
    parentWorkflowId: string,
    childWorkflowId: string,
    stepName: string,
    options?: {
      tail?: number;
      head?: number;
      offset?: number;
      limit?: number;
    }
  ) => {
    try {
      const response = await client.GET(
        '/workflows/{name}/{workflowId}/children/{childWorkflowId}/steps/{stepName}/log',
        {
          params: {
            path: {
              name: parentName,
              workflowId: parentWorkflowId,
              childWorkflowId,
              stepName,
            },
            query: {
              remoteNode,
              ...options,
            },
          },
        }
      );

      if (response.error) {
        return { error: response.error, data: null };
      }

      return {
        data: response.data as components['schemas']['Log'],
        error: null,
      };
    } catch (error) {
      return {
        error: {
          message: 'Failed to fetch child workflow step log',
          details: error,
        },
        data: null,
      };
    }
  };

  /**
   * Update child workflow step status
   * @param parentName Parent DAG name
   * @param parentWorkflowId Parent workflow ID
   * @param childWorkflowId Child workflow ID
   * @param stepName Step name
   * @param status New status
   * @returns Success or error
   */
  const updateChildWorkflowStepStatus = async (
    parentName: string,
    parentWorkflowId: string,
    childWorkflowId: string,
    stepName: string,
    status: components['schemas']['NodeStatus']
  ) => {
    try {
      const response = await client.PATCH(
        '/workflows/{name}/{workflowId}/children/{childWorkflowId}/steps/{stepName}/status',
        {
          params: {
            path: {
              name: parentName,
              workflowId: parentWorkflowId,
              childWorkflowId,
              stepName,
            },
            query: {
              remoteNode,
            },
          },
          body: {
            status,
          },
        }
      );

      if (response.error) {
        return { error: response.error, success: false };
      }

      return { success: true, error: null };
    } catch (error) {
      return {
        error: {
          message: 'Failed to update child workflow step status',
          details: error,
        },
        success: false,
      };
    }
  };

  /**
   * Build workflow hierarchy by recursively fetching child workflows
   * @param name DAG name
   * @param workflowId Root workflow ID
   * @param maxDepth Maximum depth to fetch (default: 1)
   * @param currentDepth Current depth (used internally)
   * @returns Hierarchy data or error
   */
  const buildWorkflowHierarchy = async (
    name: string,
    workflowId: string,
    maxDepth: number = 1,
    currentDepth: number = 0
  ) => {
    // Define the hierarchy node type
    type WorkflowHierarchyNode = {
      workflowId: string;
      name: string;
      details: components['schemas']['WorkflowDetails'];
      children: WorkflowHierarchyNode[];
      parentWorkflowId?: string;
      parentStepName?: string;
    };

    try {
      // Fetch the current workflow details
      const response = await client.GET('/workflows/{name}/{workflowId}', {
        params: {
          path: {
            name,
            workflowId,
          },
          query: {
            remoteNode,
          },
        },
      });

      if (response.error) {
        return { error: response.error, data: null };
      }

      const workflowDetails = response.data
        ?.workflowDetails as components['schemas']['WorkflowDetails'];

      // Initialize the hierarchy with the current workflow
      const hierarchy: WorkflowHierarchyNode = {
        workflowId,
        name,
        details: workflowDetails,
        children: [],
      };

      // Stop recursion if we've reached the maximum depth
      if (currentDepth >= maxDepth) {
        return { data: hierarchy, error: null };
      }

      // Find steps with child workflows
      const stepsWithChildWorkflows =
        workflowDetails.nodes?.filter(
          (node) => node.subRuns && node.subRuns.length > 0
        ) || [];

      // Recursively fetch child workflows
      const childPromises = stepsWithChildWorkflows.flatMap((node) =>
        (node.subRuns || []).map(async (childRun) => {
          const childHierarchy = await buildWorkflowHierarchy(
            name, // Child workflows use the same DAG name
            childRun.workflowId,
            maxDepth,
            currentDepth + 1
          );

          if (childHierarchy.data) {
            return {
              ...childHierarchy.data,
              parentWorkflowId: workflowId,
              parentStepName: node.step.name,
            } as WorkflowHierarchyNode;
          }

          return null;
        })
      );

      const childResults = await Promise.all(childPromises);
      hierarchy.children = childResults.filter(
        (child): child is WorkflowHierarchyNode => child !== null
      );

      return { data: hierarchy, error: null };
    } catch (error) {
      return {
        error: {
          message: 'Failed to build workflow hierarchy',
          details: error,
        },
        data: null,
      };
    }
  };

  return {
    getChildWorkflowDetails,
    getChildWorkflowLog,
    getChildWorkflowStepLog,
    updateChildWorkflowStepStatus,
    buildWorkflowHierarchy,
  };
};
