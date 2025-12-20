import React, { useEffect, useRef, useState } from 'react';
import { Timeline } from 'vis-timeline/standalone';
import { DataSet } from 'vis-data';
import 'vis-timeline/styles/vis-timeline-graph2d.css';
import { components } from '../../../api/v2/schema';
import { statusColorMapping } from '../../../consts';
import { useConfig } from '../../../contexts/ConfigContext';
import dayjs from '../../../lib/dayjs';
import DAGRunDetailsModal from '../../dag-runs/components/dag-run-details/DAGRunDetailsModal';
import { Button } from '@/components/ui/button';
import { ZoomIn, ZoomOut, Maximize, Clock, RotateCcw } from 'lucide-react';

type Props = {
  data: components['schemas']['DAGRunSummary'][];
  selectedDate?: {
    startTimestamp: number;
    endTimestamp?: number;
  };
};

type TimelineItem = {
  id: string;
  content: string;
  start: Date;
  end: Date;
  group: string;
  className: string;
};

function DashboardTimeChart({ data: input, selectedDate }: Props) {
  const timelineRef = useRef<HTMLDivElement>(null);
  const timelineInstance = useRef<Timeline | null>(null);
  const config = useConfig();
  const [selectedDAGRun, setSelectedDAGRun] = useState<{
    name: string;
    dagRunId: string;
  } | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Helper function to ensure we have a valid IANA timezone
  const getValidTimezone = React.useCallback((tz: string): string => {
    try {
      dayjs().tz(tz);
      return tz;
    } catch {
      if (tz.startsWith('UTC+') || tz.startsWith('UTC-')) {
        return (
          'Etc/GMT' + (tz.startsWith('UTC+') ? '-' : '+') + tz.substring(4)
        );
      }
      return dayjs.tz.guess();
    }
  }, []);

  // Function to determine appropriate time scale based on visible range
  const updateTimeAxisBasedOnZoom = React.useCallback((timeline: Timeline) => {
    try {
      const range = timeline.getWindow();
      const rangeInMs = range.end.getTime() - range.start.getTime();
      const rangeInMinutes = rangeInMs / (1000 * 60);
      const rangeInHours = rangeInMinutes / 60;
      const rangeInDays = rangeInHours / 24;

      let options = {};

      if (rangeInMinutes < 30) {
        // Less than 30 minutes - show 1-minute intervals
        options = {
          timeAxis: { scale: 'minute', step: 1 },
          format: {
            minorLabels: {
              second: 's',
              minute: 'HH:mm:ss',
            },
            majorLabels: {
              minute: 'HH:mm',
              hour: 'ddd D MMM HH:mm',
            },
          },
        };
      } else if (rangeInMinutes < 120) {
        // 30 minutes to 2 hours - show 5-minute intervals
        options = {
          timeAxis: { scale: 'minute', step: 5 },
          format: {
            minorLabels: {
              minute: 'HH:mm',
              hour: 'HH:mm',
            },
            majorLabels: {
              hour: 'ddd D MMM',
              day: 'ddd D MMM',
            },
          },
        };
      } else if (rangeInHours < 6) {
        // 2-6 hours - show 15-minute intervals
        options = {
          timeAxis: { scale: 'minute', step: 15 },
          format: {
            minorLabels: {
              minute: 'HH:mm',
              hour: 'HH:mm',
            },
            majorLabels: {
              hour: 'ddd D MMM',
              day: 'ddd D MMM',
            },
          },
        };
      } else if (rangeInHours < 24) {
        // 6-24 hours - show hourly
        options = {
          timeAxis: { scale: 'hour', step: 1 },
          format: {
            minorLabels: {
              hour: 'HH:mm',
            },
            majorLabels: {
              day: 'ddd D MMM',
            },
          },
        };
      } else if (rangeInDays < 3) {
        // 1-3 days - show 2-hour intervals
        options = {
          timeAxis: { scale: 'hour', step: 2 },
          format: {
            minorLabels: {
              hour: 'HH:mm',
              day: 'D',
            },
            majorLabels: {
              day: 'ddd D MMM',
              week: 'MMM YYYY',
            },
          },
        };
      } else if (rangeInDays < 7) {
        // 3-7 days - show 4-hour intervals
        options = {
          timeAxis: { scale: 'hour', step: 4 },
          format: {
            minorLabels: {
              hour: 'HH:mm',
              day: 'D',
            },
            majorLabels: {
              day: 'ddd D MMM',
              week: 'MMM YYYY',
            },
          },
        };
      } else if (rangeInDays < 30) {
        // 7-30 days - show daily
        options = {
          timeAxis: { scale: 'day', step: 1 },
          format: {
            minorLabels: {
              day: 'D',
              weekday: 'ddd',
            },
            majorLabels: {
              week: 'W',
              month: 'MMM YYYY',
            },
          },
        };
      } else if (rangeInDays < 90) {
        // 30-90 days - show 2-day intervals
        options = {
          timeAxis: { scale: 'day', step: 2 },
          format: {
            minorLabels: {
              day: 'D',
              week: 'W',
            },
            majorLabels: {
              month: 'MMM YYYY',
            },
          },
        };
      } else if (rangeInDays < 365) {
        // 90-365 days - show weekly
        options = {
          timeAxis: { scale: 'week', step: 1 },
          format: {
            minorLabels: {
              week: 'W',
              month: 'MMM',
            },
            majorLabels: {
              month: 'MMM YYYY',
              year: 'YYYY',
            },
          },
        };
      } else {
        // More than 365 days - show monthly
        options = {
          timeAxis: { scale: 'month', step: 1 },
          format: {
            minorLabels: {
              month: 'MMM',
            },
            majorLabels: {
              year: 'YYYY',
            },
          },
        };
      }

      timeline.setOptions(options);
    } catch (error) {
      console.warn('Error updating time axis:', error);
    }
  }, []);

  useEffect(() => {
    if (!timelineRef.current) return;

    const validTimezone = getValidTimezone(config.tz);
    const items: TimelineItem[] = [];
    const now = dayjs();

    const viewStartDate = selectedDate
      ? dayjs.unix(selectedDate.startTimestamp)
      : dayjs().startOf('day');

    const viewEndDate = selectedDate?.endTimestamp
      ? dayjs.unix(selectedDate.endTimestamp)
      : now.endOf('day');

    if (!timelineInstance.current) {
      initialViewRef.current = {
        start: !isNaN(viewStartDate.toDate().getTime())
          ? viewStartDate.toDate()
          : dayjs().startOf('day').toDate(),
        end: !isNaN(viewEndDate.toDate().getTime())
          ? viewEndDate.toDate()
          : dayjs().endOf('day').toDate(),
      };
    }

    const seenIds = new Set<string>();

    input.forEach((dagRun) => {
      const status = dagRun.status;
      const start = dagRun.startedAt;
      if (start && start !== '-') {
        const startMoment = dayjs(start);
        const end = dagRun.finishedAt !== '-' ? dayjs(dagRun.finishedAt) : now;

        const startDate = startMoment.tz(validTimezone).toDate();
        const endDate = end.tz(validTimezone).toDate();

        const id = dagRun.name + `_${dagRun.dagRunId}`;
        if (seenIds.has(id)) return; // Skip duplicates
        seenIds.add(id);

        if (
          !isNaN(startDate.getTime()) &&
          !isNaN(endDate.getTime()) &&
          startDate <= endDate
        ) {
          items.push({
            id,
            content: dagRun.name,
            start: startDate,
            end: endDate,
            group: 'main',
            className: `status-${status}`,
          });
        }
      }
    });

    const dataset = new DataSet(items);

    const validViewStartDate = !isNaN(viewStartDate.toDate().getTime())
      ? viewStartDate.toDate()
      : dayjs().startOf('day').toDate();
    const validViewEndDate = !isNaN(viewEndDate.toDate().getTime())
      ? viewEndDate.toDate()
      : dayjs().endOf('day').toDate();

    if (!timelineInstance.current) {
      timelineInstance.current = new Timeline(timelineRef.current, dataset, {
        start: validViewStartDate,
        end: validViewEndDate,
        orientation: 'top',
        stack: true,
        showMajorLabels: true,
        showMinorLabels: true,
        showTooltips: true,
        zoomable: true,
        verticalScroll: true,
        zoomKey: 'ctrlKey',
        timeAxis: { scale: 'hour', step: 1 },
        format: {
          minorLabels: {
            minute: 'HH:mm',
            hour: 'HH:mm',
          },
          majorLabels: {
            hour: 'ddd D MMM',
            day: 'ddd D MMM',
          },
        },
        height: '100%',
        maxHeight: '100%',
        margin: {
          item: { vertical: 4, horizontal: 2 },
          axis: 2,
        },
      });

      // Add range change listener for dynamic time axis
      timelineInstance.current.on('rangechanged', () => {
        if (timelineInstance.current) {
          updateTimeAxisBasedOnZoom(timelineInstance.current);
        }
      });

      // Initial update based on current view
      updateTimeAxisBasedOnZoom(timelineInstance.current);
    } else {
      timelineInstance.current.setItems(dataset);
      timelineInstance.current.setWindow(validViewStartDate, validViewEndDate);
    }

    return () => {
      if (timelineInstance.current) {
        timelineInstance.current.off('rangechanged');
        timelineInstance.current.destroy();
        timelineInstance.current = null;
      }
    };
  }, [
    input,
    config.tz,
    getValidTimezone,
    selectedDate,
    updateTimeAxisBasedOnZoom,
  ]);

  useEffect(() => {
    const timeline = timelineInstance.current;
    if (timeline) {
      timeline.off('click');

      timeline.on('click', (properties) => {
        if (properties.item) {
          const itemId = properties.item.toString();

          const matchingDAGRun = input.find(
            (dagRun) => itemId === dagRun.name + `_${dagRun.dagRunId}`
          );

          if (matchingDAGRun) {
            setSelectedDAGRun({
              name: matchingDAGRun.name,
              dagRunId: matchingDAGRun.dagRunId,
            });
            setIsModalOpen(true);
          }
        }
      });
    }

    return () => {
      if (timeline) {
        timeline.off('click');
      }
    };
  }, [input]);

  const handleCloseModal = () => {
    setIsModalOpen(false);
  };

  const initialViewRef = useRef<{ start: Date; end: Date } | null>(null);

  const handleZoomIn = () => {
    if (timelineInstance.current) {
      timelineInstance.current.zoomIn(0.5);
    }
  };

  const handleZoomOut = () => {
    if (timelineInstance.current) {
      timelineInstance.current.zoomOut(0.5);
    }
  };

  const handleFit = () => {
    if (timelineInstance.current) {
      try {
        timelineInstance.current.fit();
      } catch {
        try {
          timelineInstance.current.fit();
        } catch (fitError) {
          console.warn('Timeline fit failed:', fitError);
        }
      }
    }
  };

  const handleCurrent = () => {
    if (timelineInstance.current) {
      const now = dayjs();
      timelineInstance.current.setWindow(
        now.subtract(1, 'hour').toDate(),
        now.add(1, 'hour').toDate()
      );
    }
  };

  const handleReset = () => {
    if (timelineInstance.current && initialViewRef.current) {
      timelineInstance.current.setWindow(
        initialViewRef.current.start,
        initialViewRef.current.end
      );
    }
  };

  return (
    <TimelineWrapper>
      <div className="flex justify-between items-center gap-2 px-3 py-2 border-b border-border bg-card flex-shrink-0">
        <span className="text-xs font-medium text-muted-foreground">Timeline</span>
        <div className="flex gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={handleCurrent}
            title="Go to current time"
          >
            <Clock className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleFit}
            title="Fit all items in view"
          >
            <Maximize className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleZoomIn}
            title="Zoom in"
          >
            <ZoomIn className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleZoomOut}
            title="Zoom out"
          >
            <ZoomOut className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={handleReset}
            title="Reset view to initial state"
          >
            <RotateCcw className="h-4 w-4" />
          </Button>
        </div>
      </div>
      <div ref={timelineRef} className="flex-1 min-h-0 overflow-auto" />
      {selectedDAGRun && (
        <DAGRunDetailsModal
          name={selectedDAGRun.name}
          dagRunId={selectedDAGRun.dagRunId}
          isOpen={isModalOpen}
          onClose={handleCloseModal}
        />
      )}
      <style>
        {`
        .vis-timeline {
          font-family: inherit !important;
          font-size: 12px !important;
          background-color: var(--card) !important;
          border: none !important;
          border-radius: 0 !important;
        }
        .vis-timeline .vis-panel {
          border: none !important;
        }
        .vis-item .vis-item-overflow {
          overflow: visible;
          color: var(--foreground);
        }
        .vis-panel.vis-top {
          position: sticky;
          top: 0;
          z-index: 1;
          background-color: var(--muted) !important;
        }
        .vis-labelset {
          position: sticky;
          left: 0;
          z-index: 2;
          background-color: var(--card) !important;
        }
        .vis-foreground {
          background-color: transparent !important;
        }
        .vis-background {
          background-color: var(--card) !important;
        }
        .vis-center {
          background-color: var(--card) !important;
        }
        .vis-left {
          background-color: var(--card) !important;
        }
        .vis-right {
          background-color: var(--card) !important;
        }
        .vis-top {
          background-color: var(--muted) !important;
        }
        .vis-bottom {
          background-color: var(--card) !important;
        }
        .vis-time-axis {
          background-color: var(--muted) !important;
          color: var(--foreground) !important;
        }
        .vis-time-axis .vis-text {
          font-size: 11px !important;
          color: var(--muted-foreground) !important;
          font-family: inherit !important;
        }
        .vis-time-axis .vis-text.vis-major {
          font-size: 11px !important;
          font-weight: 600;
          color: var(--foreground) !important;
        }
        .vis-time-axis .vis-text.vis-minor {
          font-size: 10px !important;
          color: var(--muted-foreground) !important;
        }
        .vis-time-axis .vis-grid.vis-minor {
          border-color: var(--border) !important;
          opacity: 0.3;
        }
        .vis-time-axis .vis-grid.vis-major {
          border-color: var(--border) !important;
          opacity: 0.6;
        }
        .vis-item .vis-item-content {
          position: absolute;
          left: 100% !important;
          padding-left: 6px;
          transform: translateY(-50%);
          top: 50%;
          white-space: nowrap;
          font-size: 11px !important;
          font-weight: 500;
          color: var(--foreground) !important;
          text-shadow: 0 0 2px var(--card);
        }
        .vis-item {
          overflow: visible !important;
          height: 20px !important;
          border-radius: 3px !important;
          border-width: 1px !important;
          cursor: pointer !important;
          transition: opacity 0.15s ease !important;
        }
        .vis-item:hover {
          opacity: 0.85 !important;
        }
        .vis-panel {
          background-color: var(--card) !important;
        }
        .vis-item.vis-selected {
          border-color: var(--ring) !important;
          border-width: 2px !important;
        }
        .vis-current-time {
          background-color: var(--primary) !important;
          width: 2px !important;
        }
        .vis-custom-time {
          background-color: var(--primary) !important;
        }
        /* Scrollbar styling */
        .vis-timeline::-webkit-scrollbar {
          width: 8px;
          height: 8px;
        }
        .vis-timeline::-webkit-scrollbar-track {
          background: var(--muted);
          border-radius: 4px;
        }
        .vis-timeline::-webkit-scrollbar-thumb {
          background: var(--border);
          border-radius: 4px;
        }
        .vis-timeline::-webkit-scrollbar-thumb:hover {
          background: var(--muted-foreground);
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
    <div className="w-full h-full flex flex-col bg-card overflow-hidden">
      {children}
    </div>
  );
}

export default DashboardTimeChart;
