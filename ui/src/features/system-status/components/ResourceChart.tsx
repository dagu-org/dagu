import React from 'react';
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { components } from '../../../api/v2/schema';
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from '../../../components/ui/card';

type MetricPoint = components['schemas']['MetricPoint'];

interface ResourceChartProps {
  title: string;
  data: MetricPoint[] | undefined;
  color: string;
  unit?: string;
  isLoading?: boolean;
  error?: string;
}

const ResourceChart: React.FC<ResourceChartProps> = ({
  title,
  data,
  color,
  unit = '%',
  isLoading,
  error,
}) => {
  if (error) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex h-[200px] items-center justify-center text-sm text-muted-foreground">
            Failed to load data
          </div>
        </CardContent>
      </Card>
    );
  }

  if (isLoading) {
    return (
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
          <CardTitle className="text-sm font-medium">{title}</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="h-[200px] w-full bg-muted animate-pulse rounded-md" />
        </CardContent>
      </Card>
    );
  }

  const formattedData =
    data?.map((point) => ({
      time: point.timestamp
        ? new Date(point.timestamp * 1000).toLocaleTimeString()
        : '',
      value: point.value ?? 0,
    })) || [];

  const lastPoint = formattedData[formattedData.length - 1];
  const currentValue = lastPoint ? lastPoint.value : 0;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-sm font-medium">{title}</CardTitle>
        <div className="text-2xl font-bold">
          {currentValue.toFixed(1)}
          {unit}
        </div>
      </CardHeader>
      <CardContent>
        <div className="h-[200px] w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart
              data={formattedData}
              margin={{
                top: 5,
                right: 0,
                left: 0,
                bottom: 0,
              }}
            >
              <defs>
                <linearGradient
                  id={`color-${title}`}
                  x1="0"
                  y1="0"
                  x2="0"
                  y2="1"
                >
                  <stop offset="5%" stopColor={color} stopOpacity={0.3} />
                  <stop offset="95%" stopColor={color} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid
                strokeDasharray="3 3"
                vertical={false}
                stroke="hsl(var(--border))"
              />
              <XAxis dataKey="time" hide />
              <YAxis hide domain={[0, 'auto']} />
              <Tooltip
                contentStyle={{
                  backgroundColor: 'hsl(var(--background))',
                  border: '1px solid hsl(var(--border))',
                  borderRadius: 'var(--radius)',
                }}
                itemStyle={{ color: 'hsl(var(--foreground))' }}
                labelStyle={{ color: 'hsl(var(--muted-foreground))' }}
              />
              <Area
                type="monotone"
                dataKey="value"
                stroke={color}
                fillOpacity={1}
                fill={`url(#color-${title})`}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </CardContent>
    </Card>
  );
};

export default ResourceChart;
