import { Handle, Position } from '@xyflow/react';
import { cn } from '@/lib/utils';
import { useUserPreferences } from '@/contexts/UserPreference';
import type { DagNodeData } from '../types';
import { getStatusBorderColor } from '../utils/styles';
import { NodeStatus } from '@/api/v2/schema';

interface ParallelNodeProps {
  data: DagNodeData;
  selected?: boolean;
}

/**
 * Parallel execution node - shadow effect (procs shape)
 * Represents multiple parallel sub-DAG executions
 */
export function ParallelNode({ data, selected }: ParallelNodeProps) {
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
          boxShadow: `4px 4px 0 0 ${borderColor}`,
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
