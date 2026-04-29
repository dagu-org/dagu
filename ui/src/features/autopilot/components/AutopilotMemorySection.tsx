import React from 'react';
import {
  Brain,
  Loader2,
  RotateCcw,
  Save,
  Sparkles,
  Trash2,
} from 'lucide-react';

import { AutopilotDocument, type components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import ConfirmModal from '@/components/ui/confirm-dialog';
import { Textarea } from '@/components/ui/textarea';
import { useCanWrite } from '@/contexts/AuthContext';
import { useClient } from '@/hooks/api';

type AutopilotDocumentResponse =
  components['schemas']['AutopilotDocumentResponse'];
type AutopilotMemoryReflectionResponse =
  components['schemas']['AutopilotMemoryReflectionResponse'];

type AutopilotDocumentSectionProps = {
  autopilotName: string;
  document: AutopilotDocument;
};

const documentCopy: Record<
  AutopilotDocument,
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
    clearMessage: (autopilotName: string) => string;
  }
> = {
  [AutopilotDocument.MEMORY_md]: {
    title: 'Memory',
    description: 'Long-lived notes and reusable lessons for this autopilot.',
    placeholder:
      'No autopilot memory yet. Save operating rules, durable context, or learned procedures here.',
    loading: 'Loading memory...',
    saved: 'Autopilot memory saved',
    cleared: 'Autopilot memory cleared',
    loadError: 'Failed to load autopilot memory',
    saveError: 'Failed to save autopilot memory',
    clearError: 'Failed to clear autopilot memory',
    clearTitle: 'Clear Autopilot Memory',
    clearMessage: (autopilotName) =>
      `Are you sure you want to clear the memory for "${autopilotName}"? This action cannot be undone.`,
  },
  [AutopilotDocument.SOUL_md]: {
    title: 'Soul',
    description:
      'Identity, priorities, and communication style for this autopilot.',
    placeholder:
      'No autopilot soul yet. Define how this autopilot should think, speak, and prioritize work.',
    loading: 'Loading soul...',
    saved: 'Autopilot soul saved',
    cleared: 'Autopilot soul cleared',
    loadError: 'Failed to load autopilot soul',
    saveError: 'Failed to save autopilot soul',
    clearError: 'Failed to clear autopilot soul',
    clearTitle: 'Clear Autopilot Soul',
    clearMessage: (autopilotName) =>
      `Are you sure you want to clear the soul for "${autopilotName}"? This action cannot be undone.`,
  },
};

export function AutopilotDocumentSection({
  autopilotName,
  document,
}: AutopilotDocumentSectionProps): React.ReactElement {
  const client = useClient();
  const canWrite = useCanWrite();
  const copy = documentCopy[document];
  const Icon = document === AutopilotDocument.SOUL_md ? Sparkles : Brain;
  const isMemoryDocument = document === AutopilotDocument.MEMORY_md;

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
    React.useState<AutopilotMemoryReflectionResponse | null>(null);

  const loadDocument = React.useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const { data, error: apiError } = await client.GET(
        '/autopilot/{name}/documents/{document}',
        {
          params: { path: { name: autopilotName, document } },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.loadError);
      }
      const item = data as AutopilotDocumentResponse;
      setContent(item.content ?? '');
      setDocumentPath(item.path ?? '');
      setReflectionDraft(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : copy.loadError);
    } finally {
      setIsLoading(false);
    }
  }, [autopilotName, client, copy.loadError, document]);

  React.useEffect(() => {
    void loadDocument();
  }, [loadDocument]);

  async function handleSave(): Promise<void> {
    setIsSaving(true);
    setError(null);
    setSuccess(null);
    try {
      const { data, error: apiError } = await client.PUT(
        '/autopilot/{name}/documents/{document}',
        {
          params: { path: { name: autopilotName, document } },
          body: { content },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || copy.saveError);
      }
      const item = data as AutopilotDocumentResponse;
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
        '/autopilot/{name}/documents/{document}',
        {
          params: { path: { name: autopilotName, document } },
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
        '/autopilot/{name}/memory/reflect',
        {
          params: { path: { name: autopilotName } },
        }
      );
      if (apiError) {
        throw new Error(
          apiError.message || 'Failed to reflect autopilot memory'
        );
      }
      const draft = data as AutopilotMemoryReflectionResponse;
      setReflectionDraft(draft);
      setContent(draft.proposedContent ?? '');
      setSuccess('Reflection draft generated');
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to reflect autopilot memory'
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
        <p>{copy.clearMessage(autopilotName)}</p>
      </ConfirmModal>
    </>
  );
}
