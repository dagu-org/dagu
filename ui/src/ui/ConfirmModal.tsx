import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { X, Check } from 'lucide-react';
import React from 'react';

type Props = {
  title: string;
  buttonText: string;
  children: React.ReactNode;
  visible: boolean;
  dismissModal: () => void;
  onSubmit: () => void;
};

function ConfirmModal({
  children,
  title,
  buttonText,
  visible,
  dismissModal,
  onSubmit,
}: Props) {
  // Create refs for the buttons
  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const submitButtonRef = React.useRef<HTMLButtonElement>(null);

  // Handle keyboard events
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Only handle events when modal is visible
      if (!visible) return;

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

        // If Submit button is focused, trigger submit
        if (activeElement === submitButtonRef.current) {
          e.preventDefault();
          onSubmit();
          return;
        }

        // If any other button is focused, let it handle the event naturally
        if (activeElement instanceof HTMLButtonElement) {
          return;
        }

        // If no specific element is focused, trigger the submit action as default
        e.preventDefault();
        onSubmit();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, onSubmit, dismissModal]);
  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        <div className="py-4">{children}</div>

        <DialogFooter>
          <Button
            ref={cancelButtonRef}
            variant="ghost"
            onClick={dismissModal}
          >
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button ref={submitButtonRef} onClick={onSubmit}>
            <Check className="h-4 w-4" />
            {buttonText}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default ConfirmModal;
