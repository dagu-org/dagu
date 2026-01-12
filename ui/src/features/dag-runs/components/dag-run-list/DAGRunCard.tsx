import { Server } from 'lucide-react';
import { useEffect, useState } from 'react';
import { components, Status } from '../../../../api/v2/schema';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '../../../../components/ui/card';
import StatusChip from '../../../../ui/StatusChip';
import { DAGRunDetailsModal } from '../../components/dag-run-details';

interface DAGRunCardProps {
  dagRun: components['schemas']['DAGRunSummary'];
  timezoneInfo: string;
}

function DAGRunCard({ dagRun, timezoneInfo }: DAGRunCardProps) {
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Add keyboard navigation for the modal
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isModalOpen) return;

      // Close modal with Escape key
      if (event.key === 'Escape') {
        setIsModalOpen(false);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isModalOpen]);
  return (
    <Card className={`h-full hover: ${dagRun.status === Status.Running ? 'animate-running-row' : ''}`}>
      <div
        className="block h-full no-underline text-inherit cursor-pointer"
        onClick={(e) => {
          if (e.ctrlKey || e.metaKey) {
            // Open in new tab
            window.open(`/dag-runs/${dagRun.name}/${dagRun.dagRunId}`, '_blank');
          } else {
            // Open modal
            setIsModalOpen(true);
          }
        }}
      >
        <CardHeader className="pb-2 px-4 py-3">
          <CardTitle className="text-sm truncate" title={dagRun.name}>
            {dagRun.name}
          </CardTitle>
          <CardDescription className="text-xs truncate" title={dagRun.dagRunId}>
            ID: {dagRun.dagRunId}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1.5 pt-0 px-4 pb-3">
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Status:</span>
            <StatusChip status={dagRun.status} size="xs">
              {dagRun.statusLabel}
            </StatusChip>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Started:</span>
            <span className="truncate ml-2">{dagRun.startedAt}</span>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Finished:</span>
            <span className="truncate ml-2">{dagRun.finishedAt || '-'}</span>
          </div>
          {dagRun.workerId && (
            <div className="flex justify-between items-center text-xs">
              <span className="text-muted-foreground text-xs flex items-center">
                <Server className="h-3 w-3 mr-1" />
                Worker:
              </span>
              <span className="truncate ml-2 font-mono text-[11px]">
                {dagRun.workerId}
              </span>
            </div>
          )}
          <div className="text-[10px] text-muted-foreground text-right pt-1">
            {timezoneInfo}
          </div>
        </CardContent>
      </div>

      {/* DAGRun Details Modal */}
      {isModalOpen && (
        <DAGRunDetailsModal
          name={dagRun.name}
          dagRunId={dagRun.dagRunId}
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
        />
      )}
    </Card>
  );
}

export default DAGRunCard;
