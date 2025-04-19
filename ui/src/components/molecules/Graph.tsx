import React, { useState } from 'react';
import Mermaid from '../atoms/Mermaid';
import { ToggleButton, ToggleButtonGroup } from '@mui/material';
import { ZoomIn, ZoomOut, RestartAlt } from '@mui/icons-material';
import { components, NodeStatus } from '../../api/v2/schema';

type onClickNode = (name: string) => void;
export type FlowchartType = 'TD' | 'LR';

type Steps = components['schemas']['Step'][] | components['schemas']['Node'][];

type Props = {
  type: 'status' | 'config';
  flowchart?: FlowchartType;
  steps?: Steps;
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
}) => {
  const [scale, setScale] = useState(1);

  const zoomIn = () => {
    setScale((prevScale) => Math.min(prevScale + 0.1, 2));
  };

  const zoomOut = () => {
    setScale((prevScale) => Math.max(prevScale - 0.1, 0.5));
  };

  const resetZoom = () => {
    setScale(1);
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
    padding: '2em',
    borderRadius: '0.5em',
    background: `
      linear-gradient(90deg, #f8fafc 1px, transparent 1px),
      linear-gradient(180deg, #f8fafc 1px, transparent 1px)
    `,
    backgroundSize: '20px 20px',
  };

  const containerStyle = {
    position: 'relative' as const,
  };

  const toggleButtonStyle = {
    position: 'absolute' as const,
    top: '10px',
    right: '10px',
    zIndex: 1,
  };

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

    const addNodeFn = (
      step: components['schemas']['Step'],
      status: NodeStatus
    ) => {
      const id = step.name.replace(/\s/g, '_');
      const c = graphStatusMap[status] || '';
      const label = `${step.name}`;

      // Add node definition
      dat.push(`${id}[${label}]${c};`);

      // Process dependencies and add connections
      if (step.depends) {
        step.depends.forEach((dep) => {
          const depId = dep.replace(/\s/g, '_');
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

      // Add click handler if onClickNode is provided
      if (onClickNode) {
        dat.push(`click ${id} onClickMermaidNode`);
      }
    };

    // Process nodes based on type
    if (type === 'status') {
      (steps as components['schemas']['Node'][]).forEach((step) =>
        addNodeFn(step.step, step.status)
      );
    } else {
      (steps as components['schemas']['Step'][]).forEach((step) =>
        addNodeFn(step, 0)
      );
    }

    // Define node styles for different states with refined colors
    dat.push(
      'classDef none color:#333,fill:white,stroke:lightblue,stroke-width:1.2px'
    );
    dat.push(
      'classDef running color:#333,fill:white,stroke:lime,stroke-width:1.2px'
    );
    dat.push(
      'classDef error color:#333,fill:white,stroke:red,stroke-width:1.2px'
    );
    dat.push(
      'classDef cancel color:#333,fill:white,stroke:pink,stroke-width:1.2px'
    );
    dat.push(
      'classDef done color:#333,fill:white,stroke:green,stroke-width:1.2px'
    );
    dat.push(
      'classDef skipped color:#333,fill:white,stroke:gray,stroke-width:1.2px'
    );

    // Add custom link styles
    dat.push(...linkStyles);

    return dat.join('\n');
  }, [steps, onClickNode, flowchart, showIcons]);

  return (
    <div style={containerStyle}>
      <ToggleButtonGroup
        size="small"
        sx={{
          ...toggleButtonStyle,
          backgroundColor: 'white',
          '& .MuiToggleButton-root': {
            border: '1px solid rgba(0, 0, 0, 0.12)',
            borderRadius: '4px !important',
            marginRight: '8px',
            padding: '4px 8px',
            color: 'rgba(0, 0, 0, 0.54)',
            '&:hover': {
              backgroundColor: 'rgba(0, 0, 0, 0.04)',
            },
          },
        }}
      >
        <ToggleButton value="zoomin" onClick={zoomIn}>
          <ZoomIn fontSize="small" />
        </ToggleButton>
        <ToggleButton value="zoomout" onClick={zoomOut}>
          <ZoomOut fontSize="small" />
        </ToggleButton>
        <ToggleButton value="reset" onClick={resetZoom}>
          <RestartAlt fontSize="small" />
        </ToggleButton>
      </ToggleButtonGroup>
      <Mermaid style={mermaidStyle} def={graph} scale={scale} />
    </div>
  );
};

// Function to calculate the maximum breadth of the graph
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
};
