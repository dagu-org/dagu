import React from 'react';
import { Brain, Loader2, Save, Trash2 } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useCanWrite } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';
import ConfirmModal from '@/ui/ConfirmModal';

type AutomataMemoryResponse = {
  name: string;
  content: string;
  path: string;
};

type AutomataMemorySectionProps = {
  automataName: string;
  title?: string;
};

export function AutomataMemorySection({
  automataName,
  title = 'Memory',
}: AutomataMemorySectionProps): React.ReactElement {
  const client = useClient();
  const canWrite = useCanWrite();

  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [isDeleting, setIsDeleting] = React.useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState<string | null>(null);
  const [content, setContent] = React.useState('');
  const [memoryPath, setMemoryPath] = React.useState('');

  const loadMemory = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const { data, error: apiError } = await client.GET('/automata/{name}/memory', {
        params: { path: { name: automataName } },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to load automata memory');
      }
      const memory = data as AutomataMemoryResponse;
      setContent(memory.content ?? '');
      setMemoryPath(memory.path ?? '');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load automata memory');
    } finally {
      setIsLoading(false);
    }
  }, [automataName, client]);

  React.useEffect(() => {
    void loadMemory();
  }, [loadMemory]);

  async function handleSave(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { data, error: apiError } = await client.PUT('/automata/{name}/memory', {
        params: { path: { name: automataName } },
        body: { content },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to save automata memory');
      }
      const memory = data as AutomataMemoryResponse;
      setContent(memory.content ?? '');
      setMemoryPath(memory.path ?? memoryPath);
      setSuccess('Automata memory saved');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save automata memory');
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete(): Promise<void> {
    setIsDeleting(true);
    setError(null);
    setSuccess(null);
    try {
      const { error: apiError } = await client.DELETE('/automata/{name}/memory', {
        params: { path: { name: automataName } },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to clear automata memory');
      }
      setContent('');
      setShowDeleteConfirm(false);
      setSuccess('Automata memory cleared');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to clear automata memory');
    } finally {
      setIsDeleting(false);
    }
  }

  const lineCount = content ? content.split('\n').length : 0;

  return (
    <>
      <div className="min-w-0 rounded-lg border p-4">
        <div className="mb-3 flex items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 text-sm font-semibold">
              <Brain className="h-4 w-4 text-muted-foreground" />
              {title}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              Long-lived notes and reusable lessons for this automata.
            </p>
            {memoryPath ? (
              <p className="mt-1 break-all font-mono text-[11px] text-muted-foreground">
                {memoryPath}
              </p>
            ) : null}
          </div>
          <span className="text-xs text-muted-foreground">{lineCount}/200 lines</span>
        </div>

        {error ? (
          <div className="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {error}
          </div>
        ) : null}

        {success ? (
          <div className="mb-3 rounded-md bg-green-500/10 px-3 py-2 text-sm text-green-600">
            {success}
          </div>
        ) : null}

        {isLoading ? (
          <div className="flex items-center justify-center py-12 text-sm text-muted-foreground">
            <Loader2 className="mr-2 h-4 w-4 animate-spin" />
            Loading memory...
          </div>
        ) : (
          <div className="space-y-3">
            <Textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              readOnly={!canWrite}
              className="min-h-56 font-mono text-sm"
              placeholder="No automata memory yet. Save operating rules, durable context, or learned procedures here."
            />
            {canWrite ? (
              <div className="flex gap-2">
                <Button onClick={() => void handleSave()} disabled={isSaving} size="sm">
                  {isSaving ? (
                    <>
                      <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                      Saving...
                    </>
                  ) : (
                    <>
                      <Save className="mr-1.5 h-4 w-4" />
                      Save
                    </>
                  )}
                </Button>
                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => setShowDeleteConfirm(true)}
                  disabled={!content || isDeleting}
                >
                  <Trash2 className="mr-1.5 h-4 w-4" />
                  Clear
                </Button>
              </div>
            ) : null}
          </div>
        )}
      </div>

      <ConfirmModal
        title="Clear Automata Memory"
        buttonText="Clear"
        visible={showDeleteConfirm}
        dismissModal={() => setShowDeleteConfirm(false)}
        onSubmit={handleDelete}
      >
        <p>
          Are you sure you want to clear the memory for &quot;{automataName}&quot;? This
          action cannot be undone.
        </p>
      </ConfirmModal>
    </>
  );
}
