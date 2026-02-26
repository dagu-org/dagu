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
  currentPath: string;
  onSubmit: (newPath: string) => void;
  onDismiss: () => void;
};

export function RenameDocModal({ visible, currentPath, onSubmit, onDismiss }: Props) {
  const [newPath, setNewPath] = useState(currentPath);
  const [error, setError] = useState('');

  const cancelButtonRef = React.useRef<HTMLButtonElement>(null);
  const submitButtonRef = React.useRef<HTMLButtonElement>(null);

  React.useEffect(() => {
    if (visible) {
      setNewPath(currentPath);
      setError('');
    }
  }, [visible, currentPath]);

  const handleSubmit = useCallback(() => {
    const trimmed = newPath.trim();
    if (!trimmed) {
      setError('Path is required');
      return;
    }
    if (trimmed === currentPath) {
      setError('New path must be different');
      return;
    }
    if (trimmed.startsWith('/') || trimmed.endsWith('/')) {
      setError('Path should not start or end with /');
      return;
    }
    onSubmit(trimmed);
  }, [newPath, currentPath, onSubmit]);

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
        e.preventDefault();
        handleSubmit();
      }
    };
    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [visible, onDismiss, handleSubmit]);

  return (
    <Dialog open={visible} onOpenChange={(open) => !open && onDismiss()}>
      <DialogContent className="sm:max-w-[500px]">
        <DialogHeader>
          <DialogTitle>Rename Document</DialogTitle>
        </DialogHeader>
        <div className="py-3 space-y-3">
          <div>
            <label className="text-sm font-medium text-muted-foreground">Current path</label>
            <div className="text-sm font-mono mt-1 px-3 py-1.5 bg-muted rounded-md">{currentPath}</div>
          </div>
          <div>
            <label className="text-sm font-medium">New path</label>
            <input
              autoFocus
              type="text"
              value={newPath}
              onChange={(e) => { setNewPath(e.target.value); setError(''); }}
              className="w-full mt-1 px-3 py-1.5 text-sm font-mono border border-input rounded-md bg-background focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          {error && <p className="text-xs text-destructive">{error}</p>}
        </div>
        <DialogFooter>
          <Button ref={cancelButtonRef} variant="ghost" onClick={onDismiss}>
            <X className="h-4 w-4" />
            Cancel
          </Button>
          <Button ref={submitButtonRef} className="btn-3d-primary" onClick={handleSubmit}>
            <Check className="h-4 w-4" />
            Rename
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
