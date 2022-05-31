import moment from "moment";
import React from "react";
import {
  Bar,
  BarChart,
  Cell,
  LabelList,
  LabelProps,
  ResponsiveContainer,
  XAxis,
  YAxis,
} from "recharts";
import { statusColorMapping } from "../consts";
import { DAG } from "../models/Dag";
import { SchedulerStatus } from "../models/Status";

type Props = { data: DAG[] };

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
    input.forEach((wf) => {
      const status = wf.Status;
      const start = status?.StartedAt;
      if (start && start != "-") {
        const end = status.FinishedAt;
        let to = now.unix();
        if (end && end != "-") {
          to = moment(end).unix();
        }
        ret.push({
          name: status.Name,
          status: status.Status,
          values: [moment(start).unix(), to],
        });
      }
    });
    setData(ret);
  }, [input]);
  const now = moment();
  return (
    <ResponsiveContainer width="100%" height="90%">
      <BarChart data={data} layout="vertical">
        <XAxis
          name="Time"
          tickFormatter={(unixTime) => moment.unix(unixTime).format("HH:mm")}
          type="number"
          dataKey="values"
          tickCount={96}
          domain={[now.startOf("day").unix(), now.endOf("day").unix()]}
        />
        <YAxis dataKey="name" type="category" hide />
        <Bar background dataKey="values" fill="lightblue" minPointSize={2}>
          {data.map((_, index) => {
            const color = statusColorMapping[data[index].status];
            return <Cell fill={color.backgroundColor} />;
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
  );
}

export default DashboardTimechart;
