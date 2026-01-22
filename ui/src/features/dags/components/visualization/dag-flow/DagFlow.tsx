import { useMemo, useEffect, useCallback } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  BackgroundVariant,
  useReactFlow,
  ReactFlowProvider,
  type Node,
  type Edge,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import { useUserPreferences } from '@/contexts/UserPreference';
import type { DagFlowProps, LayoutDirection } from './types';
import { nodeTypes } from './nodes';
import { edgeTypes } from './edges';
import { transformStepsToGraph } from './utils/transformers';
import { useLayout } from './hooks/useLayout';
import { useNodeInteraction } from './hooks/useNodeInteraction';

interface DagFlowInnerProps extends DagFlowProps {
  isDarkMode: boolean;
}

/**
 * Inner component that uses ReactFlow hooks
 */
function DagFlowInner({
  steps,
  type,
  direction,
  onClickNode,
  onRightClickNode,
  isExpandedView = false,
  isDarkMode,
}: DagFlowInnerProps) {
  const { fitView } = useReactFlow();

  // Transform data to nodes and edges
  const { initialNodes, initialEdges } = useMemo(() => {
    if (!steps || steps.length === 0) {
      return { initialNodes: [], initialEdges: [] };
    }

    const { nodes, edges } = transformStepsToGraph(steps, type);
    return { initialNodes: nodes, initialEdges: edges };
  }, [steps, type]);

  // Apply layout
  const layoutedNodes = useLayout(initialNodes, initialEdges, direction);

  // State management - cast to generic types for xyflow compatibility
  const [nodes, setNodes, onNodesChange] = useNodesState(layoutedNodes as Node[]);
  const [edges, setEdges, onEdgesChange] = useEdgesState(initialEdges as Edge[]);

  // Update nodes/edges when data changes
  useEffect(() => {
    setNodes(layoutedNodes as Node[]);
    setEdges(initialEdges as Edge[]);
  }, [layoutedNodes, initialEdges, setNodes, setEdges]);

  // Fit view when direction changes
  useEffect(() => {
    // Small delay to ensure layout is complete
    const timer = setTimeout(() => {
      fitView({ padding: 0.2, duration: 200 });
    }, 50);
    return () => clearTimeout(timer);
  }, [direction, fitView]);

  // Click handlers
  const { handleNodeClick, handleNodeDoubleClick, handleNodeContextMenu } =
    useNodeInteraction({
      onClick: onRightClickNode, // Single click triggers right-click handler (status modal)
      onDoubleClick: onClickNode, // Double click triggers click handler (navigation)
      onRightClick: onRightClickNode, // Right click triggers right-click handler
    });

  // Prevent default context menu on the canvas
  const onPaneContextMenu = useCallback((event: MouseEvent | React.MouseEvent) => {
    event.preventDefault();
  }, []);

  return (
    <ReactFlow
      nodes={nodes}
      edges={edges}
      onNodesChange={onNodesChange}
      onEdgesChange={onEdgesChange}
      nodeTypes={nodeTypes}
      edgeTypes={edgeTypes as any}
      onNodeClick={handleNodeClick}
      onNodeDoubleClick={handleNodeDoubleClick}
      onNodeContextMenu={handleNodeContextMenu}
      onPaneContextMenu={onPaneContextMenu}
      fitView
      fitViewOptions={{ padding: 0.2 }}
      nodesDraggable={false}
      nodesConnectable={false}
      elementsSelectable={true}
      panOnScroll
      zoomOnScroll
      minZoom={0.1}
      maxZoom={2}
      defaultEdgeOptions={{
        type: 'dependency',
      }}
      proOptions={{ hideAttribution: true }}
    >
      <Background
        variant={BackgroundVariant.Dots}
        gap={20}
        size={1}
        color={isDarkMode ? 'rgba(255,255,255,0.05)' : 'rgba(0,0,0,0.08)'}
      />
      <Controls
        showInteractive={false}
        position="bottom-left"
        className="!shadow-sm !border !border-border/50 !bg-card"
      />
      {isExpandedView && (
        <MiniMap
          position="bottom-right"
          className="!bg-card !border !border-border/50"
          maskColor={isDarkMode ? 'rgba(0,0,0,0.7)' : 'rgba(255,255,255,0.7)'}
          nodeColor={() => {
            // Use a neutral color for minimap nodes
            return isDarkMode ? '#4a5568' : '#a0aec0';
          }}
        />
      )}
    </ReactFlow>
  );
}

/**
 * Main DagFlow component - wraps inner component with ReactFlowProvider
 */
export function DagFlow(props: DagFlowProps) {
  const { preferences } = useUserPreferences();
  const isDarkMode = preferences.theme !== 'light';

  return (
    <div
      className="w-full h-full"
      style={{
        minHeight: props.isExpandedView ? '100%' : '380px',
        height: props.isExpandedView ? '100%' : '380px',
      }}
    >
      <ReactFlowProvider>
        <DagFlowInner {...props} isDarkMode={isDarkMode} />
      </ReactFlowProvider>
    </div>
  );
}

export type { DagFlowProps, LayoutDirection };
