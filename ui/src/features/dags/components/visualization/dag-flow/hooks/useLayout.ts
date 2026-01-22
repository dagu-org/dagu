import { useMemo } from 'react';
import dagre from 'dagre';
import type { DagNode, DagEdge, LayoutDirection } from '../types';

interface LayoutOptions {
  /** Layout direction: TB (top-to-bottom) or LR (left-to-right) */
  direction: LayoutDirection;
  /** Width of each node */
  nodeWidth?: number;
  /** Height of each node */
  nodeHeight?: number;
  /** Horizontal spacing between nodes */
  nodeSpacing?: number;
  /** Vertical spacing between ranks */
  rankSpacing?: number;
}

const DEFAULT_NODE_WIDTH = 180;
const DEFAULT_NODE_HEIGHT = 50;
const DEFAULT_NODE_SPACING = 50;
const DEFAULT_RANK_SPACING = 50;

/**
 * Calculate node positions using dagre layout algorithm
 */
export function calculateLayout(
  nodes: DagNode[],
  edges: DagEdge[],
  options: LayoutOptions
): DagNode[] {
  const {
    direction,
    nodeWidth = DEFAULT_NODE_WIDTH,
    nodeHeight = DEFAULT_NODE_HEIGHT,
    nodeSpacing = DEFAULT_NODE_SPACING,
    rankSpacing = DEFAULT_RANK_SPACING,
  } = options;

  // Create dagre graph
  const dagreGraph = new dagre.graphlib.Graph();
  dagreGraph.setDefaultEdgeLabel(() => ({}));
  dagreGraph.setGraph({
    rankdir: direction,
    nodesep: nodeSpacing,
    ranksep: rankSpacing,
    marginx: 20,
    marginy: 20,
  });

  // Add nodes to dagre graph
  nodes.forEach((node) => {
    dagreGraph.setNode(node.id, {
      width: nodeWidth,
      height: nodeHeight,
    });
  });

  // Add edges to dagre graph
  edges.forEach((edge) => {
    dagreGraph.setEdge(edge.source, edge.target);
  });

  // Calculate layout
  dagre.layout(dagreGraph);

  // Apply calculated positions to nodes
  return nodes.map((node) => {
    const nodeWithPosition = dagreGraph.node(node.id);
    if (!nodeWithPosition) {
      return node;
    }

    return {
      ...node,
      position: {
        x: nodeWithPosition.x - nodeWidth / 2,
        y: nodeWithPosition.y - nodeHeight / 2,
      },
    };
  });
}

/**
 * Hook to calculate layout for DAG nodes
 */
export function useLayout(
  nodes: DagNode[],
  edges: DagEdge[],
  direction: LayoutDirection
): DagNode[] {
  return useMemo(() => {
    if (nodes.length === 0) {
      return [];
    }

    return calculateLayout(nodes, edges, { direction });
  }, [nodes, edges, direction]);
}
