import React, { useEffect, useRef } from 'react';
import { DataSet, Timeline } from 'vis-timeline/standalone';
import 'vis-timeline/styles/vis-timeline-graph2d.css';
import { components } from '../../../api/v2/schema';
import { statusColorMapping } from '../../../consts';
import { useConfig } from '../../../contexts/ConfigContext';
import dayjs from '../../../lib/dayjs';

type Props = { data: components['schemas']['DAGFile'][] };

type TimelineItem = {
  id: string;
  content: string;
  start: Date;
  end: Date;
  group: string;
  className: string;
};

function DashboardTimeChart({ data: input }: Props) {
  const timelineRef = useRef<HTMLDivElement>(null);
  const timelineInstance = useRef<Timeline | null>(null);
  const config = useConfig();

  useEffect(() => {
    if (!timelineRef.current) return;

    const items: TimelineItem[] = [];
    const now = dayjs();
    const startOfDay = dayjs().startOf('day');

    input.forEach((item) => {
      const dag = item.dag;
      const run = item.latestRun;
      const status = run.status;
      const start = run.startedAt;
      if (start && start !== '-') {
        const startMoment = dayjs(start);
        const end = run.finishedAt !== '-' ? dayjs(run.finishedAt) : now;

        items.push({
          id: dag.name + `_${run.requestId}`,
          content: dag.name,
          start: startMoment.tz(config.tz).toDate(),
          end: end.tz(config.tz).toDate(),
          group: 'main',
          className: `status-${status}`,
        });
      }
    });

    const dataset = new DataSet(items);

    if (!timelineInstance.current) {
      // For vis-timeline, we need to use the Timeline constructor with options
      timelineInstance.current = new Timeline(timelineRef.current, dataset, {
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

    return () => {
      if (timelineInstance.current) {
        timelineInstance.current.destroy();
        timelineInstance.current = null;
      }
    };
  }, [input, config.tz]);

  return (
    <TimelineWrapper>
      <div ref={timelineRef} style={{ width: '100%', height: '100%' }} />
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
    <div className="w-[95%] max-w-[95%] h-[60vh] overflow-auto bg-gray-200">
      {children}
    </div>
  );
}

export default DashboardTimeChart;
