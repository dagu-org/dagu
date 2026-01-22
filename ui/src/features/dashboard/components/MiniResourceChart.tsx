import React from 'react';
import {
  Area,
  AreaChart,
  ResponsiveContainer,
  Tooltip,
} from 'recharts';
import { components } from '../../../api/v2/schema';

type MetricPoint = components['schemas']['MetricPoint'];

interface MiniResourceChartProps {
  title: string;
  data: MetricPoint[] | undefined;
  color?: string;
  unit?: string;
  isLoading?: boolean;
  error?: string;
}

const MiniResourceChart: React.FC<MiniResourceChartProps> = ({
  title,
  data,
  unit = '%',
  isLoading,
  error,
}) => {
  // Use primary color with opacity for theme-aware styling
  const strokeColor = 'color-mix(in srgb, var(--primary) 50%, transparent)';
  const fillColor = 'color-mix(in srgb, var(--primary) 15%, transparent)';
  const formattedData =
    data?.map((point) => ({
      time: point.timestamp
        ? new Date(point.timestamp * 1000).toLocaleTimeString()
        : '',
      value: point.value ?? 0,
    })) || [];

  const lastPoint = formattedData[formattedData.length - 1];
  const currentValue = lastPoint ? lastPoint.value : 0;

  if (error) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-muted-foreground">{title}</span>
          <span className="text-sm font-semibold text-muted-foreground">--</span>
        </div>
        <div className="flex-1 flex items-center justify-center text-xs text-muted-foreground">
          Error
        </div>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex flex-col h-full">
        <div className="flex items-center justify-between mb-1">
          <span className="text-xs text-muted-foreground">{title}</span>
          <span className="text-sm font-semibold text-muted-foreground">--</span>
        </div>
        <div className="flex-1 bg-muted animate-pulse rounded" />
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center justify-between mb-1">
        <span className="text-xs text-muted-foreground">{title}</span>
        <span className="text-sm font-light tabular-nums text-foreground">
          {currentValue.toFixed(0)}{unit}
        </span>
      </div>
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart
            data={formattedData}
            margin={{ top: 0, right: 0, left: 0, bottom: 0 }}
          >
            <Tooltip
              contentStyle={{
                backgroundColor: 'var(--background)',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius)',
                fontSize: '10px',
                padding: '4px 8px',
              }}
              itemStyle={{ color: 'var(--foreground)' }}
              labelStyle={{ color: 'var(--muted-foreground)' }}
            />
            <Area
              type="monotone"
              dataKey="value"
              stroke={strokeColor}
              strokeWidth={1}
              fill={fillColor}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
};

export default MiniResourceChart;
