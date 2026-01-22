import type { Node, Edge, BuiltInNode } from '@xyflow/react';
import type { NodeStatus, components } from '@/api/v2/schema';

/** Data stored in each DAG node */
export type DagNodeData = {
  /** Display label for the node */
  label: string;
  /** Original step name (used for click handlers) */
  stepName: string;
  /** Current execution status */
  status: NodeStatus;
  /** Type of node for rendering */
  nodeType: 'step' | 'subdag' | 'parallel';
  /** Original step definition */
  step: components['schemas']['Step'];
  /** Runtime node data (only for status view) */
  node?: components['schemas']['Node'];
  /** Allow additional properties for xyflow compatibility */
  [key: string]: unknown;
};

/** Custom node type with DagNodeData */
export type DagNode = Node<DagNodeData, 'step' | 'subdag' | 'parallel'>;

/** Data stored in each edge */
export type DagEdgeData = {
  /** Status of the source node */
  sourceStatus: NodeStatus;
  /** Status of the target node */
  targetStatus: NodeStatus;
  /** Allow additional properties for xyflow compatibility */
  [key: string]: unknown;
};

/** Custom edge type with DagEdgeData */
export type DagEdge = Edge<DagEdgeData, 'dependency'>;

/** Layout direction: TB = top-to-bottom, LR = left-to-right */
export type LayoutDirection = 'TB' | 'LR';

/** Props for the DagFlow component */
export interface DagFlowProps {
  /** Steps or nodes to visualize */
  steps: components['schemas']['Step'][] | components['schemas']['Node'][];
  /** Type of view - status shows runtime state, config shows definition */
  type: 'status' | 'config';
  /** Layout direction */
  direction: LayoutDirection;
  /** Callback when direction changes */
  onDirectionChange?: (direction: LayoutDirection) => void;
  /** Callback for node click (double-click navigates to sub-DAG) */
  onClickNode?: (stepName: string) => void;
  /** Callback for node right-click (opens status modal) */
  onRightClickNode?: (stepName: string) => void;
  /** Whether the graph is displayed in expanded modal view */
  isExpandedView?: boolean;
}

/** All possible node types in our DAG flow */
export type DagNodeTypes = DagNode | BuiltInNode;
