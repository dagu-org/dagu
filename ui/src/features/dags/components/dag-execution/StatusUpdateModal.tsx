/**
 * StatusUpdateModal component provides a modal dialog for manually updating a step's status.
 *
 * @module features/dags/components/dag-execution
 */
import React from 'react';
import { components, NodeStatus } from '../../../../api/v2/schema';
import { Button } from '@/components/ui/button';

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
};

/**
 * StatusUpdateModal allows manually setting a step's status to success or failure
 */
function StatusUpdateModal({ visible, dismissModal, step, onSubmit }: Props) {
  // Handle ESC key to close the modal
  React.useEffect(() => {
    const callback = (event: KeyboardEvent) => {
      const e = event || window.event;
      if (e.key == 'Escape' || e.key == 'Esc') {
        dismissModal();
      }
    };
    document.addEventListener('keydown', callback);
    return () => {
      document.removeEventListener('keydown', callback);
    };
  }, [dismissModal]);

  // Don't render if no step is provided or modal is not visible
  if (!step || !visible) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50">
      <div className="w-full max-w-md rounded-lg border-2 border-black bg-white p-6 shadow-xl">
        <div className="flex items-center justify-center">
          <h2 className="text-xl font-semibold">
            Update status of "{step.name}"
          </h2>
        </div>

        <div className="mt-4 flex flex-col space-y-4">
          <div className="flex justify-center space-x-4">
            <Button
              variant="outline"
              onClick={() => onSubmit(step, NodeStatus.Success)}
            >
              Mark Success
            </Button>
            <Button
              variant="outline"
              onClick={() => onSubmit(step, NodeStatus.Failed)}
            >
              Mark Failed
            </Button>
          </div>

          <div className="flex justify-center">
            <Button
              variant="outline"
              className="text-destructive hover:bg-destructive/10"
              onClick={dismissModal}
            >
              Cancel
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}

export default StatusUpdateModal;
