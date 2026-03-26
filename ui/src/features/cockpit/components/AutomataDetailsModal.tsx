import React from 'react';
import { useNavigate } from 'react-router-dom';
import { Maximize2, X } from 'lucide-react';
import { components, Status } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { whenEnabled } from '@/hooks/queryUtils';
import { useClient, useQuery } from '@/hooks/api';
import { cn } from '@/lib/utils';
import dayjs from '@/lib/dayjs';
import { shouldIgnoreKeyboardShortcuts } from '@/lib/keyboard-shortcuts';
import { Textarea } from '@/components/ui/textarea';
import LoadingIndicator from '@/ui/LoadingIndicator';
import StatusChip from '@/ui/StatusChip';
import DAGRunDetailsModal from '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal';

type AutomataDetail = components['schemas']['AutomataDetailResponse'];
type AgentMessage = components['schemas']['AgentMessage'];
type AutomataRunSummary = components['schemas']['AutomataRunSummary'];

const CLOSE_ANIMATION_MS = 150;

function lifecycleClass(state?: string): string {
  switch (state) {
    case 'running':
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case 'waiting':
      return 'bg-amber-100 text-amber-900 dark:bg-amber-900/40 dark:text-amber-200';
    case 'paused':
      return 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100';
    case 'finished':
      return 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

function dagRunStatusToStatus(status?: string): Status | undefined {
  switch (status) {
    case 'not_started':
      return Status.NotStarted;
    case 'running':
      return Status.Running;
    case 'failed':
      return Status.Failed;
    case 'aborted':
      return Status.Aborted;
    case 'succeeded':
      return Status.Success;
    case 'queued':
      return Status.Queued;
    case 'partially_succeeded':
      return Status.PartialSuccess;
    case 'waiting':
      return Status.Waiting;
    case 'rejected':
      return Status.Rejected;
    default:
      return undefined;
  }
}

function formatAbsoluteTime(value?: string): string {
  if (!value || value === '-') {
    return 'n/a';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('MMM D, YYYY HH:mm') : 'n/a';
}

function formatRelativeTime(value?: string): string {
  if (!value || value === '-') {
    return 'n/a';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.fromNow() : 'n/a';
}

function MessageBlock({ message }: { message: AgentMessage }): React.ReactElement {
  return (
    <div className="min-w-0 rounded-md border p-3 text-sm">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        <span>{message.type}</span>
        <span className="normal-case tracking-normal">
          {formatAbsoluteTime(message.createdAt)}
        </span>
      </div>
      {message.content ? (
        <div className="whitespace-pre-wrap break-words">{message.content}</div>
      ) : null}
      {message.userPrompt?.question ? (
        <div className="whitespace-pre-wrap break-words">{message.userPrompt.question}</div>
      ) : null}
      {message.toolResults?.length ? (
        <div className="mt-2 space-y-2">
          {message.toolResults.map((result, index) => (
            <div
              key={`${message.id}-tool-${index}`}
              className="rounded bg-muted p-2 text-xs whitespace-pre-wrap break-words"
            >
              {result.content}
            </div>
          ))}
        </div>
      ) : null}
    </div>
  );
}

function RunRow({
  run,
  onOpen,
}: {
  run: AutomataRunSummary;
  onOpen: (run: AutomataRunSummary) => void;
}): React.ReactElement {
  return (
    <button
      type="button"
      onClick={() => onOpen(run)}
      className="flex w-full items-center justify-between gap-3 rounded-md border p-3 text-left transition hover:bg-muted/40"
    >
      <div className="min-w-0">
        <div className="truncate text-sm font-medium">{run.name}</div>
        <div className="mt-1 text-xs text-muted-foreground">
          {run.dagRunId} · {formatRelativeTime(run.startedAt || run.createdAt)}
        </div>
      </div>
      <StatusChip status={dagRunStatusToStatus(run.status)} size="xs">
        {run.status}
      </StatusChip>
    </button>
  );
}

export function AutomataDetailsModal({
  name,
  isOpen,
  onClose,
  onUpdated,
}: {
  name: string;
  isOpen: boolean;
  onClose: () => void;
  onUpdated?: () => void | Promise<void>;
}): React.ReactElement | null {
  const client = useClient();
  const navigate = useNavigate();
  const [shouldRender, setShouldRender] = React.useState(isOpen);
  const [isVisible, setIsVisible] = React.useState(false);
  const [selectedRun, setSelectedRun] = React.useState<AutomataRunSummary | null>(null);
  const [instructionDraft, setInstructionDraft] = React.useState('');
  const [operatorMessageDraft, setOperatorMessageDraft] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [actionError, setActionError] = React.useState('');
  const [busyAction, setBusyAction] = React.useState<string | null>(null);
  const stableNameRef = React.useRef(name);

  if (name) {
    stableNameRef.current = name;
  }
  const stableName = isOpen || shouldRender ? stableNameRef.current : '';

  React.useEffect(() => {
    if (isOpen) {
      setShouldRender(true);
      requestAnimationFrame(() => {
        requestAnimationFrame(() => setIsVisible(true));
      });
      return;
    }
    setIsVisible(false);
    const timer = setTimeout(() => {
      setShouldRender(false);
      setSelectedRun(null);
    }, CLOSE_ANIMATION_MS);
    return () => clearTimeout(timer);
  }, [isOpen]);

  React.useEffect(() => {
    if (!isOpen) {
      return;
    }
    function handleKeyDown(event: KeyboardEvent): void {
      if (shouldIgnoreKeyboardShortcuts()) {
        return;
      }
      if (event.key === 'Escape') {
        onClose();
      }
      if (event.key === 'f' || event.key === 'F') {
        navigate(`/automata/${encodeURIComponent(stableName)}`);
      }
    }
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, navigate, onClose, stableName]);

  const { data, error, isLoading, mutate } = useQuery(
    '/automata/{name}',
    whenEnabled(isOpen && !!stableName, {
      params: { path: { name: stableName } },
    }),
    {
      refreshInterval: (detail?: AutomataDetail) =>
        detail?.state?.state === 'running' ||
        detail?.state?.state === 'waiting' ||
        detail?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  React.useEffect(() => {
    setInstructionDraft(data?.state?.instruction || '');
  }, [data?.state?.instruction, stableName]);

  React.useEffect(() => {
    setSelectedOptions([]);
    setFreeTextResponse('');
  }, [data?.state?.pendingPrompt?.id, stableName]);

  const lifecycleState = data?.state?.state ?? '';
  const canStartTask =
    lifecycleState === 'idle' || lifecycleState === 'finished';
  const canSendOperatorMessage =
    !!data &&
    (lifecycleState === 'running' ||
      lifecycleState === 'waiting' ||
      lifecycleState === 'paused') &&
    !data.state.pendingPrompt;
  const canPause = lifecycleState === 'running' || lifecycleState === 'waiting';
  const canResume = lifecycleState === 'paused';

  const refreshAfterAction = React.useCallback(async () => {
    await mutate();
    if (onUpdated) {
      await onUpdated();
    }
  }, [mutate, onUpdated]);

  const onStart = React.useCallback(async () => {
    if (!stableName || !instructionDraft.trim()) return;
    setActionError('');
    setBusyAction('start');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/start', {
        params: { path: { name: stableName } },
        body: { instruction: instructionDraft || undefined },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to start automata');
      }
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to start automata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, instructionDraft, refreshAfterAction, stableName]);

  const onSendOperatorMessage = React.useCallback(async () => {
    if (!stableName || !operatorMessageDraft.trim()) return;
    setActionError('');
    setBusyAction('message');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/message', {
        params: { path: { name: stableName } },
        body: { message: operatorMessageDraft },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to send operator message');
      }
      setOperatorMessageDraft('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to send operator message'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, operatorMessageDraft, refreshAfterAction, stableName]);

  const submitHumanResponse = React.useCallback(
    async (optionIDs: string[], freeText: string) => {
      if (!stableName || !data?.state?.pendingPrompt) return;
      setActionError('');
      setBusyAction('respond');
      try {
        const { error: apiError } = await client.POST('/automata/{name}/response', {
          params: { path: { name: stableName } },
          body: {
            promptId: data.state.pendingPrompt.id,
            selectedOptionIds: optionIDs.length ? optionIDs : undefined,
            freeTextResponse: freeText || undefined,
          },
        });
        if (apiError) {
          throw new Error(apiError.message || 'Failed to respond');
        }
        setSelectedOptions([]);
        setFreeTextResponse('');
        await refreshAfterAction();
      } catch (err) {
        setActionError(err instanceof Error ? err.message : 'Failed to respond');
      } finally {
        setBusyAction(null);
      }
    },
    [client, data?.state?.pendingPrompt, refreshAfterAction, stableName]
  );

  const onPauseResume = React.useCallback(async () => {
    if (!stableName || !data) return;
    const paused = data.state.state === 'paused';
    setActionError('');
    setBusyAction(paused ? 'resume' : 'pause');
    try {
      const response = paused
        ? await client.POST('/automata/{name}/resume', {
            params: { path: { name: stableName } },
          })
        : await client.POST('/automata/{name}/pause', {
            params: { path: { name: stableName } },
          });
      if (response.error) {
        throw new Error(
          response.error.message ||
            (paused ? 'Failed to resume automata' : 'Failed to pause automata')
        );
      }
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : data.state.state === 'paused'
            ? 'Failed to resume automata'
            : 'Failed to pause automata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, data, refreshAfterAction, stableName]);

  if (!shouldRender) {
    return null;
  }

  const modalVisibilityClass = isVisible
    ? 'translate-x-0 opacity-100'
    : 'translate-x-full opacity-0';

  return (
    <>
      <div className="fixed inset-0 z-40 h-screen w-screen bg-black/20" onClick={onClose} />

      <div
        className={cn(
          'fixed top-0 bottom-0 right-0 z-50 h-screen w-full border-l bg-background transition-all duration-150 ease-out md:w-3/4 xl:w-[56rem]',
          modalVisibilityClass
        )}
      >
        <div className="flex h-full min-h-0 flex-col p-4 md:p-6">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="text-xs text-muted-foreground">
                Automata detail
              </div>
              <h2 className="truncate text-2xl font-semibold">{stableName}</h2>
            </div>
            <div className="flex items-center gap-2">
              {canPause || canResume ? (
                <Button
                  variant="outline"
                  size="sm"
                  onClick={onPauseResume}
                  disabled={busyAction === 'pause' || busyAction === 'resume'}
                >
                  {canResume ? 'Resume' : 'Pause'}
                </Button>
              ) : null}
              <Button
                variant="outline"
                size="icon"
                onClick={() => navigate(`/automata/${encodeURIComponent(stableName)}`)}
                title="Open full page (F)"
                className="relative group"
              >
                <Maximize2 className="h-4 w-4" />
                <span className="absolute -bottom-1 -right-1 rounded-sm border bg-muted px-1 text-xs font-medium text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                  F
                </span>
              </Button>
              <Button
                variant="outline"
                size="icon"
                onClick={onClose}
                title="Close (Esc)"
                className="relative group"
              >
                <X className="h-4 w-4" />
                <span className="absolute -bottom-1 -right-1 rounded-sm border bg-muted px-1 text-xs font-medium text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100">
                  Esc
                </span>
              </Button>
            </div>
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto">
            {isLoading && !data ? (
              <div className="flex h-full items-center justify-center">
                <LoadingIndicator />
              </div>
            ) : error && !data ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
                {error instanceof Error ? error.message : 'Failed to load Automata details'}
              </div>
            ) : data ? (
              <div className="space-y-6">
                {actionError ? (
                  <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-4 text-sm text-destructive">
                    {actionError}
                  </div>
                ) : null}
                <div className="flex flex-wrap items-center gap-2">
                  <span
                    className={cn(
                      'rounded-full px-3 py-1 text-xs font-medium',
                      lifecycleClass(data.state.state)
                    )}
                  >
                    {data.state.state}
                  </span>
                  {data.state.currentStage ? (
                    <span className="rounded-full border px-3 py-1 text-xs text-muted-foreground">
                      Stage: {data.state.currentStage}
                    </span>
                  ) : null}
                  {data.definition.disabled ? (
                    <span className="rounded-full border px-3 py-1 text-xs text-muted-foreground">
                      disabled
                    </span>
                  ) : null}
                </div>

                <div className="grid gap-4 lg:grid-cols-2">
                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">Mission</h3>
                    <div className="space-y-2 text-sm">
                      {data.definition.description ? (
                        <p className="text-muted-foreground">{data.definition.description}</p>
                      ) : null}
                      <p>
                        <span className="font-medium">Goal:</span>{' '}
                        {data.definition.goal || 'n/a'}
                      </p>
                      <p>
                        <span className="font-medium">Instruction:</span>{' '}
                        {data.state.instruction || 'No active instruction'}
                      </p>
                    </div>
                  </div>

                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">Task State</h3>
                    <div className="space-y-2 text-sm">
                      <p>
                        <span className="font-medium">Last updated:</span>{' '}
                        {formatAbsoluteTime(data.state.lastUpdatedAt)}
                      </p>
                      {data.state.waitingReason ? (
                        <p>
                          <span className="font-medium">Waiting reason:</span>{' '}
                          {data.state.waitingReason}
                        </p>
                      ) : null}
                      {data.state.lastSummary ? (
                        <p className="whitespace-pre-wrap break-words">
                          <span className="font-medium">Summary:</span>{' '}
                          {data.state.lastSummary}
                        </p>
                      ) : null}
                      {data.state.lastError ? (
                        <p className="whitespace-pre-wrap break-words text-destructive">
                          <span className="font-medium">Error:</span>{' '}
                          {data.state.lastError}
                        </p>
                      ) : null}
                    </div>
                  </div>
                </div>

                <div className="min-w-0 rounded-lg border p-4">
                  <div className="mb-3 flex items-center justify-between gap-3">
                    <h3 className="text-sm font-semibold">Session Messages</h3>
                    {data.state.sessionId ? (
                      <span className="text-[11px] text-muted-foreground">
                        Session: {data.state.sessionId}
                      </span>
                    ) : null}
                  </div>
                  {data.messages?.length ? (
                    <div className="max-h-[30rem] space-y-3 overflow-y-auto">
                      {data.messages.slice(-20).map((message) => (
                        <MessageBlock key={message.id} message={message} />
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      No session messages yet.
                    </div>
                  )}
                </div>

                <div className="grid gap-4 lg:grid-cols-2">
                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">
                      {data.state.state === 'finished'
                        ? 'Start New Task'
                        : 'Start Instruction'}
                    </h3>
                    <div className="space-y-3">
                      <Textarea
                        value={instructionDraft}
                        onChange={(e) => setInstructionDraft(e.target.value)}
                        placeholder="Tell this Automata what task to work on before starting it."
                        disabled={!canStartTask || !!busyAction}
                      />
                      <div className="flex items-center justify-between gap-3">
                        <div className="text-xs text-muted-foreground">
                          {canStartTask
                            ? 'Automata stays idle until an instruction is provided.'
                            : data.state.state === 'paused'
                              ? 'This Automata is paused. Resume it to continue the current task.'
                              : 'This Automata already has an active task. Use an operator message to steer it.'}
                        </div>
                        <Button
                          onClick={onStart}
                          disabled={
                            !instructionDraft.trim() ||
                            !canStartTask ||
                            !!busyAction
                          }
                        >
                          {busyAction === 'start'
                            ? 'Starting...'
                            : data.state.state === 'finished'
                              ? 'Start New Task'
                              : 'Start'}
                        </Button>
                      </div>
                    </div>
                  </div>

                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">Operator Message</h3>
                    <div className="space-y-3">
                      <Textarea
                        value={operatorMessageDraft}
                        onChange={(e) => setOperatorMessageDraft(e.target.value)}
                        placeholder="Add context, change priority, or clarify the current task."
                        disabled={!canSendOperatorMessage || !!busyAction}
                      />
                      <div className="flex items-center justify-between gap-3">
                        <div className="text-xs text-muted-foreground">
                          {data.state.pendingPrompt
                            ? 'Respond to the pending prompt before sending a general operator message.'
                            : canSendOperatorMessage
                              ? data.state.state === 'paused'
                                ? 'This records your message now, but the Automata will stay paused until you resume it.'
                                : data.state.currentRunRef
                                  ? 'This records your message now and the Automata will pick it up after the current child DAG changes state.'
                                  : 'This queues a user message into the active Automata task.'
                              : 'Operator messages are only accepted while the Automata has an active task.'}
                        </div>
                        <Button
                          variant="outline"
                          onClick={onSendOperatorMessage}
                          disabled={
                            !operatorMessageDraft.trim() ||
                            !canSendOperatorMessage ||
                            !!busyAction
                          }
                        >
                          {busyAction === 'message' ? 'Sending...' : 'Send Message'}
                        </Button>
                      </div>
                    </div>
                  </div>
                </div>

                {data.state.pendingPrompt ? (
                  <div className="rounded-lg border border-amber-400/40 bg-amber-50/50 p-4 dark:bg-amber-950/20">
                    <h3 className="mb-2 text-sm font-semibold">
                      {data.state.pendingStageTransition
                        ? 'Stage Change Approval'
                        : 'Waiting For Human Input'}
                    </h3>
                    {data.state.pendingStageTransition ? (
                      <div className="mb-3 space-y-2 text-sm">
                        <p>{data.state.pendingPrompt.question}</p>
                        <div className="grid gap-2 md:grid-cols-2">
                          <div className="rounded-md border bg-background/70 p-3">
                            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              Current Stage
                            </div>
                            <div className="mt-1 font-medium">
                              {data.state.currentStage || 'n/a'}
                            </div>
                          </div>
                          <div className="rounded-md border bg-background/70 p-3">
                            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              Requested Stage
                            </div>
                            <div className="mt-1 font-medium">
                              {data.state.pendingStageTransition.requestedStage}
                            </div>
                          </div>
                        </div>
                        {data.state.pendingStageTransition.note ? (
                          <div className="rounded-md border bg-background/70 p-3">
                            <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                              Agent Note
                            </div>
                            <div className="mt-1 whitespace-pre-wrap break-words">
                              {data.state.pendingStageTransition.note}
                            </div>
                          </div>
                        ) : null}
                      </div>
                    ) : (
                      <p className="mb-3 text-sm">{data.state.pendingPrompt.question}</p>
                    )}
                    <div className="space-y-2">
                      {data.state.state === 'paused' ? (
                        <div className="rounded-md border border-slate-300/60 bg-slate-100/70 px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-200">
                          Response will be queued, and the Automata will remain paused until you resume it.
                        </div>
                      ) : null}
                      {data.state.pendingStageTransition ? (
                        <div className="flex flex-wrap gap-2">
                          <Button
                            onClick={() => void submitHumanResponse(['approve'], '')}
                            disabled={!!busyAction}
                          >
                            {busyAction === 'respond'
                              ? 'Submitting...'
                              : 'Approve Stage Change'}
                          </Button>
                          <Button
                            variant="outline"
                            onClick={() => void submitHumanResponse(['reject'], '')}
                            disabled={!!busyAction}
                          >
                            Reject Stage Change
                          </Button>
                        </div>
                      ) : (
                        <>
                          {(data.state.pendingPrompt.options || []).map((option) => {
                            const selected = selectedOptions.includes(option.id);
                            return (
                              <label
                                key={option.id}
                                className="flex cursor-pointer items-start gap-2 rounded-md border p-2 text-sm"
                              >
                                <input
                                  type="checkbox"
                                  checked={selected}
                                  onChange={(e) => {
                                    setSelectedOptions((prev) =>
                                      e.target.checked
                                        ? [...prev, option.id]
                                        : prev.filter((id) => id !== option.id)
                                    );
                                  }}
                                />
                                <span>
                                  <span className="font-medium">{option.label}</span>
                                  {option.description ? (
                                    <span className="block text-xs text-muted-foreground">
                                      {option.description}
                                    </span>
                                  ) : null}
                                </span>
                              </label>
                            );
                          })}
                          <Textarea
                            value={freeTextResponse}
                            onChange={(e) => setFreeTextResponse(e.target.value)}
                            placeholder={
                              data.state.pendingPrompt.freeTextPlaceholder ||
                              'Add an optional note or free-text response'
                            }
                            disabled={!!busyAction}
                          />
                          <Button
                            onClick={() =>
                              void submitHumanResponse(selectedOptions, freeTextResponse)
                            }
                            disabled={!!busyAction}
                          >
                            {busyAction === 'respond' ? 'Submitting...' : 'Submit Response'}
                          </Button>
                        </>
                      )}
                    </div>
                  </div>
                ) : null}

                <div className="grid gap-4 lg:grid-cols-2">
                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">Current Stage DAGs</h3>
                    {data.allowedDags.length ? (
                      <div className="flex flex-wrap gap-2">
                        {data.allowedDags.map((dag) => (
                          <span
                            key={dag.name}
                            className="rounded-full border px-3 py-1 text-xs"
                          >
                            {dag.name}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <div className="text-sm text-muted-foreground">
                        No DAGs allowed in the current stage.
                      </div>
                    )}
                  </div>

                  <div className="min-w-0 rounded-lg border p-4">
                    <h3 className="mb-3 text-sm font-semibold">Current Child DAG</h3>
                    {data.currentRun ? (
                      <RunRow run={data.currentRun} onOpen={setSelectedRun} />
                    ) : (
                      <div className="text-sm text-muted-foreground">
                        No active child DAG run.
                      </div>
                    )}
                  </div>
                </div>

                <div className="min-w-0 rounded-lg border p-4">
                  <h3 className="mb-3 text-sm font-semibold">Recent Runs</h3>
                  {data.recentRuns?.length ? (
                    <div className="space-y-2">
                      {data.recentRuns.slice(0, 8).map((run) => (
                        <RunRow
                          key={`${run.name}-${run.dagRunId}`}
                          run={run}
                          onOpen={setSelectedRun}
                        />
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      No recent child DAG runs yet.
                    </div>
                  )}
                </div>
              </div>
            ) : null}
          </div>
        </div>
      </div>

      {selectedRun ? (
        <DAGRunDetailsModal
          name={selectedRun.name}
          dagRunId={selectedRun.dagRunId}
          isOpen={!!selectedRun}
          onClose={() => setSelectedRun(null)}
        />
      ) : null}
    </>
  );
}
