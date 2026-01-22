import {
  BaseEdge,
  getSmoothStepPath,
} from '@xyflow/react';
import type { DagEdgeData } from '../types';
import { getEdgeStyle } from '../utils/styles';
import { NodeStatus } from '@/api/v2/schema';

interface DependencyEdgeProps {
  id: string;
  sourceX: number;
  sourceY: number;
  targetX: number;
  targetY: number;
  sourcePosition: any;
  targetPosition: any;
  data?: DagEdgeData;
}

/**
 * Custom edge with status-based styling
 * - Failed target: Red dashed line
 * - Success target: Green solid line
 * - Default: Gray solid line
 */
export function DependencyEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
}: DependencyEdgeProps) {
  const [edgePath] = getSmoothStepPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
    borderRadius: 8,
  });

  const targetStatus = data?.targetStatus ?? NodeStatus.NotStarted;
  const style = getEdgeStyle(targetStatus);

  return (
    <BaseEdge
      id={id}
      path={edgePath}
      style={{
        stroke: style.stroke,
        strokeWidth: style.strokeWidth,
        strokeDasharray: style.strokeDasharray,
      }}
    />
  );
}
