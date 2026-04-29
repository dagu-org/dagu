import React from 'react';
import {
  Brain,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
  Trash2,
} from 'lucide-react';

import { ControllerDocument, type components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import ConfirmModal from '@/components/ui/confirm-dialog';
import { Textarea } from '@/components/ui/textarea';
import { useCanWrite } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';

type ControllerDocumentResponse =
  components['schemas']['ControllerDocumentResponse'];
type ControllerMemoryReflectionResponse =
  components['schemas']['ControllerMemoryReflectionResponse'];

type ControllerDocumentSectionProps = {
  controllerName: string;
  document: ControllerDocument;
};

const documentCopy: Record<
  ControllerDocument,
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
    clearMessage: (controllerName: string) => string;
  }
> = {
  [ControllerDocument.MEMORY_md]: {
    title: 'Memory',
    description: 'Long-lived notes and reusable lessons for this controller.',
    placeholder:
      'No controller memory yet. Save operating rules, durable context, or learned procedures here.',
    loading: 'Loading memory...',
    saved: 'Controller memory saved',
    cleared: 'Controller memory cleared',
    loadError: 'Failed to load controller memory',
    saveError: 'Failed to save controller memory',
    clearError: 'Failed to clear controller memory',
    clearTitle: 'Clear Controller Memory',
    clearMessage: (controllerName) =>
      `Are you sure you want to clear the memory for "${controllerName}"? This action cannot be undone.`,
  },
  [ControllerDocument.SOUL_md]: {
    title: 'Soul',
    description:
      'Identity, priorities, and communication style for this controller.',
    placeholder:
      'No controller soul yet. Define how this controller should think, speak, and prioritize work.',
    loading: 'Loading soul...',
    saved: 'Controller soul saved',
    cleared: 'Controller soul cleared',
    loadError: 'Failed to load controller soul',
    saveError: 'Failed to save controller soul',
    clearError: 'Failed to clear controller soul',
    clearTitle: 'Clear Controller Soul',
    clearMessage: (controllerName) =>
      `Are you sure you want to clear the soul for "${controllerName}"? This action cannot be undone.`,
  },
};

export function ControllerDocumentSection({
  controllerName,
  document,
}: ControllerDocumentSectionProps): React.ReactElement {
  const client = useClient();
  const canWrite = useCanWrite();
  const copy = documentCopy[document];
  const Icon = document === ControllerDocument.SOUL_md ? Sparkles : Brain;
  const isMemoryDocument = document === ControllerDocument.MEMORY_md;

  const [isLoading, setIsLoading] = React.useState(true);
  const [isSaving, setIsSaving] = React.useState(false);
  const [isDeleting, setIsDeleting] = React.useState(false);
  const [isReflecting, setIsReflecting] = React.useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [success, setSuccess] = React.useState<string | null>(null);
  const [content, setContent] = React.useState('');
  const [documentPath, setDocumentPath] = React.useState('');
  const [reflectionDraft, setReflectionDraft] =
    React.useState<ControllerMemoryReflectionResponse | null>(null);

  const loadDocument = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const { data, error: apiError } = await client.GET(
        '/controller/{name}/documents/{document}',
        {
          params: { path: { name: controllerName, document } },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.loadError);
      }
      const item = data as ControllerDocumentResponse;
      setContent(item.content ?? '');
      setDocumentPath(item.path ?? '');
      setReflectionDraft(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.loadError);
    } finally {
      setIsLoading(false);
    }
  }, [controllerName, client, copy.loadError, document]);

  React.useEffect(() => {
    void loadDocument();
  }, [loadDocument]);

  async function handleSave(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { data, error: apiError } = await client.PUT(
        '/controller/{name}/documents/{document}',
        {
          params: { path: { name: controllerName, document } },
          body: { content },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.saveError);
      }
      const item = data as ControllerDocumentResponse;
      setContent(item.content ?? '');
      setDocumentPath(item.path ?? documentPath);
      setReflectionDraft(null);
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
        '/controller/{name}/documents/{document}',
        {
          params: { path: { name: controllerName, document } },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.clearError);
      }
      setContent('');
      setReflectionDraft(null);
      setShowDeleteConfirm(false);
      setSuccess(copy.cleared);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.clearError);
    } finally {
      setIsDeleting(false);
    }
  }

  async function handleReflect(): Promise<void> {
    if (!isMemoryDocument) {
      return;
    }
    setIsReflecting(true);
    setError(null);
    setSuccess(null);
    try {
      const { data, error: apiError } = await client.POST(
        '/controller/{name}/memory/reflect',
        {
          params: { path: { name: controllerName } },
        }
      );
      if (apiError) {
        throw new Error(
          apiError.message || 'Failed to reflect controller memory'
        );
      }
      const draft = data as ControllerMemoryReflectionResponse;
      setReflectionDraft(draft);
      setContent(draft.proposedContent ?? '');
      setSuccess('Reflection draft generated');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to reflect controller memory'
      );
    } finally {
      setIsReflecting(false);
    }
  }

  function handleRevertReflection(): void {
    if (!reflectionDraft) {
      return;
    }
    setContent(reflectionDraft.currentContent ?? '');
    setReflectionDraft(null);
    setSuccess(null);
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
            {reflectionDraft?.rationale ? (
              <div className="rounded-md bg-muted px-3 py-2 text-sm">
                <div className="mb-1 text-xs font-semibold uppercase text-muted-foreground">
                  Reflection rationale
                </div>
                <p className="whitespace-pre-wrap text-muted-foreground">
                  {reflectionDraft.rationale}
                </p>
              </div>
            ) : null}
            {canWrite ? (
              <div className="flex flex-wrap gap-2">
                <Button
                  onClick={() => void handleSave()}
                  disabled={isSaving || isReflecting}
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
                  disabled={!content || isDeleting || isReflecting}
                >
                  <Trash2 className="mr-1.5 h-4 w-4" />
                  Clear
                </Button>
                {isMemoryDocument ? (
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={() => void handleReflect()}
                    disabled={isReflecting || isSaving || isDeleting}
                  >
                    {isReflecting ? (
                      <>
                        <Loader2 className="mr-1.5 h-4 w-4 animate-spin" />
                        Reflecting...
                      </>
                    ) : (
                      <>
                        <Sparkles className="mr-1.5 h-4 w-4" />
                        Reflect
                      </>
                    )}
                  </Button>
                ) : null}
                {reflectionDraft ? (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleRevertReflection}
                    disabled={isSaving || isReflecting}
                  >
                    <RotateCcw className="mr-1.5 h-4 w-4" />
                    Revert draft
                  </Button>
                ) : null}
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
        <p>{copy.clearMessage(controllerName)}</p>
      </ConfirmModal>
    </>
  );
}
