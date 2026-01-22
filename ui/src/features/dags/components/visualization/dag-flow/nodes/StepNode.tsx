import { Handle, Position } from '@xyflow/react';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';
import type { DagNodeData } from '../types';
import { getStatusBorderColor } from '../utils/styles';
import { NodeStatus } from '@/api/v2/schema';

interface StepNodeProps {
  data: DagNodeData;
  selected?: boolean;
}

/**
 * Normal step node - rectangular shape with status-based border
 */
export function StepNode({ data, selected }: StepNodeProps) {
  const { preferences } = useUserPreferences();
  const isDarkMode = preferences.theme !== 'light';
  const borderColor = getStatusBorderColor(data.status, isDarkMode);
  const isRunning = data.status === NodeStatus.Running;

  return (
    <>
      <Handle
        type="target"
        position={Position.Top}
        className="!bg-muted-foreground !border-none !w-2 !h-2"
      />
      <div
        className={cn(
          'px-4 py-2 rounded-md text-sm font-medium',
          'bg-card text-foreground',
          'min-w-[120px] text-center',
          'transition-all duration-200',
          'cursor-pointer select-none',
          isRunning && 'animate-pulse',
          selected && 'ring-2 ring-primary ring-offset-2 ring-offset-background'
        )}
        style={{
          borderWidth: '2.5px',
          borderStyle: 'solid',
          borderColor,
        }}
      >
        {data.label}
      </div>
      <Handle
        type="source"
        position={Position.Bottom}
        className="!bg-muted-foreground !border-none !w-2 !h-2"
      />
    </>
  );
}
