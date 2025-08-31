import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useCallback, useEffect, useState } from 'react';
import { validateDAGName, DAG_NAME_PATTERN_STRING } from '../lib/dag-validation';

export interface DAGNameInputModalProps {
  /** Whether the modal is open */
  isOpen: boolean;
  /** Function to close the modal */
  onClose: () => void;
  /** Function called when form is submitted with valid name */
  onSubmit: (name: string) => void;
  /** Mode determines the UI text and behavior */
  mode: 'create' | 'rename';
  /** Initial value for the input (used in rename mode) */
  initialValue?: string;
  /** Loading state */
  isLoading?: boolean;
  /** Error from the parent component (e.g., server errors) */
  externalError?: string | null;
}

/**
 * Reusable modal for DAG name input with validation
 * Supports both create and rename modes
 */
export function DAGNameInputModal({
  isOpen,
  onClose,
  onSubmit,
  mode,
  initialValue = '',
  isLoading = false,
  externalError = null,
}: DAGNameInputModalProps) {
  const [name, setName] = useState(initialValue);
  const [validationError, setValidationError] = useState<string | null>(null);

  // Update name when initialValue changes (for rename mode)
  useEffect(() => {
    setName(initialValue);
  }, [initialValue]);

  // Clear errors when modal opens/closes
  useEffect(() => {
    if (isOpen) {
      setValidationError(null);
    } else {
      setName(initialValue);
      setValidationError(null);
    }
  }, [isOpen, initialValue]);

  const handleNameChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const newName = e.target.value;
    setName(newName);
    
    // Clear errors when user types
    if (validationError) {
      setValidationError(null);
    }
  }, [validationError]);

  const handleSubmit = useCallback((e: React.FormEvent) => {
    e.preventDefault();
    
    const validation = validateDAGName(name);
    if (!validation.isValid) {
      setValidationError(validation.error || 'Invalid DAG name');
      return;
    }
    
    onSubmit(name.trim());
  }, [name, onSubmit]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      onClose();
    }
  }, [onClose]);

  // Get modal content based on mode
  const getModalContent = () => {
    switch (mode) {
      case 'create':
        return {
          title: 'Create New DAG',
          description: 'Enter a name for your new DAG. Only letters, numbers, underscores, dots, and hyphens are allowed.',
          submitText: 'Create',
          placeholder: 'my_new_dag',
        };
      case 'rename':
        return {
          title: 'Rename DAG',
          description: 'Enter a new name for your DAG. Only letters, numbers, underscores, dots, and hyphens are allowed.',
          submitText: 'Rename',
          placeholder: 'my_renamed_dag',
        };
      default:
        throw new Error(`Unsupported mode: ${mode}`);
    }
  };

  const modalContent = getModalContent();
  const currentError = externalError || validationError;

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[425px]" onKeyDown={handleKeyDown}>
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>{modalContent.title}</DialogTitle>
            <DialogDescription>
              {modalContent.description}
              <div className="mt-1 font-mono text-xs bg-slate-100 p-1 rounded dark:bg-slate-800">
                Pattern: {DAG_NAME_PATTERN_STRING}
              </div>
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="dag-name" className="text-right">
                DAG Name
              </Label>
              <Input
                id="dag-name"
                value={name}
                onChange={handleNameChange}
                className="col-span-3"
                placeholder={modalContent.placeholder}
                pattern={DAG_NAME_PATTERN_STRING}
                autoFocus
                disabled={isLoading}
              />
            </div>
            {currentError && (
              <div className="text-destructive text-sm px-4">
                {currentError}
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={onClose}
              disabled={isLoading}
            >
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading}>
              {isLoading ? `${modalContent.submitText}ing...` : modalContent.submitText}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}