import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { useUserPreferences } from '@/contexts/UserPreference';
import { cn, toMermaidNodeId } from '@/lib/utils';
import {
  ArrowDownUp,
  ArrowRightLeft,
  Expand,
  GitGraph,
  Maximize2,
  RotateCcw,
  ZoomIn,
  ZoomOut,
} from 'lucide-react';
import React, { useState } from 'react';
import { components, NodeStatus } from '../../../../api/v1/schema';
import Mermaid from '../../../../ui/Mermaid';

/**
 * Escapes special characters in labels for safe Mermaid syntax interpolation.
 * Prevents parsing errors from quotes, backslashes, or newlines in step names/values.
 */
function escapeMermaidLabel(str: string): string {
  return str
    .replace(/\\/g, '\\\\') // Escape backslashes first
    .replace(/"/g, '\\"') // Escape double quotes
    .replace(/\n/g, '\\n') // Convert newlines to literal \n
    .replace(/\r/g, ''); // Remove carriage returns
}

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
  type: 'status' | 'config';
  /** Direction of the flowchart - TD (top-down) or LR (left-right) */
  flowchart?: FlowchartType;
  /** Callback when flowchart direction changes */
  onChangeFlowchart?: (value: FlowchartType) => void;
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
  /** Whether the graph is currently displayed in an expanded modal view */
  isExpandedView?: boolean;
  /** Custom height for the graph container */
  height?: string | number;
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
function Graph({
  steps,
  flowchart = 'TD',
  onChangeFlowchart,
  type = 'status',
  onClickNode,
  onRightClickNode,
  showIcons = true,
  isExpandedView = false,
  height,
}: Props): React.JSX.Element {
  const [scale, setScale] = useState(isExpandedView ? 0.8 : 1);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const containerRef = React.useRef<HTMLDivElement>(null);
  const { preferences } = useUserPreferences();
  const isDarkMode = preferences.theme !== 'light';

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
    setScale(isExpandedView ? 0.4 : 0.3);
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

  const mermaidStyle: React.CSSProperties = React.useMemo(() => {
    const defaultHeight = '380px';

    function getHeightValue(): string {
      if (isExpandedView) {
        return '100%';
      }
      if (height === undefined) {
        return defaultHeight;
      }
      return typeof height === 'number' ? `${height}px` : height;
    }

    const heightValue = getHeightValue();
    const gridBackground = isDarkMode
      ? `linear-gradient(90deg, rgba(255,255,255,0.05) 1px, transparent 1px),
         linear-gradient(180deg, rgba(255,255,255,0.05) 1px, transparent 1px)`
      : `linear-gradient(90deg, rgba(0,0,0,0.08) 1px, transparent 1px),
         linear-gradient(180deg, rgba(0,0,0,0.08) 1px, transparent 1px)`;

    return {
      display: 'flex',
      alignItems: 'flex-start',
      justifyContent: 'flex-start',
      width: width,
      minWidth: '100%',
      minHeight: heightValue,
      height: heightValue,
      borderRadius: '0.5em',
      background: gridBackground,
      backgroundSize: '20px 20px',
    };
  }, [width, isExpandedView, height, isDarkMode]);

  const graph = React.useMemo(() => {
    if (!steps || steps.length === 0) return '';

    const dat: string[] = [];
    dat.push(`flowchart ${flowchart};`);

    // Add legend comment
    dat.push(
      `%% Shapes: Rectangle=Normal Step, Subprocess=Single Sub DAG, Processes=Parallel Execution`
    );

    // Store the click handler in window for backward compatibility
    // but we'll use double-click for navigation
    if (onClickNode) {
      window.onClickMermaidNode = onClickNode;
    }

    // Track link style indices for individual arrow styling
    let linkIndex = 0;
    const linkStyles: string[] = [];
    // Track node classes for separate application
    const nodeClasses = new Map<string, string>();

    function addNodeFn(
      step: components['schemas']['Step'],
      status: NodeStatus,
      node?: components['schemas']['Node']
    ): void {
      const id = toMermaidNodeId(step.name);
      const c = graphStatusMap[status] || '';

      // Check if this is a sub dagRun node (has a call property)
      const subDAGName = step.call;
      // Check if this is a sub dagRun node (has a 'run' property)
      const isSubDAGRun = !!step.call;
      const hasParallelExecutions = !!step.parallel;
      // Check if this is a router step
      const isRouterStep = step.executorConfig?.type === 'router' || !!step.router;

      // Add indicator for sub dagRun nodes in the label only
      // Escape any special characters in the label to prevent Mermaid parsing errors
      let label = step.name;
      if (isSubDAGRun && subDAGName) {
        if (hasParallelExecutions && node?.subRuns) {
          // Show parallel execution count in the label - avoid brackets in stadium nodes
          label = `${step.name} → ${subDAGName} x${node.subRuns.length}`;
        } else {
          // Single sub DAG run
          label = `${step.name} → ${subDAGName}`;
        }
      }

      // Use different shapes based on node type
      if (isRouterStep) {
        // Diamond shape for router/decision nodes
        // Escape labels to prevent Mermaid parsing errors from special characters
        const routerLabel = step.router?.value
          ? `${escapeMermaidLabel(step.name)}\\n${escapeMermaidLabel(step.router.value)}`
          : escapeMermaidLabel(step.name);
        dat.push(`${id}@{ shape: diamond, label: "${routerLabel}"};`);
        if (c) {
          nodeClasses.set(id, c.replace(':::', ''));
        }
      } else if (isSubDAGRun) {
        // Escape label to prevent Mermaid parsing errors
        const escapedLabel = escapeMermaidLabel(label);
        if (hasParallelExecutions) {
          // Multiple parallel executions - use procs icon
          dat.push(`${id}@{ shape: procs, label: "${escapedLabel}"};`);
        } else {
          // Single sub DAG - use subproc icon
          dat.push(`${id}@{ shape: subproc, label: "${escapedLabel}"};`);
        }
        // Store class for later application (remove ::: prefix)
        if (c) {
          nodeClasses.set(id, c.replace(':::', ''));
        }
      } else {
        // Normal step - use rectangle with inline class syntax
        // Escape label to prevent Mermaid parsing errors
        dat.push(`${id}["${escapeMermaidLabel(label)}"]${c};`);
      }

      // Process dependencies and add connections
      if (step.depends) {
        step.depends.forEach((dep) => {
          const depId = toMermaidNodeId(dep);
          if (status === NodeStatus.Failed) {
            // Dashed line for error state
            dat.push(`${depId} -.- ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#c4726a,stroke-width:1.8px,stroke-dasharray:3`
            );
          } else if (status === NodeStatus.Success) {
            // Solid line with success color
            dat.push(`${depId} --> ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#7da87d,stroke-width:1.8px`
            );
          } else {
            // Default connection style
            dat.push(`${depId} --> ${id};`);
            linkStyles.push(
              `linkStyle ${linkIndex} stroke:#6b635a,stroke-width:1px`
            );
          }
          linkIndex++;
        });
      }

      // We no longer add the standard Mermaid click handler
      // Double-click will be handled by our custom implementation
    }

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

    // Define node styles for different states
    // Use theme-appropriate colors for light/dark modes
    const nodeFill = isDarkMode ? '#161a3d' : '#ffffff'; // --card for dark, white for light
    const nodeColor = isDarkMode ? '#f1f5f9' : '#0f1129'; // --foreground for dark, --background for light
    const strokeDefault = isDarkMode ? '#2d336d' : '#94a3b8';

    // Unified status colors
    const statusColors = {
      none: '#5f6368',      // neutral gray
      running: '#34a853',   // subdued green
      done: '#1e8e3e',      // success green
      error: '#d93025',     // error red
      cancel: '#d946ef',    // pink/magenta for aborted
      skipped: '#5f6368',   // neutral gray
      partial: '#e37400',   // warning amber
      waiting: '#e37400',   // warning amber
      rejected: '#d93025',  // error red
    };

    dat.push(
      `classDef none color:${nodeColor},fill:${nodeFill},stroke:${statusColors.none},stroke-width:2.5px`
    );
    dat.push(
      `classDef running color:${nodeColor},fill:${nodeFill},stroke:${statusColors.running},stroke-width:2.5px`
    );
    dat.push(
      `classDef error color:${nodeColor},fill:${nodeFill},stroke:${statusColors.error},stroke-width:2.5px`
    );
    dat.push(
      `classDef cancel color:${nodeColor},fill:${nodeFill},stroke:${statusColors.cancel},stroke-width:2.5px`
    );
    dat.push(
      `classDef done color:${nodeColor},fill:${nodeFill},stroke:${statusColors.done},stroke-width:2.5px`
    );
    dat.push(
      `classDef skipped color:${nodeColor},fill:${nodeFill},stroke:${statusColors.skipped},stroke-width:2.5px`
    );
    dat.push(
      `classDef partial color:${nodeColor},fill:${nodeFill},stroke:${statusColors.partial},stroke-width:2.5px`
    );
    dat.push(
      `classDef waiting color:${nodeColor},fill:${nodeFill},stroke:${statusColors.waiting},stroke-width:2.5px`
    );
    dat.push(
      `classDef rejected color:${nodeColor},fill:${nodeFill},stroke:${statusColors.rejected},stroke-width:2.5px`
    );

    // Add custom link styles
    dat.push(...linkStyles);

    // Apply classes to nodes that use the new shape syntax (procs/subproc)
    nodeClasses.forEach((className, nodeId) => {
      dat.push(`class ${nodeId} ${className};`);
    });

    return dat.join('\n');
  }, [steps, onClickNode, flowchart, showIcons, isDarkMode]);

  return (
    <div
      className={cn('relative', isExpandedView ? 'h-full flex flex-col' : '')}
      ref={containerRef}
    >
      <div className="absolute right-4 top-2 z-10 bg-card rounded-md shadow-sm border border-border/50">
        <ToggleGroup aria-label="Graph controls">
          {onChangeFlowchart && (
            <>
              <ToggleButton
                value="LR"
                groupValue={flowchart}
                onClick={() => onChangeFlowchart('LR')}
                aria-label="Horizontal layout"
                position="first"
              >
                <ArrowRightLeft className="h-4 w-4" />
              </ToggleButton>
              <ToggleButton
                value="TD"
                groupValue={flowchart}
                onClick={() => onChangeFlowchart('TD')}
                aria-label="Vertical layout"
                position="middle"
              >
                <ArrowDownUp className="h-4 w-4" />
              </ToggleButton>
              <div className="w-px h-6 bg-border mx-1 self-center" />
            </>
          )}

          <ToggleButton
            value="zoomin"
            onClick={() => zoomIn()}
            aria-label="Zoom in"
            position={onChangeFlowchart ? 'middle' : 'first'}
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
            position="middle"
          >
            <RotateCcw className="h-4 w-4" />
          </ToggleButton>

          {!isExpandedView && (
            <>
              <div className="w-px h-6 bg-border mx-1 self-center" />
              <ToggleButton
                value="expand"
                onClick={() => setIsModalOpen(true)}
                aria-label="Expand graph"
                position="last"
              >
                <Expand className="h-4 w-4" />
              </ToggleButton>
            </>
          )}
        </ToggleGroup>
      </div>

      <div
        className={cn(
          'overflow-auto custom-scrollbar',
          isExpandedView
            ? 'flex-1 rounded-lg border border-border/30 bg-muted/5'
            : ''
        )}
      >
        <Mermaid
          style={mermaidStyle}
          def={graph}
          scale={scale}
          onClick={onRightClickNode}
          onDoubleClick={onClickNode}
          onRightClick={onRightClickNode}
        />
      </div>

      {!isExpandedView && (
        <Dialog open={isModalOpen} onOpenChange={setIsModalOpen}>
          <DialogContent className="max-w-[95vw] w-full max-h-[90vh] h-full flex flex-col p-6 overflow-hidden">
            <DialogHeader className="flex-shrink-0 mb-2">
              <DialogTitle className="flex items-center gap-2 text-xl font-semibold">
                <GitGraph className="h-5 w-5 text-primary" />
                Visual Graph
              </DialogTitle>
            </DialogHeader>
            <div className="flex-1 min-h-0 bg-surface rounded-xl p-1 shadow-inner border border-border/20">
              <Graph
                steps={steps}
                flowchart={flowchart}
                onChangeFlowchart={onChangeFlowchart}
                type={type}
                onClickNode={onClickNode}
                onRightClickNode={onRightClickNode}
                showIcons={showIcons}
                isExpandedView={true}
              />
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

/**
 * Calculate the maximum breadth of the graph
 * This helps determine the appropriate width for the graph container
 */
function calculateGraphBreadth(steps: Steps): number {
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

  function calculateLevel(nodeName: string, level = 0): void {
    if (visited.has(nodeName)) return;
    visited.add(nodeName);

    nodeLevels.set(nodeName, Math.max(level, nodeLevels.get(nodeName) || 0));

    // Process children
    const children = parentMap.get(nodeName) || [];
    children.forEach((child) => calculateLevel(child, level + 1));
  }

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
}

export default Graph;

// Map node status to CSS classes for styling
const graphStatusMap: Record<NodeStatus, string> = {
  [NodeStatus.NotStarted]: ':::none',
  [NodeStatus.Running]: ':::running',
  [NodeStatus.Failed]: ':::error',
  [NodeStatus.Aborted]: ':::cancel',
  [NodeStatus.Success]: ':::done',
  [NodeStatus.Skipped]: ':::skipped',
  [NodeStatus.PartialSuccess]: ':::partial',
  [NodeStatus.Waiting]: ':::waiting',
  [NodeStatus.Rejected]: ':::rejected',
};
