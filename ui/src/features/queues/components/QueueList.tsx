import { Layers } from 'lucide-react';
import React from 'react';
import type { components } from '../../../api/v2/schema';
import QueueCard from './QueueCard';

interface QueueListProps {
  queues: components['schemas']['Queue'][];
  isLoading?: boolean;
  onDAGRunClick: (dagRun: components['schemas']['DAGRunSummary']) => void;
  onQueueCleared?: () => void;
}

function QueueList({
  queues,
  isLoading,
  onDAGRunClick,
  onQueueCleared,
}: QueueListProps) {
  const [selectedIndex, setSelectedIndex] = React.useState<number>(-1);

  // Enhanced keyboard navigation for queues
  React.useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ArrowDown') {
        event.preventDefault();
        setSelectedIndex((prev) => {
          const newIndex = prev < queues.length - 1 ? prev + 1 : prev;
          return newIndex;
        });
      } else if (event.key === 'ArrowUp') {
        event.preventDefault();
        setSelectedIndex((prev) => {
          const newIndex = prev > 0 ? prev - 1 : prev === -1 ? 0 : prev;
          return newIndex;
        });
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [queues.length]);

  // Initialize selection when queues change
  React.useEffect(() => {
    if (queues.length > 0 && selectedIndex === -1) {
      setSelectedIndex(0);
    } else if (selectedIndex >= queues.length) {
      setSelectedIndex(queues.length - 1);
    }
  }, [queues, selectedIndex]);

  if (isLoading) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="text-center space-y-2">
          <Layers className="h-8 w-8 text-muted-foreground mx-auto animate-pulse" />
          <p className="text-sm text-muted-foreground">Loading queues...</p>
        </div>
      </div>
    );
  }

  if (queues.length === 0) {
    return (
      <div className="flex items-center justify-center h-32">
        <div className="text-center space-y-2">
          <Layers className="h-8 w-8 text-muted-foreground mx-auto" />
          <p className="text-sm text-muted-foreground">No queues found</p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-2">
      {queues.map((queue, index) => (
        <QueueCard
          key={queue.name}
          queue={queue}
          isSelected={index === selectedIndex}
          onDAGRunClick={onDAGRunClick}
          onQueueCleared={onQueueCleared}
        />
      ))}
    </div>
  );
}

export default QueueList;
