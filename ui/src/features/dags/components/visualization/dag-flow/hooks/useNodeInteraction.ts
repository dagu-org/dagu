import { useCallback, useRef } from 'react';
import type { Node } from '@xyflow/react';
import type { DagNodeData } from '../types';

interface UseNodeInteractionOptions {
  /** Callback for node click (triggers after 250ms delay, mapped to right-click behavior) */
  onClick?: (stepName: string) => void;
  /** Callback for node double-click (navigation to sub-DAG) */
  onDoubleClick?: (stepName: string) => void;
  /** Callback for node right-click (status modal) */
  onRightClick?: (stepName: string) => void;
  /** Delay in ms before single click fires (to differentiate from double-click) */
  clickDelay?: number;
}

type NodeClickHandler = (event: React.MouseEvent, node: Node) => void;

interface UseNodeInteractionReturn {
  /** Handler for single click events */
  handleNodeClick: NodeClickHandler;
  /** Handler for double-click events */
  handleNodeDoubleClick: NodeClickHandler;
  /** Handler for right-click (context menu) events */
  handleNodeContextMenu: NodeClickHandler;
}

/**
 * Hook to handle node click interactions with proper single/double-click differentiation
 *
 * Behavior matches the original Mermaid implementation:
 * - Single click: Triggers after 250ms delay (mapped to right-click handler)
 * - Double click: Cancels single click, immediately triggers (navigation)
 * - Right click: Cancels single click, prevents default context menu
 */
export function useNodeInteraction({
  onClick,
  onDoubleClick,
  onRightClick,
  clickDelay = 250,
}: UseNodeInteractionOptions): UseNodeInteractionReturn {
  // Store timeouts per node to allow proper cancellation
  const clickTimeouts = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  const clearNodeTimeout = useCallback((nodeId: string) => {
    const existing = clickTimeouts.current.get(nodeId);
    if (existing) {
      clearTimeout(existing);
      clickTimeouts.current.delete(nodeId);
    }
  }, []);

  const handleNodeClick: NodeClickHandler = useCallback(
    (_event, node) => {
      const data = node.data as DagNodeData;
      const stepName = data.stepName;

      // Clear any existing timeout for this node
      clearNodeTimeout(node.id);

      // Set new timeout for single click
      const timeout = setTimeout(() => {
        clickTimeouts.current.delete(node.id);
        onClick?.(stepName);
      }, clickDelay);

      clickTimeouts.current.set(node.id, timeout);
    },
    [onClick, clickDelay, clearNodeTimeout]
  );

  const handleNodeDoubleClick: NodeClickHandler = useCallback(
    (event, node) => {
      event.stopPropagation();
      const data = node.data as DagNodeData;
      const stepName = data.stepName;

      // Cancel pending single click
      clearNodeTimeout(node.id);

      onDoubleClick?.(stepName);
    },
    [onDoubleClick, clearNodeTimeout]
  );

  const handleNodeContextMenu: NodeClickHandler = useCallback(
    (event, node) => {
      event.preventDefault();
      const data = node.data as DagNodeData;
      const stepName = data.stepName;

      // Cancel pending single click
      clearNodeTimeout(node.id);

      onRightClick?.(stepName);
    },
    [onRightClick, clearNodeTimeout]
  );

  return {
    handleNodeClick,
    handleNodeDoubleClick,
    handleNodeContextMenu,
  };
}
