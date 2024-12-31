import React from 'react';
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
  showIcons?: boolean;
  animate?: boolean;
};

declare global {
  interface Window {
    onClickMermaidNode: onClickNode;
  }
}

const Graph: React.FC<Props> = ({
  steps,
  flowchart = 'TD',
  type = 'status',
  onClickNode,
  showIcons = true,
  animate = true,
}) => {
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
    padding: '2em',
    borderRadius: '0.5em',
    background: `
      linear-gradient(90deg, #f8fafc 1px, transparent 1px),
      linear-gradient(180deg, #f8fafc 1px, transparent 1px)
    `,
    backgroundSize: '20px 20px',
  };

  // Define FontAwesome icons for each status with colors and animations
  const statusIcons: { [key: number]: string } = {
    [NodeStatus.None]:
      "<i class='fas fa-circle-notch' style='color: #3b82f6; animation: spin 2s linear infinite;'></i>",
    [NodeStatus.Running]:
      "<i class='fas fa-spinner' style='color: #22c55e; animation: spin 1s linear infinite;'></i>",
    [NodeStatus.Error]:
      "<i class='fas fa-exclamation-circle' style='color: #ef4444'></i>",
    [NodeStatus.Cancel]: "<i class='fas fa-ban' style='color: #ec4899'></i>",
    [NodeStatus.Success]:
      "<i class='fas fa-check-circle' style='color: #16a34a'></i>",
    [NodeStatus.Skipped]:
      "<i class='fas fa-forward' style='color: #64748b'></i>",
  };
  if (!animate) {
    // Remove animations if disabled
    Object.keys(statusIcons).forEach((key: string) => {
      statusIcons[+key] = statusIcons[+key].replace(/animation:.*?;/g, '');
    });
  }

  const graph = React.useMemo(() => {
    if (!steps) return '';

    const dat: string[] = [];
    dat.push(`flowchart ${flowchart};`);

    if (onClickNode) {
      window.onClickMermaidNode = onClickNode;
    }

    // Track link style indices for individual arrow styling
    let linkIndex = 0;
    const linkStyles: string[] = [];

    const addNodeFn = (step: Step, status: NodeStatus) => {
      const id = step.Name.replace(/\s/g, '_');
      const c = graphStatusMap[status] || '';

      // Construct node label with icon if enabled
      const icon = showIcons ? statusIcons[status] || '' : '';
      const label = `${icon} &nbsp; ${step.Name}`;

      // Add node definition
      dat.push(`${id}[${label}]${c};`);

      // Process dependencies and add connections
      if (step.Depends) {
        step.Depends.forEach((d) => {
          const depId = d.replace(/\s/g, '_');
          if (status === NodeStatus.Error) {
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

      // Add click handler if onClickNode is provided
      if (onClickNode) {
        dat.push(`click ${id} onClickMermaidNode`);
      }
    };

    // Process nodes based on type
    if (type === 'status') {
      (steps as Node[]).forEach((s) => addNodeFn(s.Step, s.Status));
    } else {
      (steps as Step[]).forEach((s) => addNodeFn(s, NodeStatus.None));
    }

    // Define node styles for different states with refined colors
    dat.push(
      'classDef none fill:#f0f9ff,stroke:#93c5fd,color:#1e40af,stroke-width:1.2px,white-space:nowrap'
    );
    dat.push(
      'classDef running fill:#f0fdf4,stroke:#86efac,color:#166534,stroke-width:1.2px,white-space:nowrap'
    );
    dat.push(
      'classDef error fill:#fef2f2,stroke:#fca5a5,color:#aa1010,stroke-width:1.2px,white-space:nowrap'
    );
    dat.push(
      'classDef cancel fill:#fdf2f8,stroke:#f9a8d4,color:#9d174d,stroke-width:1.2px,white-space:nowrap'
    );
    dat.push(
      'classDef done fill:#f0fdf4,stroke:#86efac,color:#166534,stroke-width:1.2px,white-space:nowrap'
    );
    dat.push(
      'classDef skipped fill:#f8fafc,stroke:#cbd5e1,color:#475569,stroke-width:1.2px,white-space:nowrap'
    );

    // Add custom link styles
    dat.push(...linkStyles);

    return dat.join('\n');
  }, [steps, onClickNode, flowchart, showIcons]);

  return <Mermaid style={mermaidStyle} def={graph} />;
};

// Function to calculate the maximum breadth of the graph
const calculateGraphBreadth = (steps: Step[] | Node[]) => {
  // Create a map of nodes and their dependencies
  const nodeMap = new Map<string, string[]>();
  const parentMap = new Map<string, string[]>();

  // Initialize maps
  steps.forEach((node) => {
    const step = 'Step' in node ? node.Step : node;
    nodeMap.set(step.Name, step.Depends || []);
    step.Depends?.forEach((dep) => {
      if (!parentMap.has(dep)) {
        parentMap.set(dep, []);
      }
      parentMap.get(dep)?.push(step.Name);
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
    const step = 'Step' in node ? node.Step : node;
    if (!step.Depends || step.Depends.length === 0) {
      calculateLevel(step.Name);
    }
  });

  // Count nodes at each level
  const levelCounts = new Map<number, number>();
  nodeLevels.forEach((level, _) => {
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
  [NodeStatus.None]: ':::none',
  [NodeStatus.Running]: ':::running',
  [NodeStatus.Error]: ':::error',
  [NodeStatus.Cancel]: ':::cancel',
  [NodeStatus.Success]: ':::done',
  [NodeStatus.Skipped]: ':::skipped',
};
