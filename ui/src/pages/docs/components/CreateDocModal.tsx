import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/ui/CustomDialog';
import { Check, X } from 'lucide-react';
import React, { useCallback, useState } from 'react';

type Props = {
  visible: boolean;
  parentDir?: string;
  onSubmit: (docPath: string) => void;
  onDismiss: () => void;
};

export function CreateDocModal({ visible, parentDir, onSubmit, onDismiss }: Props) {
  const [path, setPath] = useState(parentDir ? `${parentDir}/` : '');
  const [error, setError] = useState('');

  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const submitButtonRef = React.useRef<HTMLButtonElement>(null);

  React.useEffect(() => {
    if (visible) {
      setPath(parentDir ? `${parentDir}/` : '');
      setError('');
    }
  }, [visible, parentDir]);

  const handleSubmit = useCallback(() => {
    const trimmed = path.trim();
    if (!trimmed) {
      setError('Path is required');
      return;
    }
    if (trimmed.startsWith('/') || trimmed.endsWith('/')) {
      setError('Path should not start or end with /');
      return;
    }
    onSubmit(trimmed);
  }, [path, onSubmit]);

  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!visible) return;
      if (e.key === 'Enter') {
        const activeElement = document.activeElement;
        if (activeElement === cancelButtonRef.current) {
          e.preventDefault();
          onDismiss();
          return;
        }
        if (activeElement === submitButtonRef.current) {
          e.preventDefault();
          handleSubmit();
          return;
        }
        if (activeElement instanceof HTMLInputElement) {
          e.preventDefault();
          handleSubmit();
          return;
        }
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [visible, onDismiss, handleSubmit]);

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && onDismiss()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>New Document</DialogTitle>
        </DialogHeader>
        <div className="py-3 space-y-2">
          <label className="text-sm font-medium">Document Path</label>
          <input
            autoFocus
            type="text"
            value={path}
            onChange={(e) => { setPath(e.target.value); setError(''); }}
            placeholder="runbooks/deployment"
            className="w-full px-3 py-1.5 text-sm font-mono border border-input rounded-md bg-background focus:outline-none focus:ring-1 focus:ring-ring"
          />
          <p className="text-xs text-muted-foreground">
            Relative path without .md extension. Use / for directories.
          </p>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button ref={cancelButtonRef} variant="ghost" onClick={onDismiss}>
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button ref={submitButtonRef} className="btn-3d-primary" onClick={handleSubmit}>
            <Check className="h-4 w-4" />
            Create
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
