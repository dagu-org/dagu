import moment from 'moment';
import React from 'react';
import Mermaid from '../../../ui/Mermaid';
import { components, Status } from '../../../api/v2/schema';

type Props = {
  status: components['schemas']['RunDetails'];
};

const timeFormat = 'YYYY-MM-DD HH:mm:ss';

function TimelineChart({ status }: Props) {
  if (status.status == Status.NotStarted || status.status == Status.Running) {
    return null;
  }
  const graph = React.useMemo(() => {
    const ret = [
      'gantt',
      'dateFormat YYYY-MM-DD HH:mm:ss',
      'axisFormat %H:%M:%S',
      'todayMarker off',
    ];
    [...status.nodes]
      .sort((a, b) => {
        return a.startedAt.localeCompare(b.startedAt);
      })
      .forEach((step) => {
        if (!step.startedAt || step.startedAt == '-') {
          return;
        }
        ret.push(
          step.step.name +
            ' : ' +
            moment(step.startedAt).format(timeFormat) +
            ',' +
            moment(step.finishedAt).format(timeFormat)
        );
      });
    return ret.join('\n');
  }, [status]);
  return <Mermaid def={graph} scale={1.0} />;
}

export default TimelineChart;
