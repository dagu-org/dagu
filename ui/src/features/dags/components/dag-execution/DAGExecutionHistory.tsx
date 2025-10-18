/**
 * Graph component for visualizing DAG dagRuns using Mermaid.js
 *
 * @module features/dags/components/visualization
 */
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { Maximize2, RotateCcw, ZoomIn, ZoomOut } from 'lucide-react';
import React, { useState } from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';
import Mermaid from '../../../../ui/Mermaid';

/** Callback type for node click events */
type onClickNode = (name: string) => void;

/** Callback type for node right-click events */
type onRightClickNode = (name: string) => void;

/** Flowchart direction type - TD (top-down) or LR (left-right) */
export type FlowchartType = 'TD' | 'LR';

/** Steps can be either configuration steps or runtime nodes */
type Steps = components['schemas']['Step'][] | components['schemas']['Node'][];

/** Props for the Graph component */
type Props = {
  /** Type of graph to render - status shows runtime state, config shows definition */
  type?: 'status' | 'config';
  /** Direction of the flowchart - TD (top-down) or LR (left-right) */
  flowchart?: FlowchartType;
  /** Steps or nodes to visualize */
  steps?: Steps;
  /** Callback for node click events (double-click) */
  onClickNode?: onClickNode;
  /** Callback for node right-click events */
  onRightClickNode?: onRightClickNode;
  /** Whether to show status icons */
  showIcons?: boolean;
  /** Whether to animate running nodes */
  animate?: boolean;
};

/** Extend window interface to include the click handler (kept for backward compatibility) */
declare global {
  interface Window {
    onClickMermaidNode: onClickNode;
  }
}

/**
 * Graph component for visualizing DAG dagRuns
 * Renders a Mermaid.js flowchart with nodes and connections
 */
const Graph: React.FC<Props> = ({
  steps,
  flowchart = 'TD',
  type = 'status',
  onClickNode,
  onRightClickNode,
  showIcons = true,
}) => {
  const [scale, setScale] = useState(1);
  const containerRef = React.useRef<HTMLDivElement>(null);

  /** Increase zoom level */
  const zoomIn = () => {
    setScale((prevScale) => Math.min(prevScale + 0.1, 2));
  };

  /** Decrease zoom level */
  const zoomOut = () => {
    setScale((prevScale) => Math.max(prevScale - 0.1, 0.1));
  };

  /** Reset zoom to default */
  const resetZoom = () => {
    setScale(1);
  };

  /** Fit graph to container - zoom out to show entire graph */
  const fitToScreen = () => {
    // Simple approach: set to a small scale that typically shows the full graph
    // This is more reliable than trying to calculate exact dimensions
    setScale(0.3);
  };

  // Calculate width based on flowchart type and graph breadth
  const width = React.useMemo(() => {
    if (!steps) return '100%';

    if (flowchart === 'LR') {
      return `${steps.length * 240}px`;
    } else {
      // For TD layout, calculate based on maximum breadth
      const maxBreadth = calculateGraphBreadth(steps);
      // Assuming each node needs about 200px of width, plus some padding
      return `${Math.max(maxBreadth * 300, 600)}px`;
    }
  }, [steps, flowchart]);

  const mermaidStyle = {
    display: 'flex',
    alignItems: 'flex-center',
    justifyContent: 'flex-start',
    width: width,
    minWidth: '100%',
    minHeight: '200px',
    maxHeight: '300px',
    padding: '2em',
    borderRadius: '0.5em',
    background: `
      linear-gradient(90deg, #f8fafc 1px, transparent 1px),
      linear-gradient(180deg, #f8fafc 1px, transparent 1px)
    `,
    backgroundSize: '20px 20px',
  };

  const graph = React.useMemo(() => {
    if (!steps || steps.length === 0) return '';

    const dat: string[] = [];
    dat.push(`flowchart ${flowchart};`);

    // Add legend comment
    dat.push(
      `%% Shapes: Rectangle=Normal Step, Hexagon=Child DAG, Stadium=Parallel Execution`
    );

    // Store the click handler in window for backward compatibility
    // but we'll use double-click for navigation
    if (onClickNode) {
      window.onClickMermaidNode = onClickNode;
    }

    // Track link style indices for individual arrow styling
    let linkIndex = 0;
    const linkStyles: string[] = [];

    const addNodeFn = (
      step: components['schemas']['Step'],
      status: NodeStatus,
      node?: components['schemas']['Node']
    ) => {
      const id = step.name.replace(/[\s-]/g, 'dagutmp'); // Replace spaces and dashes with 'x'
      const c = graphStatusMap[status] || '';

      // Check if this is a child dagRun node (has a 'run' property)
      const childDAGName = step.call;
      const isChildDAGRun = !!childDAGName;
      const hasParallelExecutions = node?.children && node.children.length > 1;

      // Add indicator for child dagRun nodes in the label only
      // Escape any special characters in the label to prevent Mermaid parsing errors
      let label = step.name;
      if (isChildDAGRun && childDAGName) {
        if (hasParallelExecutions && node?.children) {
          // Show parallel execution count in the label - avoid brackets in stadium nodes
          label = `${step.name} → ${childDAGName} x${node.children.length}`;
        } else {
          // Single child DAG run
          label = `${step.name} → ${childDAGName}`;
        }
      }

      // Use different shape for child dagRuns
      if (isChildDAGRun) {
        dat.push(`${id}{{${label}}}${c};`);
      } else {
        dat.push(`${id}["${label}"]${c};`);
      }

      // Process dependencies and add connections
      if (step.depends) {
        step.depends.forEach((dep) => {
          const depId = dep.replace(/[-\s]/g, 'dagutmp');
          if (status === NodeStatus.Failed) {
            // Dashed line for error state
            dat.push(`${depId} -.- ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#ef4444,stroke-width:1.8px,stroke-dasharray:3`
            );
          } else if (status === NodeStatus.Success) {
            // Solid line with success color
            dat.push(`${depId} --> ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#16a34a,stroke-width:1.8px`
            );
          } else {
            // Default connection style
            dat.push(`${depId} --> ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#64748b,stroke-width:1px`
            );
          }
          linkIndex++;
        });
      }

      // We no longer add the standard Mermaid click handler
      // Double-click will be handled by our custom implementation
    };

    // Process nodes based on type
    if (type === 'status') {
      (steps as components['schemas']['Node'][]).forEach((node) =>
        addNodeFn(node.step, node.status, node)
      );
    } else {
      (steps as components['schemas']['Step'][]).forEach((step) =>
        addNodeFn(step, 0)
      );
    }

    // Define node styles for different states with refined colors
    // Check if dark mode is active
    const isDarkMode = document.documentElement.classList.contains('dark');
    const nodeFill = isDarkMode ? '#18181b' : 'white'; // zinc-900 for dark mode
    const nodeColor = isDarkMode ? '#e4e4e7' : '#333'; // zinc-200 for dark mode text

    dat.push(
      `classDef none color:${nodeColor},fill:${nodeFill},stroke:lightblue,stroke-width:1.2px`
    );
    dat.push(
      `classDef running color:${nodeColor},fill:${nodeFill},stroke:lime,stroke-width:1.2px`
    );
    dat.push(
      `classDef error color:${nodeColor},fill:${nodeFill},stroke:red,stroke-width:1.2px`
    );
    dat.push(
      `classDef cancel color:${nodeColor},fill:${nodeFill},stroke:pink,stroke-width:1.2px`
    );
    dat.push(
      `classDef done color:${nodeColor},fill:${nodeFill},stroke:green,stroke-width:1.2px`
    );
    dat.push(
      `classDef skipped color:${nodeColor},fill:${nodeFill},stroke:gray,stroke-width:1.2px`
    );
    dat.push(
      `classDef partial color:${nodeColor},fill:${nodeFill},stroke:#f59e0b,stroke-width:1.2px`
    );

    // Add custom link styles
    dat.push(...linkStyles);

    return dat.join('\n');
  }, [steps, onClickNode, flowchart, showIcons]);

  return (
    <div className="relative" ref={containerRef}>
      <div className="absolute right-2 top-2 z-10 bg-white dark:bg-zinc-900 rounded-md">
        <ToggleGroup aria-label="Zoom controls">
          <ToggleButton
            value="zoomin"
            onClick={() => zoomIn()}
            aria-label="Zoom in"
            position="first"
          >
            <ZoomIn className="h-4 w-4" />
          </ToggleButton>
          <ToggleButton
            value="zoomout"
            onClick={() => zoomOut()}
            aria-label="Zoom out"
            position="middle"
          >
            <ZoomOut className="h-4 w-4" />
          </ToggleButton>
          <ToggleButton
            value="fit"
            onClick={() => fitToScreen()}
            aria-label="Fit to screen"
            position="middle"
          >
            <Maximize2 className="h-4 w-4" />
          </ToggleButton>
          <ToggleButton
            value="reset"
            onClick={() => resetZoom()}
            aria-label="Reset zoom"
            position="last"
          >
            <RotateCcw className="h-4 w-4" />
          </ToggleButton>
        </ToggleGroup>
      </div>
      <Mermaid
        style={mermaidStyle}
        def={graph}
        scale={scale}
        onDoubleClick={onClickNode}
        onRightClick={onRightClickNode}
      />
    </div>
  );
};

/**
 * Calculate the maximum breadth of the graph
 * This helps determine the appropriate width for the graph container
 */
const calculateGraphBreadth = (steps: Steps) => {
  // Create a map of nodes and their dependencies
  const nodeMap = new Map<string, string[]>();
  const parentMap = new Map<string, string[]>();

  // Initialize maps
  steps.forEach((node) => {
    const step = 'step' in node ? node.step : node;
    nodeMap.set(step.name, step.depends || []);
    step.depends?.forEach((dep) => {
      if (!parentMap.has(dep)) {
        parentMap.set(dep, []);
      }
      parentMap.get(dep)?.push(step.name);
    });
  });

  // Calculate levels for each node
  const nodeLevels = new Map<string, number>();
  const visited = new Set<string>();

  const calculateLevel = (nodeName: string, level = 0) => {
    if (visited.has(nodeName)) return;
    visited.add(nodeName);

    nodeLevels.set(nodeName, Math.max(level, nodeLevels.get(nodeName) || 0));

    // Process children
    const children = parentMap.get(nodeName) || [];
    children.forEach((child) => calculateLevel(child, level + 1));
  };

  // Start from nodes with no dependencies
  steps.forEach((node) => {
    const step = 'step' in node ? node.step : node;
    if (!step.depends || step.depends.length === 0) {
      calculateLevel(step.name);
    }
  });

  // Count nodes at each level
  const levelCounts = new Map<number, number>();
  nodeLevels.forEach((level) => {
    levelCounts.set(level, (levelCounts.get(level) || 0) + 1);
  });

  // Find maximum breadth
  let maxBreadth = 0;
  levelCounts.forEach((count) => {
    maxBreadth = Math.max(maxBreadth, count);
  });

  return maxBreadth;
};

export default Graph;

// Map node status to CSS classes for styling
const graphStatusMap = {
  [0]: ':::none',
  [1]: ':::running',
  [2]: ':::error',
  [3]: ':::cancel',
  [4]: ':::done',
  [5]: ':::skipped',
  [6]: ':::partial',
};
