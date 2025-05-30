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

  // Handle keyboard events
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Only handle events when modal is visible
      if (!visible || !step) return;

      // Handle Enter key
      if (e.key === 'Enter') {
        // Get the active element
        const activeElement = document.activeElement;

        // Don't do anything if focus is on an input element
        const isInputFocused =
          activeElement instanceof HTMLInputElement ||
          activeElement instanceof HTMLTextAreaElement ||
          activeElement instanceof HTMLSelectElement;

        if (isInputFocused) {
          return;
        }

        // If Cancel button is focused, trigger cancel
        if (activeElement === cancelButtonRef.current) {
          e.preventDefault();
          dismissModal();
          return;
        }

        // If Success button is focused, trigger success
        if (activeElement === successButtonRef.current) {
          e.preventDefault();
          onSubmit(step, NodeStatus.Success);
          return;
        }

        // If Failed button is focused, trigger failed
        if (activeElement === failedButtonRef.current) {
          e.preventDefault();
          onSubmit(step, NodeStatus.Failed);
          return;
        }

        // If any other button is focused, let it handle the event naturally
        if (activeElement instanceof HTMLButtonElement) {
          return;
        }

        // If no specific element is focused, trigger the success action as default
        e.preventDefault();
        onSubmit(step, NodeStatus.Success);
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, step, onSubmit, dismissModal]);

  // We're not using the position prop anymore as we want the modal to be centered
  // The Dialog component from shadcn/ui will center the modal by default

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Update status of "{step.name}"</DialogTitle>
        </DialogHeader>

        <div className="py-4">
          <div className="flex justify-center space-x-4">
            <Button
              ref={successButtonRef}
              variant="default"
              className="bg-green-600 hover:bg-green-700"
              onClick={() => onSubmit(step, NodeStatus.Success)}
            >
              Mark Success
            </Button>
            <Button
              ref={failedButtonRef}
              variant="default"
              className="bg-red-600 hover:bg-red-700"
              onClick={() => onSubmit(step, NodeStatus.Failed)}
            >
              Mark Failed
            </Button>
          </div>
        </div>

        <DialogFooter>
          <Button
            ref={cancelButtonRef}
            variant="outline"
            onClick={dismissModal}
          >
            Cancel
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default StatusUpdateModal;
