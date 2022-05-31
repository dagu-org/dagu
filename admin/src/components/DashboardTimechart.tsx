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
    const now = moment().unix();
    input.forEach((wf) => {
      const status = wf.Status;
      if (!status || !status.StartedAt || status.StartedAt == "-") {
        return;
      }
      const f = status.FinishedAt;
      const to = !f || f == "-" ? now : moment(f).unix();
      ret.push({
        name: status.Name,
        status: status.Status,
        values: [moment(status.StartedAt).unix(), to],
      });
    });
    setData(ret);
    console.log({ ret });
  }, [input]);
  return (
    <ResponsiveContainer width="100%" height="90%">
      <BarChart data={data} layout="vertical">
        <XAxis
          name="Time"
          tickFormatter={(unixTime) => moment(unixTime).format("HH:mm")}
          type="number"
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
            content={({ x, y, height, value }: LabelProps) => {
              return (
                <text
                  x={10 + Number(x)}
                  y={Number(y) + (Number(height) || 0) / 2}
                  fill="#222"
                  fontSize={12}
                  textAnchor="middle"
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
