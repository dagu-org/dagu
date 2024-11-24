import React, { useEffect, useRef } from 'react';
import { Box } from '@mui/material';
import moment, { MomentInput } from 'moment-timezone';
import { Timeline, DataSet } from 'vis-timeline/standalone';
import 'vis-timeline/styles/vis-timeline-graph2d.css';
import { statusColorMapping } from '../../consts';
import { DAGStatus } from '../../models';
import { WorkflowListItem } from '../../models/api';
import { useConfig } from '../../contexts/ConfigContext';

type Props = { data: DAGStatus[] | WorkflowListItem[] };

type TimelineItem = {
  id: string;
  content: string;
  start: Date;
  end: Date;
  group: string;
  className: string;
};

function DashboardTimechart({ data: input }: Props) {
  const timelineRef = useRef<HTMLDivElement>(null);
  const timelineInstance = useRef<Timeline | null>(null);
  const config = useConfig();

  useEffect(() => {
    if (!timelineRef.current) return;

    const items: TimelineItem[] = [];
    const now = moment();
    const startOfDay = moment().startOf('day');

    input.forEach((wf) => {
      const status = wf.Status;
      const start = status?.StartedAt;
      if (start && start !== '-') {
        const startMoment = moment(start);
        const end =
          status.FinishedAt && status.FinishedAt !== '-'
            ? moment(status.FinishedAt)
            : now;

        items.push({
          id: status.Name + `_${status.RequestId}`,
          content: status.Name,
          start: startMoment.tz(config.tz).toDate(),
          end: end.tz(config.tz).toDate(),
          group: 'main',
          className: `status-${status.Status}`,
        });
      }
    });

    const dataset = new DataSet(items);

    if (!timelineInstance.current) {
      timelineInstance.current = new Timeline(timelineRef.current, dataset, {
        moment: (date: MomentInput) => moment(date).tz(config.tz),
        start: startOfDay.toDate(),
        end: now.endOf('day').toDate(),
        orientation: 'top',
        stack: true,
        showMajorLabels: true,
        showMinorLabels: true,
        showTooltips: true,
        zoomable: false,
        verticalScroll: true,
        timeAxis: { scale: 'hour', step: 1 },
        format: {
          minorLabels: {
            minute: 'HH:mm',
            hour: 'HH:mm',
          },
          majorLabels: {
            hour: 'ddd D MMMM',
            day: 'ddd D MMMM',
          },
        },
        height: '100%',
        maxHeight: '100%',
        margin: { item: { vertical: 10 } },
      });
    } else {
      timelineInstance.current.setItems(dataset);
    }

    console.log(
      {input, items}
    )

    return () => {
      if (timelineInstance.current) {
        timelineInstance.current.destroy();
        timelineInstance.current = null;
      }
    };
  }, [input]);

  return (
    <TimelineWrapper>
      <div
        ref={timelineRef}
        style={{ width: '100%', height: '100%' }}
      />
      <style>
        {`
        .vis-item .vis-item-overflow {
          overflow: visible;
          color: black;
        }
        .vis-panel.vis-top {
          position: sticky;
          top: 0;
          z-index: 1;
          background-color: white;
        }
        .vis-labelset {
          position: sticky;
          left: 0;
          z-index: 2;
          background-color: white;
        }
        .vis-item .vis-item-content {
          position: absolute;
          left: 100% !important;
          padding-left: 5px;
          transform: translateY(-50%);
          top: 50%;
          white-space: nowrap;
        }
        .vis-item {
          overflow: visible !important;
        }
        `}
      </style>
      <style>{`
        ${Object.entries(statusColorMapping)
          .map(
            ([status, color]) => `
          .status-${status.toLowerCase()} {
            background-color: ${color.backgroundColor};
            color: ${color.color};
            border-color: ${color.backgroundColor};
          }
        `
          )
          .join('\n')}
      `}</style>
    </TimelineWrapper>
  );
}

function TimelineWrapper({ children }: { children: React.ReactNode }) {
  return (
    <Box
      sx={{
        width: '95%',
        maxWidth: '95%',
        height: '60vh',
        overflow: 'auto',
        backgroundColor: 'lightgray',
      }}
    >
      {children}
    </Box>
  );
}

export default DashboardTimechart;