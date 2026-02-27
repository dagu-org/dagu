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
import { Plus, X } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';

const DOC_PATH_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_. -]*(\/[a-zA-Z0-9][a-zA-Z0-9_. -]*)*$/;

function validateDocPath(path: string): { isValid: boolean; error?: string } {
  const trimmed = path.trim();
  if (!trimmed) {
    return { isValid: false, error: 'Path is required' };
  }
  if (trimmed.length > 256) {
    return { isValid: false, error: 'Path must be 256 characters or fewer' };
  }
  if (!DOC_PATH_PATTERN.test(trimmed)) {
    return {
      isValid: false,
      error: 'Invalid path. Use letters, numbers, underscores, dots, hyphens, and spaces. Use / for directories.',
    };
  }
  return { isValid: true };
}

interface CreateDocModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (path: string) => Promise<void>;
  parentDir?: string;
  isLoading?: boolean;
  externalError?: string | null;
}

export function CreateDocModal({
  isOpen,
  onClose,
  onSubmit,
  parentDir = '',
  isLoading = false,
  externalError = null,
}: CreateDocModalProps) {
  const [path, setPath] = useState('');
  const [validationError, setValidationError] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      setPath(parentDir ? `${parentDir}/` : '');
      setValidationError(null);
    }
  }, [isOpen, parentDir]);

  const handlePathChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setPath(e.target.value);
      if (validationError) {
        setValidationError(null);
      }
    },
    [validationError]
  );

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const trimmed = path.trim();
      const validation = validateDocPath(trimmed);
      if (!validation.isValid) {
        setValidationError(validation.error || 'Invalid path');
        return;
      }
      await onSubmit(trimmed);
    },
    [path, onSubmit]
  );

  const currentError = externalError || validationError;

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="sm:max-w-[425px]">
        <form onSubmit={handleSubmit}>
          <DialogHeader>
            <DialogTitle>Create New Document</DialogTitle>
            <DialogDescription>
              Enter a path for your new document. Use / for directories.
            </DialogDescription>
          </DialogHeader>
          <div className="grid gap-4 py-4">
            <div className="grid grid-cols-4 items-center gap-4">
              <Label htmlFor="doc-path" className="text-right">
                Path
              </Label>
              <Input
                id="doc-path"
                value={path}
                onChange={handlePathChange}
                className="col-span-3 font-mono"
                placeholder="runbooks/deployment"
                autoFocus
                disabled={isLoading}
              />
            </div>
            <div className="text-xs text-muted-foreground px-4">
              Relative path without .md extension. Use / for directories.
            </div>
            {currentError && (
              <div className="text-destructive text-sm px-4">{currentError}</div>
            )}
          </div>
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={onClose}
              disabled={isLoading}
            >
              <X className="h-4 w-4" />
              Cancel
            </Button>
            <Button type="submit" disabled={isLoading}>
              <Plus className="h-4 w-4" />
              {isLoading ? 'Creating...' : 'Create'}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}
