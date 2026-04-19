/**
 * DAGSpecReadOnly component displays a DAG-run specification snapshot.
 * Root DAG-run snapshots can be edited locally and retried as a new run.
 *
 * @module features/dags/components/dag-editor
 */
import React from 'react';
import { Button } from '@/components/ui/button';
import { useErrorModal } from '@/components/ui/error-modal';
import { useSimpleToast } from '@/components/ui/simple-toast';
import { cn } from '@/lib/utils';
import { RefreshCw } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { AppBarContext } from '../../../../contexts/AppBarContext';
import { useClient, useQuery } from '../../../../hooks/api';
import ConfirmModal from '../../../../ui/ConfirmModal';
import DAGEditorWithDocs from './DAGEditorWithDocs';

/**
 * Props for the DAGSpecReadOnly component
 */
type DAGSpecReadOnlyProps = {
  /** DAG name to fetch the spec for */
  dagName: string;
  /** DAG run ID */
  dagRunId: string;
  /** Optional sub-DAG run ID for viewing subdag specs */
  subDAGRunId?: string;
  /** Additional class name for the container */
  className?: string;
};

type EditRetryPreview = {
  dagName: string;
  skippedSteps: string[];
  runnableSteps: string[];
  ineligibleSteps: { stepName: string; reason: string }[];
  errors: string[];
  warnings: string[];
};

const normalizeEditRetryPreview = (
  preview: EditRetryPreview | null | undefined
): EditRetryPreview | null => {
  if (!preview) {
    return null;
  }
  return {
    dagName: preview.dagName ?? '',
    skippedSteps: preview.skippedSteps ?? [],
    runnableSteps: preview.runnableSteps ?? [],
    ineligibleSteps: preview.ineligibleSteps ?? [],
    errors: preview.errors ?? [],
    warnings: preview.warnings ?? [],
  };
};

function PreviewStepList({ label, steps }: { label: string; steps: string[] }) {
  if (steps.length === 0) {
    return null;
  }

  return (
    <div className="space-y-1">
      <div className="text-xs font-medium uppercase text-muted-foreground">
        {label}
      </div>
      <div className="max-h-32 overflow-y-auto rounded-md border bg-muted/20 p-2">
        <div className="flex flex-wrap gap-1.5">
          {steps.map((step) => (
            <span
              key={step}
              className="rounded border bg-background px-1.5 py-0.5 font-mono text-xs"
            >
              {step}
            </span>
          ))}
        </div>
      </div>
    </div>
  );
}

/**
 * Skeleton placeholder for the editor while loading
 */
function EditorSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn(
        'flex flex-col bg-surface border border-border rounded-lg overflow-hidden min-h-[300px] h-[70vh]',
        className
      )}
    >
      <div className="flex-shrink-0 flex justify-between items-center p-2 border-b border-border">
        <div className="h-6 w-16 bg-muted animate-pulse rounded" />
      </div>
      <div className="flex-1 p-4 space-y-2">
        <div className="h-4 w-3/4 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/2 bg-muted animate-pulse rounded" />
        <div className="h-4 w-2/3 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/3 bg-muted animate-pulse rounded" />
        <div className="h-4 w-3/4 bg-muted animate-pulse rounded" />
        <div className="h-4 w-1/2 bg-muted animate-pulse rounded" />
      </div>
    </div>
  );
}

/**
 * DAGSpecReadOnly fetches and displays a DAG specification in readonly mode
 * with the Schema Documentation sidebar available for reference.
 */
function DAGSpecReadOnly({
  dagName,
  dagRunId,
  subDAGRunId,
  className,
}: DAGSpecReadOnlyProps) {
  const appBarContext = React.useContext(AppBarContext);
  const client = useClient();
  const navigate = useNavigate();
  const { showError } = useErrorModal();
  const { showToast } = useSimpleToast();
  const [sourceSpec, setSourceSpec] = React.useState('');
  const [editedSpec, setEditedSpec] = React.useState('');
  const [previewLoading, setPreviewLoading] = React.useState(false);
  const [retrySubmitting, setRetrySubmitting] = React.useState(false);
  const [previewVisible, setPreviewVisible] = React.useState(false);
  const [retryPreview, setRetryPreview] =
    React.useState<EditRetryPreview | null>(null);

  // Select endpoint based on whether this is a subdag
  const endpoint = subDAGRunId
    ? ('/dag-runs/{name}/{dagRunId}/sub-dag-runs/{subDAGRunId}/spec' as const)
    : ('/dag-runs/{name}/{dagRunId}/spec' as const);

  // Build path params conditionally
  const pathParams = subDAGRunId
    ? { name: dagName, dagRunId: dagRunId, subDAGRunId: subDAGRunId }
    : { name: dagName, dagRunId: dagRunId };

  // Fetch DAG specification data using the appropriate endpoint
  const { data, isLoading, error } = useQuery(endpoint, {
    params: {
      query: {
        remoteNode: appBarContext.selectedRemoteNode || 'local',
      },
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      path: pathParams as any,
    },
  });

  React.useEffect(() => {
    if (!data?.spec) {
      setSourceSpec('');
      setEditedSpec('');
      setRetryPreview(null);
      setPreviewVisible(false);
      return;
    }
    setSourceSpec(data.spec);
    setEditedSpec(data.spec);
    setRetryPreview(null);
    setPreviewVisible(false);
  }, [data?.spec]);

  const isEditableRetry = !subDAGRunId;
  const hasLoadedSpec = sourceSpec !== '';
  const editorSpec = !hasLoadedSpec && data?.spec ? data.spec : editedSpec;
  const hasEdits =
    isEditableRetry && hasLoadedSpec && editorSpec !== sourceSpec;

  const previewEditedSpec = React.useCallback(async () => {
    if (!hasEdits || !editorSpec.trim() || previewLoading || retrySubmitting) {
      return;
    }
    setPreviewLoading(true);
    setRetryPreview(null);
    try {
      const { data: previewData, error: previewError } = await client.POST(
        '/dag-runs/{name}/{dagRunId}/edit-retry/preview',
        {
          params: {
            path: {
              name: dagName,
              dagRunId,
            },
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
          },
          body: {
            spec: editorSpec,
            dagName,
            persistSpec: false,
          },
        }
      );
      if (previewError) {
        showError(
          previewError.message || 'Failed to preview edited retry',
          'Check the edited YAML and try again.'
        );
        return;
      }
      const preview = normalizeEditRetryPreview(previewData);
      if (!preview) {
        showError(
          'Failed to preview edited retry',
          'The server did not return a retry preview.'
        );
        return;
      }
      setRetryPreview(preview);
      setPreviewVisible(true);
    } catch (err) {
      showError(
        err instanceof Error && err.message
          ? err.message
          : 'Failed to preview edited retry',
        'Check your connection and try again.'
      );
    } finally {
      setPreviewLoading(false);
    }
  }, [
    appBarContext.selectedRemoteNode,
    client,
    dagName,
    dagRunId,
    editorSpec,
    hasEdits,
    previewLoading,
    retrySubmitting,
    showError,
  ]);

  const submitEditedRetry = React.useCallback(async () => {
    if (
      !retryPreview ||
      retryPreview.errors.length > 0 ||
      !editorSpec.trim() ||
      retrySubmitting
    ) {
      return;
    }

    setRetrySubmitting(true);
    try {
      const { data: retryData, error: retryError } = await client.POST(
        '/dag-runs/{name}/{dagRunId}/edit-retry',
        {
          params: {
            path: {
              name: dagName,
              dagRunId,
            },
            query: {
              remoteNode: appBarContext.selectedRemoteNode || 'local',
            },
          },
          body: {
            spec: editorSpec,
            dagName,
            persistSpec: false,
            skipSteps: retryPreview.skippedSteps,
          },
        }
      );
      if (retryError) {
        showError(
          retryError.message || 'Failed to retry edited DAG',
          'Check the edited YAML and preview, then try again.'
        );
        return;
      }
      if (!retryData?.dagRunId) {
        showError(
          'Failed to retry edited DAG',
          'The server did not return a new DAG-run ID.'
        );
        return;
      }
      setPreviewVisible(false);
      showToast(`New DAG run created: ${retryData.dagRunId}`);
      navigate(
        `/dag-runs/${encodeURIComponent(dagName)}/${encodeURIComponent(
          retryData.dagRunId
        )}`
      );
    } catch (err) {
      showError(
        err instanceof Error && err.message
          ? err.message
          : 'Failed to retry edited DAG',
        'Check your connection and try again.'
      );
    } finally {
      setRetrySubmitting(false);
    }
  }, [
    appBarContext.selectedRemoteNode,
    client,
    dagName,
    dagRunId,
    editorSpec,
    navigate,
    retryPreview,
    retrySubmitting,
    showError,
    showToast,
  ]);

  const handleSpecChange = React.useCallback((value?: string) => {
    setEditedSpec(value ?? '');
    setRetryPreview(null);
  }, []);

  if (isLoading) {
    return <EditorSkeleton className={className} />;
  }

  if (error) {
    return (
      <div className="text-sm text-danger p-4">
        Failed to load DAG spec: {error.message ?? 'Unknown error'}
      </div>
    );
  }

  if (!data?.spec) {
    return (
      <div className="text-sm text-muted-foreground p-4">
        No DAG spec available for this DAG.
      </div>
    );
  }

  return (
    <>
      <DAGEditorWithDocs
        value={editorSpec}
        onChange={handleSpecChange}
        readOnly={!isEditableRetry}
        className={className}
        headerActions={
          hasEdits ? (
            <Button
              type="button"
              size="xs"
              variant="primary"
              disabled={previewLoading || retrySubmitting || !editorSpec.trim()}
              onClick={() => void previewEditedSpec()}
            >
              <RefreshCw className="h-3.5 w-3.5" />
              {previewLoading ? 'Previewing...' : 'Retry as a new run'}
            </Button>
          ) : undefined
        }
        modelUri={`inmemory://dagu/dag-runs/${encodeURIComponent(
          dagName
        )}/${encodeURIComponent(dagRunId)}/${encodeURIComponent(
          subDAGRunId ?? 'root'
        )}.yaml`}
      />

      <ConfirmModal
        title="Retry Edited DAG Run"
        buttonText={retrySubmitting ? 'Creating...' : 'Create new run'}
        visible={previewVisible}
        dismissModal={() => {
          if (!retrySubmitting) {
            setPreviewVisible(false);
          }
        }}
        onSubmit={() => void submitEditedRetry()}
        submitDisabled={
          retrySubmitting || !retryPreview || retryPreview.errors.length > 0
        }
        contentClassName="sm:max-w-[680px]"
        bodyClassName="max-h-[70vh] overflow-y-auto pr-1"
      >
        <div className="space-y-3 text-sm">
          <p>
            Review the server preview before creating a new run from this edited
            DAG spec.
          </p>

          {retryPreview && (
            <>
              <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
                <div>
                  <div className="text-xs uppercase text-muted-foreground">
                    Source DAG-run ID
                  </div>
                  <div className="font-mono text-xs">{dagRunId}</div>
                </div>
                <div>
                  <div className="text-xs uppercase text-muted-foreground">
                    Target DAG
                  </div>
                  <div className="font-mono text-xs">
                    {retryPreview.dagName}
                  </div>
                </div>
                <div>
                  <div className="text-xs uppercase text-muted-foreground">
                    Skipped steps
                  </div>
                  <div>{retryPreview.skippedSteps.length}</div>
                </div>
                <div>
                  <div className="text-xs uppercase text-muted-foreground">
                    Runnable steps
                  </div>
                  <div>{retryPreview.runnableSteps.length}</div>
                </div>
              </div>

              {retryPreview.errors.length > 0 && (
                <div className="space-y-1 rounded-md border border-destructive/40 bg-destructive/5 p-2 text-destructive">
                  {retryPreview.errors.map((error) => (
                    <div key={error}>{error}</div>
                  ))}
                </div>
              )}

              {retryPreview.warnings.length > 0 && (
                <div className="space-y-1 rounded-md border p-2 text-muted-foreground">
                  {retryPreview.warnings.map((warning) => (
                    <div key={warning}>{warning}</div>
                  ))}
                </div>
              )}

              <PreviewStepList
                label="Skipped steps"
                steps={retryPreview.skippedSteps}
              />
              <PreviewStepList
                label="Runnable steps"
                steps={retryPreview.runnableSteps}
              />

              {retryPreview.ineligibleSteps.length > 0 && (
                <div className="space-y-1">
                  <div className="text-xs font-medium uppercase text-muted-foreground">
                    Not eligible to skip
                  </div>
                  <div className="max-h-32 overflow-y-auto rounded-md border bg-muted/20 p-2">
                    {retryPreview.ineligibleSteps.map((step) => (
                      <div key={step.stepName} className="text-xs">
                        <span className="font-mono">{step.stepName}</span>:{' '}
                        {step.reason}
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </>
          )}
        </div>
      </ConfirmModal>
    </>
  );
}

export default DAGSpecReadOnly;
