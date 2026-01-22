import { NodeStatus, components } from '@/api/v2/schema';
import type { DagNode, DagEdge, DagNodeData } from '../types';

type Steps = components['schemas']['Step'][] | components['schemas']['Node'][];

/**
 * Transform steps/nodes into xyflow format (nodes and edges)
 */
export function transformStepsToGraph(
  steps: Steps,
  type: 'status' | 'config'
): { nodes: DagNode[]; edges: DagEdge[] } {
  const nodes: DagNode[] = [];
  const edges: DagEdge[] = [];

  // Build a map of step names for quick lookup
  const stepMap = new Map<string, { step: components['schemas']['Step']; status: NodeStatus; node?: components['schemas']['Node'] }>();

  steps.forEach((item) => {
    const step = type === 'status'
      ? (item as components['schemas']['Node']).step
      : (item as components['schemas']['Step']);
    const node = type === 'status' ? (item as components['schemas']['Node']) : undefined;
    const status = type === 'status'
      ? (item as components['schemas']['Node']).status
      : NodeStatus.NotStarted;

    stepMap.set(step.name, { step, status, node });
  });

  // Create nodes and edges
  steps.forEach((item) => {
    const step = type === 'status'
      ? (item as components['schemas']['Node']).step
      : (item as components['schemas']['Step']);
    const node = type === 'status' ? (item as components['schemas']['Node']) : undefined;
    const status = type === 'status'
      ? (item as components['schemas']['Node']).status
      : NodeStatus.NotStarted;

    // Determine node type
    const isSubDag = !!step.call;
    const isParallel = !!step.parallel;
    let nodeType: DagNodeData['nodeType'] = 'step';
    if (isSubDag && isParallel) {
      nodeType = 'parallel';
    } else if (isSubDag) {
      nodeType = 'subdag';
    }

    // Build label
    let label = step.name;
    if (isSubDag && step.call) {
      if (isParallel && node?.subRuns) {
        label = `${step.name} -> ${step.call} x${node.subRuns.length}`;
      } else {
        label = `${step.name} -> ${step.call}`;
      }
    }

    // Create node
    const dagNode: DagNode = {
      id: step.name,
      type: nodeType,
      position: { x: 0, y: 0 }, // Will be set by layout
      data: {
        label,
        stepName: step.name,
        status,
        nodeType,
        step,
        node,
      },
    };
    nodes.push(dagNode);

    // Create edges from dependencies
    if (step.depends) {
      step.depends.forEach((depName) => {
        const sourceData = stepMap.get(depName);
        const sourceStatus = sourceData?.status ?? NodeStatus.NotStarted;

        edges.push({
          id: `${depName}->${step.name}`,
          source: depName,
          target: step.name,
          type: 'dependency',
          data: {
            sourceStatus,
            targetStatus: status,
          },
        });
      });
    }
  });

  return { nodes, edges };
}
