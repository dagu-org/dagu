/**
 * TimelineChart component visualizes the execution timeline of a DAG run using a Gantt chart.
 *
 * @module features/dags/components/visualization
 */
import React from 'react';
import { components, Status } from '../../../../api/v2/schema';
import dayjs from '../../../../lib/dayjs';
import Mermaid from '../../../../ui/Mermaid';

/**
 * Props for the TimelineChart component
 */
type Props = {
  /** DAG run details containing execution information */
  status: components['schemas']['RunDetails'];
};

/** Format for displaying timestamps */
const timeFormat = 'YYYY-MM-DD HH:mm:ss';

/**
 * TimelineChart component renders a Gantt chart showing the execution timeline of DAG steps
 * Only renders for completed DAG runs (not shown for running or not started DAGs)
 */
function TimelineChart({ status }: Props) {
  // Don't render timeline for DAGs that haven't completed yet
  if (status.status == Status.NotStarted || status.status == Status.Running) {
    return null;
  }

  /**
   * Generate the Mermaid Gantt chart definition
   */
  const graph = React.useMemo(() => {
    const ret = [
      'gantt',
      'dateFormat YYYY-MM-DD HH:mm:ss',
      'axisFormat %H:%M:%S',
      'todayMarker off',
    ];

    // Sort nodes by start time and add them to the chart
    [...status.nodes]
      .sort((a, b) => {
        return a.startedAt.localeCompare(b.startedAt);
      })
      .forEach((step) => {
        // Skip steps that haven't started
        if (!step.startedAt || step.startedAt == '-') {
          return;
        }

        // Add step to the Gantt chart with start and end times
        ret.push(
          step.step.name +
            ' : ' +
            dayjs(step.startedAt).format(timeFormat) +
            ',' +
            dayjs(step.finishedAt).format(timeFormat)
        );
      });

    return ret.join('\n');
  }, [status]);

  return <Mermaid def={graph} scale={1.0} />;
}

export default TimelineChart;
