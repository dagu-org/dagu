// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { components, StatusLabel } from '@/api/v1/schema';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Sparkles, Wand2, X } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';

type DAGRunDetails = components['schemas']['DAGRunDetails'];

type Props = {
  isOpen: boolean;
  onClose: () => void;
  onSubmit: (prompt: string) => Promise<void> | void;
  dagFile: string;
  dagName?: string;
  latestDAGRun?: DAGRunDetails;
  isLoading?: boolean;
};

export default function ImproveDAGDefinitionModal({
  isOpen,
  onClose,
  onSubmit,
  dagFile,
  dagName,
  latestDAGRun,
  isLoading = false,
}: Props) {
  const [prompt, setPrompt] = useState('');
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!isOpen) {
      setPrompt('');
      setError(null);
    }
  }, [isOpen]);

  const latestRunVariant = useMemo(() => {
    switch (latestDAGRun?.statusLabel) {
      case StatusLabel.succeeded:
        return 'success';
      case StatusLabel.failed:
      case StatusLabel.aborted:
      case StatusLabel.rejected:
        return 'error';
      case StatusLabel.queued:
      case StatusLabel.waiting:
      case StatusLabel.partially_succeeded:
        return 'warning';
      case StatusLabel.running:
        return 'info';
      default:
        return 'secondary';
    }
  }, [latestDAGRun?.statusLabel]);

  const handleSubmit = useCallback(async () => {
    const trimmed = prompt.trim();
    if (!trimmed) {
      setError('Describe what should be improved before starting the agent.');
      return;
    }
    setError(null);
    await onSubmit(trimmed);
  }, [onSubmit, prompt]);

  const handleKeyDown = useCallback(
    async (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if ((event.metaKey || event.ctrlKey) && event.key === 'Enter') {
        event.preventDefault();
        await handleSubmit();
      }
    },
    [handleSubmit]
  );

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && onClose()}>
      <DialogContent className="gap-3 p-3 pr-10 sm:max-w-xl">
        <DialogHeader className="space-y-1">
          <DialogTitle className="flex items-center gap-2 text-base">
            <Sparkles className="h-4 w-4 text-primary" />
            Improve DAG Definition
          </DialogTitle>
          <DialogDescription className="text-xs leading-5">
            Start a fresh agent session with this DAG reference, the latest run
            details, and your request.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-3 py-1">
          <div className="rounded-lg border border-border bg-slate-200 p-3 dark:bg-slate-700">
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="outline">{dagFile}</Badge>
              {dagName && dagName !== dagFile ? (
                <Badge variant="secondary">{dagName}</Badge>
              ) : null}
              {latestDAGRun ? (
                <>
                  <Badge variant={latestRunVariant}>
                    {latestDAGRun.statusLabel}
                  </Badge>
                  <Badge variant="outline">
                    {latestDAGRun.dagRunId.slice(0, 12)}
                  </Badge>
                </>
              ) : (
                <Badge variant="secondary">No run yet</Badge>
              )}
            </div>

            <p className="mt-2 text-xs leading-5 text-muted-foreground">
              Focus the agent on reliability, readability, timeouts, retries,
              dependency structure, step naming, or any other improvement you
              want applied to this DAG definition.
            </p>
          </div>

          <div className="grid gap-2">
            <Label htmlFor="improve-dag-prompt">What should be improved?</Label>
            <Textarea
              id="improve-dag-prompt"
              value={prompt}
              onChange={(event) => {
                setPrompt(event.target.value);
                if (error) {
                  setError(null);
                }
              }}
              onKeyDown={handleKeyDown}
              placeholder="Example: tighten retries and timeouts, clarify step names, and make failures easier to debug from the latest run."
              className="min-h-28 bg-slate-200 px-3 py-1 dark:bg-slate-700"
              autoFocus
              disabled={isLoading}
            />
            <p className="text-xs text-muted-foreground">
              Press Cmd/Ctrl+Enter to launch the agent.
            </p>
            {error ? <p className="text-sm text-destructive">{error}</p> : null}
          </div>
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
          <Button
            type="button"
            variant="primary"
            onClick={handleSubmit}
            disabled={isLoading}
          >
            <Wand2 className="h-4 w-4" />
            {isLoading ? 'Starting...' : 'Start Improvement'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
