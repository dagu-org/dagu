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
  color: string;
  unit?: string;
  isLoading?: boolean;
  error?: string;
}

const MiniResourceChart: React.FC<MiniResourceChartProps> = ({
  title,
  data,
  color,
  unit = '%',
  isLoading,
  error,
}) => {
  const formattedData =
    data?.map((point) => ({
      time: point.timestamp
        ? new Date(point.timestamp * 1000).toLocaleTimeString()
        : '',
      value: point.value ?? 0,
    })) || [];

  const lastPoint = formattedData[formattedData.length - 1];
  const currentValue = lastPoint ? lastPoint.value : 0;
  const gradientId = `mini-gradient-${title.replace(/\s+/g, '-')}`;

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
        <span className="text-sm font-semibold" style={{ color }}>
          {currentValue.toFixed(0)}{unit}
        </span>
      </div>
      <div className="flex-1 min-h-0">
        <ResponsiveContainer width="100%" height="100%">
          <AreaChart
            data={formattedData}
            margin={{ top: 0, right: 0, left: 0, bottom: 0 }}
          >
            <defs>
              <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="0%" stopColor={color} stopOpacity={0.3} />
                <stop offset="100%" stopColor={color} stopOpacity={0.05} />
              </linearGradient>
            </defs>
            <Tooltip
              contentStyle={{
                backgroundColor: 'hsl(var(--background))',
                border: '1px solid hsl(var(--border))',
                borderRadius: 'var(--radius)',
                fontSize: '10px',
                padding: '4px 8px',
              }}
              itemStyle={{ color: 'hsl(var(--foreground))' }}
              labelStyle={{ color: 'hsl(var(--muted-foreground))' }}
            />
            <Area
              type="monotone"
              dataKey="value"
              stroke={color}
              strokeWidth={1.5}
              fill={`url(#${gradientId})`}
            />
          </AreaChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
};

export default MiniResourceChart;
