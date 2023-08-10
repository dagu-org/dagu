import moment from 'moment';
import React from 'react';
import { SchedulerStatus, Status } from '../../models';
import Mermaid from '../atoms/Mermaid';

type Props = {
  status: Status;
};

const timeFormat = 'YYYY-MM-DD HH:mm:ss';

function TimelineChart({ status }: Props) {
  if (
    status.Status == SchedulerStatus.None ||
    status.Status == SchedulerStatus.Running
  ) {
    return null;
  }
  const graph = React.useMemo(() => {
    const ret = [
      'gantt',
      'dateFormat YYYY-MM-DD HH:mm:ss',
      'axisFormat %H:%M:%S',
      'todayMarker off',
    ];
    [...status.Nodes]
      .sort((a, b) => {
        return a.StartedAt.localeCompare(b.StartedAt);
      })
      .forEach((step) => {
        if (!step.StartedAt || step.StartedAt == '-') {
          return;
        }
        ret.push(
          step.Step.Name +
            ' : ' +
            moment(step.StartedAt).format(timeFormat) +
            ',' +
            moment(step.FinishedAt).format(timeFormat)
        );
      });
    return ret.join('\n');
  }, [status]);
  return <Mermaid def={graph} />;
}

export default TimelineChart;
