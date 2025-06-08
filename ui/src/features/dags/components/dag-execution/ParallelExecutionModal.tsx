import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ExternalLink, GitBranch } from 'lucide-react';
import React from 'react';
import { components } from '../../../../api/v2/schema';

type Props = {
  isOpen: boolean;
  onClose: () => void;
  stepName: string;
  childDAGName: string;
  children: components['schemas']['ChildDAGRun'][];
  onSelectChild: (childIndex: number, openInNewTab?: boolean) => void;
};

export function ParallelExecutionModal({
  isOpen,
  onClose,
  stepName,
  childDAGName,
  children,
  onSelectChild,
}: Props) {
  return (
    <Dialog open={isOpen} onOpenChange={onClose}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <GitBranch className="h-5 w-5" />
            Select Child DAG-run
          </DialogTitle>
          <DialogDescription>
            Choose which child DAG-run of "{childDAGName}" to view
          </DialogDescription>
        </DialogHeader>
        
        <div className="mt-4 space-y-2 max-h-[400px] overflow-y-auto">
          {children.map((child, index) => (
            <div key={child.dagRunId} className="flex gap-2">
              <Button
                variant="outline"
                className="flex-1 justify-start text-left p-6 hover:bg-muted shadow-none"
                onClick={(e) => {
                  const openInNewTab = e.metaKey || e.ctrlKey;
                  onSelectChild(index, openInNewTab);
                  if (!openInNewTab) {
                    onClose();
                  }
                }}
              >
                <div className="flex flex-col gap-2 w-full">
                  <div className="flex items-center gap-2">
                    <span className="font-medium">Child DAG-run #{index + 1}</span>
                  </div>
                  {child.params && (
                    <div className="text-sm text-muted-foreground font-mono">
                      {child.params}
                    </div>
                  )}
                </div>
              </Button>
              <Button
                variant="ghost"
                size="icon"
                className="h-auto px-3"
                onClick={() => {
                  onSelectChild(index, true);
                }}
                title="Open in new tab"
              >
                <ExternalLink className="h-4 w-4" />
              </Button>
            </div>
          ))}
        </div>
        
        <div className="text-sm text-muted-foreground mt-4">
          Total: {children.length} child DAG-runs
        </div>
      </DialogContent>
    </Dialog>
  );
}