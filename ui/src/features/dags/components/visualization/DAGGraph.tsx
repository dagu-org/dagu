/**
 * DAGGraph component provides a tabbed interface for visualizing DAG workflows as either a graph or timeline.
 *
 * @module features/dags/components/visualization
 */
import { Tab, Tabs } from '@/components/ui/tabs';
import { cn } from '@/lib/utils';
import { GanttChart, GitGraph } from 'lucide-react';
import React from 'react';
import { useCookies } from 'react-cookie';
import { components, Status } from '../../../../api/v2/schema';
import BorderedBox from '../../../../ui/BorderedBox';
import { FlowchartSwitch, FlowchartType, Graph, TimelineChart } from './';

/**
 * Props for the DAGGraph component
 */
type Props = {
  /** DAG workflow details containing execution information */
  workflow: components['schemas']['WorkflowDetails'];
  /** Callback for when a step is selected in the graph */
  onSelectStep?: (id: string) => void;
};

/**
 * DAGGraph component provides a tabbed interface for visualizing DAG workflows
 * with options to switch between graph and timeline views
 */
function DAGGraph({ workflow, onSelectStep }: Props) {
  // Active tab state (0 = Graph, 1 = Timeline)
  const [sub, setSub] = React.useState('0');

  // Flowchart direction preference stored in cookies
  const [cookie, setCookie] = useCookies(['flowchart']);
  const [flowchart, setFlowchart] = React.useState(cookie['flowchart']);

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
      <div className="flex justify-between items-start mb-4">
        <Tabs className="w-auto">
          <Tab
            isActive={sub === '0'}
            onClick={() => setSub('0')}
            className={cn(
              'flex items-center gap-2 text-sm h-10 cursor-pointer',
              sub === '0' && 'bg-primary text-primary-foreground font-medium'
            )}
          >
            <GitGraph className="h-4 w-4" />
            Graph
          </Tab>
          <Tab
            isActive={sub === '1'}
            onClick={() => setSub('1')}
            className={cn(
              'flex items-center gap-2 text-sm h-10 cursor-pointer',
              sub === '1' && 'bg-primary text-primary-foreground font-medium'
            )}
          >
            <GanttChart className="h-4 w-4" />
            Timeline
          </Tab>
        </Tabs>

        <FlowchartSwitch value={flowchart} onChange={onChangeFlowchart} />
      </div>

      <BorderedBox className="py-4 px-4 flex flex-col overflow-x-auto">
        <div className="overflow-x-auto">
          {sub === '0' ? (
            <Graph
              steps={workflow.nodes}
              type="status"
              flowchart={flowchart}
              onClickNode={onSelectStep}
              showIcons={workflow.status > Status.NotStarted}
              animate={workflow.status == Status.Running}
            />
          ) : (
            <TimelineChart status={workflow} />
          )}
        </div>
      </BorderedBox>
    </div>
  );
}

export default DAGGraph;
