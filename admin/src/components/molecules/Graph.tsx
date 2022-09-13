import React from 'react';
import { Canvas, NodeData, EdgeData } from 'reaflow';
import { Node, NodeStatus } from '../../models';
import { Step } from '../../models';
import Mermaid from '../atoms/Mermaid';

type onClickNode = (name: string) => void;

export type FlowchartType = 'TD' | 'LR';

type Props = {
  type: 'status' | 'config';
  flowchart?: FlowchartType;
  steps?: Step[] | Node[];
  onClickNode?: onClickNode;
};

declare global {
  interface Window {
    onClickMermaidNode: onClickNode;
  }
}

function Graph({
  steps,
  flowchart = 'TD',
  type = 'status',
  onClickNode,
}: Props) {
  const mermaidStyle = {
    display: 'flex',
    alignItems: 'flex-center',
    justifyContent: 'flex-start',
    width: flowchart == 'LR' && steps ? steps.length * 240 + 'px' : '100%',
    minWidth: '100%',
    minHeight: '200px',
    padding: '2em',
    borderRadius: '0.5em',
    backgroundSize: '20px 20px',
  };
  const [nodes, edges] = React.useMemo(() => {
    const nodes: NodeData[] = [];
    const edges: EdgeData[] = [];
    if (!steps) {
      return [nodes, edges];
    }
    const addNodeFn = (step: Step, status: NodeStatus) => {
      nodes.push({
        id: step.Name,
        text: step.Name,
        data: {
          label: step.Name,
          status,
        },
      });
      step.Depends?.forEach((dep) => {
        edges.push({
          id: `${step.Name}-${dep}`,
          from: step.Name,
          to: dep,
        });
      });
    };
    if (type == 'status') {
      (steps as Node[]).forEach((s) => addNodeFn(s.Step, s.Status));
    } else {
      (steps as Step[]).forEach((s) => addNodeFn(s, NodeStatus.None));
    }
    const dat = flowchart == 'TD' ? ['flowchart TD;'] : ['flowchart LR;'];
    if (onClickNode) {
      window.onClickMermaidNode = onClickNode;
    }
    // dat.push(
    //   'linkStyle default stroke:#ddeeff,stroke-width:2px,fill:none,color:#404040'
    // );
    // dat.push('classDef none fill:white,stroke:lightblue,stroke-width:2px');
    // dat.push('classDef running fill:white,stroke:lime,stroke-width:2px');
    // dat.push('classDef error fill:white,stroke:red,stroke-width:2px');
    // dat.push('classDef cancel fill:white,stroke:pink,stroke-width:2px');
    // dat.push('classDef done fill:white,stroke:green,stroke-width:2px');
    // dat.push('classDef skipped fill:white,stroke:gray,stroke-width:2px');
    return [nodes, edges];
  }, [steps, onClickNode, flowchart]);
  return <Canvas nodes={nodes} edges={edges} />;
  // return <Mermaid style={mermaidStyle} def={graph} />;
}

export default Graph;

const graphStatusMap = {
  [NodeStatus.None]: ':::none',
  [NodeStatus.Running]: ':::running',
  [NodeStatus.Error]: ':::error',
  [NodeStatus.Cancel]: ':::cancel',
  [NodeStatus.Success]: ':::done',
  [NodeStatus.Skipped]: ':::skipped',
};
