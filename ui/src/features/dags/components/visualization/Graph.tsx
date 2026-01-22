import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ToggleButton, ToggleGroup } from '@/components/ui/toggle-group';
import { cn } from '@/lib/utils';
import {
  ArrowDownUp,
  ArrowRightLeft,
  Expand,
  GitGraph,
} from 'lucide-react';
import React, { useState, useCallback } from 'react';
import { components } from '../../../../api/v2/schema';
import { DagFlow, type LayoutDirection } from './dag-flow';

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
};

/**
 * Graph component for visualizing DAG dagRuns
 * Renders a flowchart with nodes and connections using @xyflow/react
 */
function Graph({
  steps,
  flowchart = 'TD',
  onChangeFlowchart,
  type = 'status',
  onClickNode,
  onRightClickNode,
  isExpandedView = false,
}: Props): React.JSX.Element {
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Convert FlowchartType to LayoutDirection
  const direction: LayoutDirection = flowchart === 'LR' ? 'LR' : 'TB';

  // Handle direction change
  const handleDirectionChange = useCallback(
    (newDirection: LayoutDirection) => {
      const flowchartType: FlowchartType = newDirection === 'LR' ? 'LR' : 'TD';
      onChangeFlowchart?.(flowchartType);
    },
    [onChangeFlowchart]
  );

  // Show empty state if no steps
  if (!steps || steps.length === 0) {
    return (
      <div className="flex items-center justify-center h-[380px] text-muted-foreground">
        No steps to display
      </div>
    );
  }

  return (
    <div
      className={cn('relative', isExpandedView ? 'h-full flex flex-col' : '')}
    >
      {/* Controls toolbar */}
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

          {!isExpandedView && (
            <ToggleButton
              value="expand"
              onClick={() => setIsModalOpen(true)}
              aria-label="Expand graph"
              position={onChangeFlowchart ? 'last' : 'first'}
            >
              <Expand className="h-4 w-4" />
            </ToggleButton>
          )}
        </ToggleGroup>
      </div>

      {/* Graph container */}
      <div
        className={cn(
          'rounded-lg border border-border/30 bg-muted/5',
          isExpandedView ? 'flex-1' : ''
        )}
      >
        <DagFlow
          steps={steps}
          type={type}
          direction={direction}
          onDirectionChange={handleDirectionChange}
          onClickNode={onClickNode}
          onRightClickNode={onRightClickNode}
          isExpandedView={isExpandedView}
        />
      </div>

      {/* Expanded modal */}
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
                isExpandedView={true}
              />
            </div>
          </DialogContent>
        </Dialog>
      )}
    </div>
  );
}

export default Graph;
