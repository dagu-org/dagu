/**
 * DAGGraph component provides a tabbed interface for visualizing DAG dagRuns as either a graph or timeline.
 *
 * @module features/dags/components/visualization
 */
import { Tab, Tabs } from '@/components/ui/tabs';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import {
  GanttChart,
  GitGraph,
  GripHorizontal,
  MousePointerClick,
} from 'lucide-react';
import React from 'react';
import { useCookies } from 'react-cookie';
import { components, Status } from '../../../../api/v1/schema';
import { useConfig } from '../../../../contexts/ConfigContext';
import BorderedBox from '../../../../ui/BorderedBox';
import { FlowchartType, Graph, TimelineChart } from './';

/**
 * Props for the DAGGraph component
 */
type Props = {
  /** DAG dagRun details containing execution information */
  dagRun: components['schemas']['DAGRunDetails'];
  /** Callback for when a step is selected in the graph (double-click) */
  onSelectStep?: (id: string) => void;
  /** Callback for when a step is right-clicked in the graph */
  onRightClickStep?: (id: string) => void;
};

/**
 * DAGGraph component provides a tabbed interface for visualizing DAG dagRuns
 * with options to switch between graph and timeline views
 */
function DAGGraph({ dagRun, onSelectStep, onRightClickStep }: Props) {
  // Active tab state (0 = Graph, 1 = Timeline)
  const [sub, setSub] = React.useState('0');
  const config = useConfig();

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);
  const [graphHeight, setGraphHeight] = React.useState(380);

  const handleResizeMouseDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const startY = e.clientY;
    const startHeight = graphHeight;

    const handleMouseMove = (mv: MouseEvent) => {
      const newHeight = startHeight + (mv.clientY - startY);
      setGraphHeight(Math.max(200, newHeight));
    };

    const handleMouseUp = () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };

    document.addEventListener('mousemove', handleMouseMove);
    document.addEventListener('mouseup', handleMouseUp);
  };

  /**
   * Handle flowchart direction change and save preference to cookie
   */
  const onChangeFlowchart = (value: FlowchartType) => {
    if (!value) {
      return;
    }
    setCookie('flowchart', value, { path: '/' });
    setFlowchart(value);
  };

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-start mb-4 gap-2">
        <Tabs className="w-auto self-center sm:self-auto">
          <Tab
            isActive={sub === '0'}
            onClick={() => setSub('0')}
            className="flex items-center gap-2 cursor-pointer"
          >
            <GitGraph className="h-4 w-4" />
            Graph
          </Tab>
          <Tab
            isActive={sub === '1'}
            onClick={() => setSub('1')}
            className="flex items-center gap-2 cursor-pointer"
          >
            <GanttChart className="h-4 w-4" />
            Timeline
          </Tab>
        </Tabs>

        <div className="self-center sm:self-auto"></div>
      </div>

      <BorderedBox className="pt-4 px-4 pb-0 flex flex-col items-stretch overflow-hidden">
        {sub === '0' && (
          <div className="flex justify-end mb-2">
            <Tooltip>
              <TooltipTrigger asChild>
                <div className="flex items-center text-xs text-muted-foreground bg-muted px-2 py-1 rounded cursor-help">
                  <MousePointerClick className="h-3 w-3 mr-1" />
                  {config.permissions.runDags
                    ? 'Double-click to navigate / Right-click to change status'
                    : 'Double-click to navigate'}
                </div>
              </TooltipTrigger>
              <TooltipContent>
                <div className="space-y-1">
                  <p>Double-click: Navigate to sub dagRun</p>
                  {config.permissions.runDags && (
                    <p>Right-click: Update node status</p>
                  )}
                </div>
              </TooltipContent>
            </Tooltip>
          </div>
        )}
        <div className="overflow-x-auto -mx-4 px-4">
          {sub === '0' ? (
            <Graph
              steps={dagRun.nodes}
              type="status"
              flowchart={flowchart}
              onChangeFlowchart={onChangeFlowchart}
              onClickNode={onSelectStep}
              onRightClickNode={
                config.permissions.runDags ? onRightClickStep : undefined
              }
              showIcons={dagRun.status > Status.NotStarted}
              animate={dagRun.status == Status.Running}
              height={graphHeight}
            />
          ) : (
            <TimelineChart status={dagRun} />
          )}
        </div>
        {sub === '0' && (
          <div
            className="flex justify-center items-center py-2 cursor-row-resize hover:bg-muted/50 transition-colors w-full select-none"
            onMouseDown={handleResizeMouseDown}
          >
            <GripHorizontal className="h-4 w-4 text-muted-foreground/50" />
          </div>
        )}
        {sub === '1' && <div className="pb-4" />}
      </BorderedBox>
    </div>
  );
}

export default DAGGraph;
