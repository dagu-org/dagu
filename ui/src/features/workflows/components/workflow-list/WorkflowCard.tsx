import { useEffect, useState } from 'react';
import { components } from '../../../../api/v2/schema';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '../../../../components/ui/card';
import StatusChip from '../../../../ui/StatusChip';
import { WorkflowDetailsModal } from '../../components/workflow-details';

interface WorkflowCardProps {
  workflow: components['schemas']['WorkflowSummary'];
  timezoneInfo: string;
}

function WorkflowCard({ workflow, timezoneInfo }: WorkflowCardProps) {
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
    <Card className="h-full hover:shadow-md transition-shadow">
      <div
        className="block h-full no-underline text-inherit cursor-pointer"
        onClick={() => setIsModalOpen(true)}
      >
        <CardHeader className="pb-2 px-4 py-3">
          <CardTitle className="text-sm truncate" title={workflow.name}>
            {workflow.name}
          </CardTitle>
          <CardDescription
            className="text-xs truncate"
            title={workflow.workflowId}
          >
            ID: {workflow.workflowId}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-1.5 pt-0 px-4 pb-3">
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Status:</span>
            <StatusChip status={workflow.status} size="xs">
              {workflow.statusLabel}
            </StatusChip>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Started:</span>
            <span className="truncate ml-2">{workflow.startedAt}</span>
          </div>
          <div className="flex justify-between items-center text-xs">
            <span className="text-muted-foreground text-xs">Finished:</span>
            <span className="truncate ml-2">{workflow.finishedAt || '-'}</span>
          </div>
          <div className="text-[10px] text-muted-foreground text-right pt-1">
            {timezoneInfo}
          </div>
        </CardContent>
      </div>

      {/* Workflow Details Modal */}
      {isModalOpen && (
        <WorkflowDetailsModal
          name={workflow.name}
          workflowId={workflow.workflowId}
          isOpen={isModalOpen}
          onClose={() => setIsModalOpen(false)}
        />
      )}
    </Card>
  );
}

export default WorkflowCard;
