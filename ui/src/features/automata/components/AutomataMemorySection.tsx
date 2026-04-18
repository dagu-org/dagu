import React from 'react';
import { Brain, Loader2, Save, Sparkles, Trash2 } from 'lucide-react';

import { AutomataDocument, type components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import { useCanWrite } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';
import ConfirmModal from '@/ui/ConfirmModal';

type AutomataDocumentResponse =
  components['schemas']['AutomataDocumentResponse'];

type AutomataDocumentSectionProps = {
  automataName: string;
  document: AutomataDocument;
};

const documentCopy: Record<
  AutomataDocument,
  {
    title: string;
    description: string;
    placeholder: string;
    loading: string;
    saved: string;
    cleared: string;
    loadError: string;
    saveError: string;
    clearError: string;
    clearTitle: string;
    clearMessage: (automataName: string) => string;
  }
> = {
  [AutomataDocument.MEMORY_md]: {
    title: 'Memory',
    description: 'Long-lived notes and reusable lessons for this automata.',
    placeholder:
      'No automata memory yet. Save operating rules, durable context, or learned procedures here.',
    loading: 'Loading memory...',
    saved: 'Automata memory saved',
    cleared: 'Automata memory cleared',
    loadError: 'Failed to load automata memory',
    saveError: 'Failed to save automata memory',
    clearError: 'Failed to clear automata memory',
    clearTitle: 'Clear Automata Memory',
    clearMessage: (automataName) =>
      `Are you sure you want to clear the memory for "${automataName}"? This action cannot be undone.`,
  },
  [AutomataDocument.SOUL_md]: {
    title: 'Soul',
    description:
      'Identity, priorities, and communication style for this automata.',
    placeholder:
      'No automata soul yet. Define how this automata should think, speak, and prioritize work.',
    loading: 'Loading soul...',
    saved: 'Automata soul saved',
    cleared: 'Automata soul cleared',
    loadError: 'Failed to load automata soul',
    saveError: 'Failed to save automata soul',
    clearError: 'Failed to clear automata soul',
    clearTitle: 'Clear Automata Soul',
    clearMessage: (automataName) =>
      `Are you sure you want to clear the soul for "${automataName}"? This action cannot be undone.`,
  },
};

export function AutomataDocumentSection({
  automataName,
  document,
}: AutomataDocumentSectionProps): React.ReactElement {
  const client = useClient();
  const canWrite = useCanWrite();
  const copy = documentCopy[document];
  const Icon = document === AutomataDocument.SOUL_md ? Sparkles : Brain;

  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [isDeleting, setIsDeleting] = React.useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState<string | null>(null);
  const [content, setContent] = React.useState('');
  const [documentPath, setDocumentPath] = React.useState('');

  const loadDocument = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const { data, error: apiError } = await client.GET(
        '/automata/{name}/documents/{document}',
        {
          params: { path: { name: automataName, document } },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.loadError);
      }
      const item = data as AutomataDocumentResponse;
      setContent(item.content ?? '');
      setDocumentPath(item.path ?? '');
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.loadError);
    } finally {
      setIsLoading(false);
    }
  }, [automataName, client, copy.loadError, document]);

  React.useEffect(() => {
    void loadDocument();
  }, [loadDocument]);

  async function handleSave(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { data, error: apiError } = await client.PUT(
        '/automata/{name}/documents/{document}',
        {
          params: { path: { name: automataName, document } },
          body: { content },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.saveError);
      }
      const item = data as AutomataDocumentResponse;
      setContent(item.content ?? '');
      setDocumentPath(item.path ?? documentPath);
      setSuccess(copy.saved);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.saveError);
    } finally {
      setIsSaving(false);
    }
  }

  async function handleDelete(): Promise<void> {
    setIsDeleting(true);
    setError(null);
    setSuccess(null);
    try {
      const { error: apiError } = await client.DELETE(
        '/automata/{name}/documents/{document}',
        {
          params: { path: { name: automataName, document } },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.clearError);
      }
      setContent('');
      setShowDeleteConfirm(false);
      setSuccess(copy.cleared);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.clearError);
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
              <Icon className="h-4 w-4 text-muted-foreground" />
              {copy.title}
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              {copy.description}
            </p>
            {documentPath ? (
              <p className="mt-1 break-all font-mono text-[11px] text-muted-foreground">
                {documentPath}
              </p>
            ) : null}
          </div>
          <span className="text-xs text-muted-foreground">
            {lineCount}/200 lines
          </span>
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
            {copy.loading}
          </div>
        ) : (
          <div className="space-y-3">
            <Textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              readOnly={!canWrite}
              className="min-h-56 font-mono text-sm"
              placeholder={copy.placeholder}
            />
            {canWrite ? (
              <div className="flex gap-2">
                <Button
                  onClick={() => void handleSave()}
                  disabled={isSaving}
                  size="sm"
                >
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
        title={copy.clearTitle}
        buttonText="Clear"
        visible={showDeleteConfirm}
        dismissModal={() => setShowDeleteConfirm(false)}
        onSubmit={handleDelete}
      >
        <p>{copy.clearMessage(automataName)}</p>
      </ConfirmModal>
    </>
  );
}
