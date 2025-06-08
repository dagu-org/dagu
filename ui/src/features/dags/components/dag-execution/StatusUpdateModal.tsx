/**
 * StatusUpdateModal component provides a modal dialog for manually updating a step's status.
 *
 * @module features/dags/components/dag-execution
 */
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { CheckCircle2, XCircle } from 'lucide-react';
import React from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';

/**
 * Props for the StatusUpdateModal component
 */
type Props = {
  /** Whether the modal is visible */
  visible: boolean;
  /** Function to close the modal */
  dismissModal: () => void;
  /** Step to update status for */
  step?: components['schemas']['Step'];
  /** Function called when the user submits the status update */
  onSubmit: (step: components['schemas']['Step'], status: NodeStatus) => void;
  /** Optional position for the modal (x, y coordinates) */
  position?: { x: number; y: number };
};

/**
 * StatusUpdateModal allows manually setting a step's status to success or failure
 */
function StatusUpdateModal({ visible, dismissModal, step, onSubmit }: Props) {
  // Don't render if no step is provided
  if (!step) {
    return null;
  }

  // Create refs for the buttons
  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const successButtonRef = React.useRef<HTMLButtonElement>(null);
  const failedButtonRef = React.useRef<HTMLButtonElement>(null);
  
  // Track which button is selected (0 = success, 1 = failed)
  const [selectedButton, setSelectedButton] = React.useState(0);

  // Handle keyboard events
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Only handle events when modal is visible
      if (!visible || !step) return;

      switch (e.key) {
        case 'ArrowLeft':
        case 'ArrowRight':
          e.preventDefault();
          setSelectedButton(selectedButton === 0 ? 1 : 0);
          break;
          
        case 'Enter':
          e.preventDefault();
          if (selectedButton === 0) {
            onSubmit(step, NodeStatus.Success);
          } else {
            onSubmit(step, NodeStatus.Failed);
          }
          break;
          
        case 'Escape':
          e.preventDefault();
          dismissModal();
          break;
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, step, onSubmit, dismissModal, selectedButton]);

  // We're not using the position prop anymore as we want the modal to be centered
  // The Dialog component from shadcn/ui will center the modal by default

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className="sm:max-w-[400px]">
        <DialogHeader>
          <DialogTitle className="text-base font-mono">
            Update Status
          </DialogTitle>
          <p className="text-sm text-muted-foreground font-mono mt-1">
            {step.name}
          </p>
        </DialogHeader>

        <div className="py-6">
          <div className="grid grid-cols-2 gap-3">
            <button
              ref={successButtonRef}
              className={`
                group relative overflow-hidden rounded-lg border p-4 transition-all duration-200 focus:outline-none
                ${selectedButton === 0 
                  ? 'border-green-500 dark:border-green-600 bg-green-50 dark:bg-green-950/20' 
                  : 'border-zinc-200 dark:border-zinc-800 hover:border-green-500 dark:hover:border-green-600 hover:bg-green-50 dark:hover:bg-green-950/20'
                }
              `}
              onClick={() => onSubmit(step, NodeStatus.Success)}
              onMouseEnter={() => setSelectedButton(0)}
            >
              <div className="flex flex-col items-center gap-2">
                <CheckCircle2 className="h-8 w-8 text-green-600 dark:text-green-500" />
                <span className="font-mono text-sm">Success</span>
              </div>
            </button>
            
            <button
              ref={failedButtonRef}
              className={`
                group relative overflow-hidden rounded-lg border p-4 transition-all duration-200 focus:outline-none
                ${selectedButton === 1 
                  ? 'border-red-500 dark:border-red-600 bg-red-50 dark:bg-red-950/20' 
                  : 'border-zinc-200 dark:border-zinc-800 hover:border-red-500 dark:hover:border-red-600 hover:bg-red-50 dark:hover:bg-red-950/20'
                }
              `}
              onClick={() => onSubmit(step, NodeStatus.Failed)}
              onMouseEnter={() => setSelectedButton(1)}
            >
              <div className="flex flex-col items-center gap-2">
                <XCircle className="h-8 w-8 text-red-600 dark:text-red-500" />
                <span className="font-mono text-sm">Failed</span>
              </div>
            </button>
          </div>
        </div>

        <div className="flex items-center gap-3 text-xs text-zinc-500 dark:text-zinc-500 font-mono pt-2 border-t border-zinc-200 dark:border-zinc-800">
          <span>←→ Select</span>
          <span className="opacity-40">•</span>
          <span>Enter: Confirm</span>
          <span className="opacity-40">•</span>
          <span>ESC: Cancel</span>
        </div>
      </DialogContent>
    </Dialog>
  );
}

export default StatusUpdateModal;
