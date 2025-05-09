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

  // Helper function to ensure we have a valid IANA timezone
  const getValidTimezone = React.useCallback((tz: string): string => {
    // If it's already a valid timezone, return it
    try {
      // Test if the timezone is valid
      dayjs().tz(tz);
      return tz;
    } catch {
      // If it's an offset format like UTC+9, convert to a valid IANA timezone
      if (tz.startsWith('UTC+') || tz.startsWith('UTC-')) {
        // Default to a common timezone in that offset
        return (
          'Etc/GMT' + (tz.startsWith('UTC+') ? '-' : '+') + tz.substring(4)
        );
      }
      // Fall back to the browser's timezone
      return dayjs.tz.guess();
    }
  }, []);

  useEffect(() => {
    if (!timelineRef.current) return;

    // Get a valid timezone
    const validTimezone = getValidTimezone(config.tz);

    const items: TimelineItem[] = [];
    const now = dayjs();
    const startOfDay = dayjs().startOf('day');

    input.forEach((item) => {
      const dag = item.dag;
      const workflow = item.latestWorkflow;
      const status = workflow.status;
      const start = workflow.startedAt;
      if (start && start !== '-') {
        const startMoment = dayjs(start);
        const end =
          workflow.finishedAt !== '-' ? dayjs(workflow.finishedAt) : now;

        items.push({
          id: dag.name + `_${workflow.workflowId}`,
          content: dag.name,
          start: startMoment.tz(validTimezone).toDate(),
          end: end.tz(validTimezone).toDate(),
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
  }, [input, config.tz, getValidTimezone]);

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
