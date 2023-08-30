import { Box } from '@mui/material';
import moment from 'moment';
import React from 'react';
import {
  Bar,
  BarChart,
  Cell,
  LabelList,
  LabelProps,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from 'recharts';
import { statusColorMapping } from '../../consts';
import { DAGStatus } from '../../models';
import { SchedulerStatus } from '../../models';
import { WorkflowListItem } from '../../models/api';

type Props = { data: DAGStatus[] | WorkflowListItem[] };

type DataFrame = {
  name: string;
  status: SchedulerStatus;
  values: [number, number];
};

function DashboardTimechart({ data: input }: Props) {
  const [data, setData] = React.useState<DataFrame[]>([]);
  React.useEffect(() => {
    const ret: DataFrame[] = [];
    const now = moment();
    const startOfDayUnix = moment().startOf('day').unix();
    input.forEach((wf) => {
      const status = wf.Status;
      const start = status?.StartedAt;
      if (start && start != '-') {
        const startUnix = Math.max(moment(start).unix(), startOfDayUnix);
        const end = status.FinishedAt;
        let to = now.unix();
        if (end && end != '-') {
          to = moment(end).unix();
        }
        ret.push({
          name: status.Name,
          status: status.Status,
          values: [startUnix, to],
        });
      }
    });
    const sorted = ret.sort((a, b) => {
      return a.values[0] < b.values[0] ? -1 : 1;
    });
    setData(sorted);
  }, [input]);
  const now = moment();
  const shouldScroll = data.length >= 40;
  return (
    <TimelineWrapper shouldScroll={shouldScroll}>
      <ResponsiveContainer
        width="100%"
        minHeight={shouldScroll ? data.length * 12 : undefined}
        height={shouldScroll ? undefined : '90%'}
      >
        <BarChart data={data} layout="vertical">
          <XAxis
            name="Time"
            tickFormatter={(unixTime) => moment.unix(unixTime).format('HH:mm')}
            type="number"
            dataKey="values"
            tickCount={96}
            domain={[now.startOf('day').unix(), now.endOf('day').unix()]}
          />
          <YAxis dataKey="name" type="category" hide />
          <Bar background dataKey="values" fill="lightblue" minPointSize={2}>
            {data.map((_, index) => {
              const color = statusColorMapping[data[index].status];
              return <Cell key={index} fill={color.backgroundColor} />;
            })}
            <LabelList
              dataKey="name"
              position="insideLeft"
              content={({ x, y, width, height, value }: LabelProps) => {
                return (
                  <text
                    x={Number(x) + Number(width) + 2}
                    y={Number(y) + (Number(height) || 0) / 2}
                    fill="#000"
                    fontSize={12}
                    textAnchor="start"
                  >
                    {value}
                  </text>
                );
              }}
            />
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </TimelineWrapper>
  );
}

function TimelineWrapper({
  children,
  shouldScroll,
}: {
  children: React.ReactNode;
  shouldScroll: boolean;
}) {
  if (shouldScroll) {
    return (
      <Box
        sx={{
          width: '100%',
          maxWidth: '100%',
          height: '90%',
          overflow: 'auto',
        }}
      >
        {children}
      </Box>
    );
  }
  return <React.Fragment>{children}</React.Fragment>;
}

export default DashboardTimechart;
