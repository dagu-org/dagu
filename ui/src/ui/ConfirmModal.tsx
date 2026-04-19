// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Check, X } from 'lucide-react';
import React from 'react';

type Props = {
  title: string;
  buttonText: string;
  children: React.ReactNode;
  visible: boolean;
  dismissModal: () => void;
  onSubmit: () => void;
  submitDisabled?: boolean;
  contentClassName?: string;
  bodyClassName?: string;
};

function ConfirmModal({
  children,
  title,
  buttonText,
  visible,
  dismissModal,
  onSubmit,
  submitDisabled = false,
  contentClassName,
  bodyClassName,
}: Props) {
  // Create refs for the buttons
  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const submitButtonRef = React.useRef<HTMLButtonElement>(null);

  // Auto-focus the submit button when modal opens so Enter triggers submit
  React.useEffect(() => {
    if (visible && submitButtonRef.current) {
      // Delay to allow Radix Dialog to finish mounting and focus-trapping
      requestAnimationFrame(() => {
        submitButtonRef.current?.focus();
      });
    }
  }, [visible]);

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
          if (submitDisabled) {
            return;
          }
          onSubmit();
          return;
        }

        // If any other button is focused, let it handle the event naturally
        if (activeElement instanceof HTMLButtonElement) {
          return;
        }

        // If no specific element is focused, trigger the submit action as default
        e.preventDefault();
        if (submitDisabled) {
          return;
        }
        onSubmit();
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('keydown', handleKeyDown);
    };
  }, [visible, onSubmit, dismissModal, submitDisabled]);
  return (
    <Dialog open={visible} onOpenChange={(open) => !open && dismissModal()}>
      <DialogContent className={cn('sm:max-w-[500px]', contentClassName)}>
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription className="sr-only">
            Confirm the requested action.
          </DialogDescription>
        </DialogHeader>

        <div className={cn('py-4', bodyClassName)}>{children}</div>

        <DialogFooter>
          <Button ref={cancelButtonRef} variant="ghost" onClick={dismissModal}>
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button
            ref={submitButtonRef}
            className="btn-3d-primary"
            disabled={submitDisabled}
            onClick={onSubmit}
          >
            <Check className="h-4 w-4" />
            {buttonText}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export default ConfirmModal;
