import React, { useEffect, useRef, useState } from 'react';
import { DataSet, Timeline } from 'vis-timeline/standalone';
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

    // Use selected date for timeline view range if provided, otherwise use today
    const viewStartDate = selectedDate
      ? dayjs.unix(selectedDate.startTimestamp)
      : dayjs().startOf('day');

    const viewEndDate = selectedDate?.endTimestamp
      ? dayjs.unix(selectedDate.endTimestamp)
      : now.endOf('day');

    // Store the initial view range for reset functionality
    if (!timelineInstance.current) {
      // Store initial view range in a ref with validated dates
      initialViewRef.current = {
        start: !isNaN(viewStartDate.toDate().getTime()) ? viewStartDate.toDate() : dayjs().startOf('day').toDate(),
        end: !isNaN(viewEndDate.toDate().getTime()) ? viewEndDate.toDate() : dayjs().endOf('day').toDate(),
      };
    }

    input.forEach((dagRun) => {
      const status = dagRun.status;
      const start = dagRun.startedAt;
      if (start && start !== '-') {
        const startMoment = dayjs(start);
        const end =
          dagRun.finishedAt !== '-' ? dayjs(dagRun.finishedAt) : now;

        // Validate that we have valid dates before adding to timeline
        const startDate = startMoment.tz(validTimezone).toDate();
        const endDate = end.tz(validTimezone).toDate();
        
        if (!isNaN(startDate.getTime()) && !isNaN(endDate.getTime()) && startDate <= endDate) {
          items.push({
            id: dagRun.name + `_${dagRun.dagRunId}`,
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

    // Validate view dates before using them
    const validViewStartDate = !isNaN(viewStartDate.toDate().getTime()) ? viewStartDate.toDate() : dayjs().startOf('day').toDate();
    const validViewEndDate = !isNaN(viewEndDate.toDate().getTime()) ? viewEndDate.toDate() : dayjs().endOf('day').toDate();

    if (!timelineInstance.current) {
      // For vis-timeline, we need to use the Timeline constructor with options
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
          axis: 2
        },
      });
    } else {
      timelineInstance.current.setItems(dataset);
      // Update the timeline window when selectedDate changes
      timelineInstance.current.setWindow(
        validViewStartDate,
        validViewEndDate
      );
    }

    return () => {
      if (timelineInstance.current) {
        timelineInstance.current.destroy();
        timelineInstance.current = null;
      }
    };
  }, [input, config.tz, getValidTimezone, selectedDate]);

  // Add click event handler whenever the timeline instance is created or updated
  useEffect(() => {
    const timeline = timelineInstance.current;
    if (timeline) {
      // Remove any existing click handlers to avoid duplicates
      timeline.off('click');

      // Add the click handler
      timeline.on('click', (properties) => {
        if (properties.item) {
          const itemId = properties.item.toString();

          // Find the original dagRun item that matches this ID
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
      // Clean up the event handler when the component unmounts or timeline changes
      if (timeline) {
        timeline.off('click');
      }
    };
  }, [input]); // Re-run when input data changes, as that's when timeline might be recreated

  // Handle modal close
  const handleCloseModal = () => {
    setIsModalOpen(false);
  };

  // Reference to store initial view range for reset functionality
  const initialViewRef = useRef<{ start: Date; end: Date } | null>(null);

  // Timeline navigation handlers
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
      // Check if timeline has valid items before calling fit
      try {
        // Use the itemsData property instead of getItems() method
        const dataset = timelineInstance.current.itemsData;
        if (dataset && dataset.length > 0) {
          timelineInstance.current.fit();
        }
      } catch (error) {
        // If there's an error accessing items, just try to fit anyway
        // but wrap in try-catch to prevent crashes
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
      // Move to current time with a 2-hour window
      timelineInstance.current.setWindow(
        now.subtract(1, 'hour').toDate(),
        now.add(1, 'hour').toDate()
      );
    }
  };

  const handleReset = () => {
    if (timelineInstance.current && initialViewRef.current) {
      // Reset to the initial view range
      timelineInstance.current.setWindow(
        initialViewRef.current.start,
        initialViewRef.current.end
      );
    }
  };

  return (
    <TimelineWrapper>
      <div className="flex justify-end gap-1 p-2 border-b bg-muted/30">
        <Button
          variant="ghost"
          size="sm"
          onClick={handleCurrent}
          title="Go to current time"
          className="h-6 px-2 text-xs"
        >
          <Clock className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleFit}
          title="Fit all items in view"
          className="h-6 px-2 text-xs"
        >
          <Maximize className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleZoomIn}
          title="Zoom in"
          className="h-6 px-2 text-xs"
        >
          <ZoomIn className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleZoomOut}
          title="Zoom out"
          className="h-6 px-2 text-xs"
        >
          <ZoomOut className="h-3 w-3" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleReset}
          title="Reset view to initial state"
          className="h-6 px-2 text-xs"
        >
          <RotateCcw className="h-3 w-3" />
        </Button>
      </div>
      <div ref={timelineRef} style={{ width: '100%', height: '100%' }} />
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
          font-size: 12px !important;
        }
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
          padding-left: 4px;
          transform: translateY(-50%);
          top: 50%;
          white-space: nowrap;
          font-size: 12px !important;
          font-weight: 500;
        }
        .vis-item {
          overflow: visible !important;
          height: 18px !important;
        }
        .vis-time-axis .vis-text {
          font-size: 11px !important;
        }
        .vis-time-axis .vis-text.vis-major {
          font-size: 12px !important;
          font-weight: 600;
        }
        .vis-time-axis .vis-text.vis-minor {
          font-size: 10px !important;
        }
        .vis-time-axis .vis-grid.vis-minor {
          border-color: #f0f0f0;
        }
        .vis-time-axis .vis-grid.vis-major {
          border-color: #ddd;
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
    <div className="w-full h-[60vh] overflow-auto bg-background border-t">
      {children}
    </div>
  );
}

export default DashboardTimeChart;
