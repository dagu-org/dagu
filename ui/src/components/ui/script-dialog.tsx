/**
 * ScriptDialog component for displaying script content in a clean modal.
 *
 * @module components/ui/script-dialog
 */
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { FileText } from 'lucide-react';
import { useState } from 'react';

interface ScriptDialogProps {
  /** The script content to display */
  script: string;
  /** Optional step name for the dialog title */
  stepName?: string;
  /** Render prop for the trigger element */
  children: React.ReactNode;
}

/**
 * ScriptDialog displays script content in a clean modal dialog.
 * Click on the trigger element to open the dialog.
 */
export function ScriptDialog({
  script,
  stepName,
  children,
}: ScriptDialogProps) {
  const [open, setOpen] = useState(false);

  const lines = script.split('\n');

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <div
        onClick={() => setOpen(true)}
        className="cursor-pointer"
        role="button"
        tabIndex={0}
        onKeyDown={(e) => {
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault();
            setOpen(true);
          }
        }}
      >
        {children}
      </div>
      <DialogContent className="max-w-2xl max-h-[80vh] flex flex-col p-0 gap-0">
        <DialogHeader className="px-4 py-3 border-b border-slate-200">
          <DialogTitle className="flex items-center gap-2 text-sm font-semibold">
            <FileText className="h-4 w-4 text-amber-500" />
            {stepName ? `Script: ${stepName}` : 'Script Content'}
          </DialogTitle>
        </DialogHeader>
        <div className="flex-1 overflow-auto min-h-0">
          <div className="bg-zinc-900 min-h-full">
            <pre className="font-mono text-[12px] text-zinc-100 p-3">
              {lines.map((line, index) => (
                <div key={index} className="flex hover:bg-zinc-800 px-1">
                  <span className="text-zinc-500 mr-4 select-none w-8 text-right flex-shrink-0">
                    {index + 1}
                  </span>
                  <span className="whitespace-pre-wrap break-all flex-grow">
                    {line || ' '}
                  </span>
                </div>
              ))}
            </pre>
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}

interface ScriptBadgeProps {
  /** The script content */
  script: string;
  /** Optional step name for the dialog title */
  stepName?: string;
}

/**
 * ScriptBadge is a pre-styled trigger that shows "Script defined" badge.
 * Clicking it opens the ScriptDialog.
 */
export function ScriptBadge({ script, stepName }: ScriptBadgeProps) {
  return (
    <ScriptDialog script={script} stepName={stepName}>
      <div className="flex items-center gap-1.5 text-xs bg-amber-50 rounded-md px-1.5 py-0.5 w-fit hover:bg-amber-100 transition-colors">
        <FileText className="h-3.5 w-3.5 text-amber-500" />
        <span className="font-medium text-amber-600">
          Script defined
        </span>
      </div>
    </ScriptDialog>
  );
}
