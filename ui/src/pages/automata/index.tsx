import React from 'react';
import { Link, useNavigate, useParams } from 'react-router-dom';
import useSWR from 'swr';

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
import { Textarea } from '@/components/ui/textarea';
import { AppBarContext } from '@/contexts/AppBarContext';
import fetchJson from '@/lib/fetchJson';
import LoadingIndicator from '@/ui/LoadingIndicator';

declare const getConfig: () => { apiURL: string };

type AutomataSummary = {
  name: string;
  description?: string;
  purpose: string;
  goal: string;
  instruction?: string;
  state: string;
  stage?: string;
  disabled?: boolean;
  lastUpdatedAt?: string;
  currentRun?: {
    name: string;
    dagRunId: string;
    status: string;
  };
};

type AutomataDetail = {
  definition: {
    name: string;
    description?: string;
    purpose: string;
    goal: string;
    stages: string[];
    disabled?: boolean;
  };
  state: {
    state: string;
    instruction?: string;
    currentStage?: string;
    waitingReason?: string;
    pendingPrompt?: {
      id: string;
      question: string;
      options?: { id: string; label: string; description?: string }[];
      allowFreeText?: boolean;
      freeTextPlaceholder?: string;
    };
    sessionId?: string;
    currentRunRef?: { name: string; id: string };
    lastSummary?: string;
    lastError?: string;
  };
  allowedDags: {
    name: string;
    description?: string;
    tags?: string[];
  }[];
  currentRun?: {
    name: string;
    dagRunId: string;
    status: string;
  };
  recentRuns?: {
    name: string;
    dagRunId: string;
    status: string;
    startedAt?: string;
    finishedAt?: string;
    error?: string;
  }[];
  messages?: {
    id: string;
    type: string;
    content?: string;
    created_at?: string;
    user_prompt?: {
      question: string;
    };
    tool_results?: { content: string; is_error?: boolean }[];
  }[];
};

const AUTOMATA_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_.-]*$/;
const DEFAULT_STAGE_LINES = ['research', 'plan', 'implement', 'review'].join(
  '\n'
);

async function sendJSON(
  path: string,
  method: string,
  body?: unknown
): Promise<void> {
  const token = localStorage.getItem('dagu_auth_token');
  const response = await fetch(`${getConfig().apiURL}${path}`, {
    method,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body ? JSON.stringify(body) : undefined,
  });

  if (!response.ok) {
    let message = response.statusText;
    try {
      const data = await response.json();
      message = data?.message || message;
    } catch {
      // keep status text
    }
    throw new Error(message);
  }
}

function parseLineList(value: string): string[] {
  return value
    .split('\n')
    .map((line) => line.trim())
    .filter(Boolean);
}

function quoteYAML(value: string): string {
  return JSON.stringify(value.trim());
}

function buildAutomataSpec(input: {
  description: string;
  purpose: string;
  goal: string;
  stageLines: string;
  allowedDAGLines: string;
}): string {
  const stages = parseLineList(input.stageLines);
  const allowedDAGs = parseLineList(input.allowedDAGLines);
  return [
    `description: ${quoteYAML(input.description || 'Automata workflow')}`,
    `purpose: ${quoteYAML(input.purpose)}`,
    `goal: ${quoteYAML(input.goal)}`,
    '',
    'stages:',
    ...stages.map((stage) => `  - ${quoteYAML(stage)}`),
    '',
    'allowedDAGs:',
    '  names:',
    ...allowedDAGs.map((dagName) => `    - ${quoteYAML(dagName)}`),
    '',
    'agent:',
    '  safeMode: true',
    '',
  ].join('\n');
}

function validateAutomataCreateForm(input: {
  name: string;
  purpose: string;
  goal: string;
  stageLines: string;
  allowedDAGLines: string;
}): string | null {
  const name = input.name.trim();
  if (!name) {
    return 'Automata name is required.';
  }
  if (!AUTOMATA_NAME_PATTERN.test(name)) {
    return 'Automata name must start with a letter or number and use only letters, numbers, underscores, dots, and hyphens.';
  }
  if (!input.purpose.trim()) {
    return 'Purpose is required.';
  }
  if (!input.goal.trim()) {
    return 'Goal is required.';
  }
  if (parseLineList(input.stageLines).length === 0) {
    return 'At least one stage is required.';
  }
  if (parseLineList(input.allowedDAGLines).length === 0) {
    return 'At least one allowed DAG name is required.';
  }
  return null;
}

function statusClass(state: string): string {
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

function AutomataPage(): React.ReactElement {
  const appBar = React.useContext(AppBarContext);
  const navigate = useNavigate();
  const { name } = useParams();

  const [showCreateDialog, setShowCreateDialog] = React.useState(false);
  const [createName, setCreateName] = React.useState('');
  const [createDescription, setCreateDescription] = React.useState('');
  const [createPurpose, setCreatePurpose] = React.useState('');
  const [createGoal, setCreateGoal] = React.useState('');
  const [createStages, setCreateStages] = React.useState(DEFAULT_STAGE_LINES);
  const [createAllowedDAGs, setCreateAllowedDAGs] = React.useState('');
  const [createError, setCreateError] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);

  const [instructionDraft, setInstructionDraft] = React.useState('');
  const [operatorMessageDraft, setOperatorMessageDraft] = React.useState('');
  const [stageOverride, setStageOverride] = React.useState('');
  const [stageNote, setStageNote] = React.useState('');
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [error, setError] = React.useState('');

  const [isEditingSpec, setIsEditingSpec] = React.useState(false);
  const [specDraft, setSpecDraft] = React.useState('');
  const [specError, setSpecError] = React.useState('');
  const [isSavingSpec, setIsSavingSpec] = React.useState(false);

  React.useEffect(() => {
    appBar.setTitle('Automata');
  }, [appBar]);

  const listQuery = useSWR<{ automata: AutomataSummary[] }>(
    '/automata',
    fetchJson,
    { refreshInterval: 15000 }
  );

  const detailQuery = useSWR<AutomataDetail>(
    name ? `/automata/${encodeURIComponent(name)}` : null,
    fetchJson,
    {
      refreshInterval: (data) =>
        data?.state?.state === 'running' ||
        data?.state?.state === 'waiting' ||
        data?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useSWR<{ spec: string }>(
    name ? `/automata/${encodeURIComponent(name)}/spec` : null,
    fetchJson,
    { refreshInterval: 15000 }
  );

  const detail = detailQuery.data;
  const lifecycleState = detail?.state?.state ?? '';
  const canStartTask =
    lifecycleState === 'idle' || lifecycleState === 'finished';
  const canSendOperatorMessage =
    !!detail &&
    (lifecycleState === 'running' ||
      lifecycleState === 'waiting' ||
      lifecycleState === 'paused') &&
    !detail.state.pendingPrompt;
  const canPause = lifecycleState === 'running' || lifecycleState === 'waiting';
  const canResume = lifecycleState === 'paused';
  const scheduleConfigured = /(^|\n)schedule\s*:/.test(
    specQuery.data?.spec || ''
  );

  React.useEffect(() => {
    if (detail?.definition?.stages?.length && !stageOverride) {
      const initialStage =
        detail.state?.currentStage ?? detail.definition.stages[0] ?? '';
      if (initialStage) {
        setStageOverride(initialStage);
      }
    }
  }, [detail?.definition?.stages, detail?.state?.currentStage, stageOverride]);

  React.useEffect(() => {
    setInstructionDraft(detail?.state?.instruction || '');
  }, [detail?.state?.instruction, name]);

  React.useEffect(() => {
    setOperatorMessageDraft('');
  }, [name]);

  React.useEffect(() => {
    if (!isEditingSpec) {
      setSpecDraft(specQuery.data?.spec || '');
    }
  }, [isEditingSpec, specQuery.data?.spec]);

  React.useEffect(() => {
    setIsEditingSpec(false);
    setSpecError('');
  }, [name]);

  const resetCreateForm = () => {
    setCreateName('');
    setCreateDescription('');
    setCreatePurpose('');
    setCreateGoal('');
    setCreateStages(DEFAULT_STAGE_LINES);
    setCreateAllowedDAGs('');
    setCreateError('');
    setIsCreating(false);
  };

  const openCreateDialog = () => {
    resetCreateForm();
    setShowCreateDialog(true);
  };

  const closeCreateDialog = () => {
    if (isCreating) {
      return;
    }
    setShowCreateDialog(false);
    resetCreateForm();
  };

  const onCreate = async () => {
    const validationError = validateAutomataCreateForm({
      name: createName,
      purpose: createPurpose,
      goal: createGoal,
      stageLines: createStages,
      allowedDAGLines: createAllowedDAGs,
    });
    if (validationError) {
      setCreateError(validationError);
      return;
    }

    const automataName = createName.trim();
    setCreateError('');
    setIsCreating(true);

    try {
      await sendJSON(
        `/automata/${encodeURIComponent(automataName)}/spec`,
        'PUT',
        {
          spec: buildAutomataSpec({
            description: createDescription,
            purpose: createPurpose,
            goal: createGoal,
            stageLines: createStages,
            allowedDAGLines: createAllowedDAGs,
          }),
        }
      );
      await listQuery.mutate();
      setShowCreateDialog(false);
      resetCreateForm();
      navigate(`/automata/${encodeURIComponent(automataName)}`);
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : 'Failed to create automata'
      );
      setIsCreating(false);
    }
  };

  const onStart = async () => {
    if (!name) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/start`, 'POST', {
        instruction: instructionDraft,
      });
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start automata');
    }
  };

  const onOverrideStage = async () => {
    if (!name || !stageOverride) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/stage`, 'POST', {
        stage: stageOverride,
        note: stageNote,
      });
      setStageNote('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to update stage');
    }
  };

  const onRespond = async () => {
    if (!name || !detail?.state?.pendingPrompt) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/response`, 'POST', {
        promptId: detail.state.pendingPrompt.id,
        selectedOptionIds: selectedOptions,
        freeTextResponse,
      });
      setSelectedOptions([]);
      setFreeTextResponse('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to respond');
    }
  };

  const onSendOperatorMessage = async () => {
    if (!name || !operatorMessageDraft.trim()) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/message`, 'POST', {
        message: operatorMessageDraft,
      });
      setOperatorMessageDraft('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to send operator message'
      );
    }
  };

  const onPauseResume = async () => {
    if (!name || !detail) return;
    setError('');
    const paused = detail.state.state === 'paused';
    try {
      await sendJSON(
        `/automata/${encodeURIComponent(name)}/${paused ? 'resume' : 'pause'}`,
        'POST'
      );
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : paused
            ? 'Failed to resume automata'
            : 'Failed to pause automata'
      );
    }
  };

  const onSaveSpec = async () => {
    if (!name) return;
    setSpecError('');
    setIsSavingSpec(true);
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/spec`, 'PUT', {
        spec: specDraft,
      });
      await Promise.all([
        detailQuery.mutate(),
        listQuery.mutate(),
        specQuery.mutate(),
      ]);
      setIsEditingSpec(false);
    } catch (err) {
      setSpecError(err instanceof Error ? err.message : 'Failed to save spec');
    } finally {
      setIsSavingSpec(false);
    }
  };

  return (
    <>
      <Dialog
        open={showCreateDialog}
        onOpenChange={(open) => {
          if (open) {
            setShowCreateDialog(true);
            return;
          }
          closeCreateDialog();
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-2xl">
          <DialogHeader>
            <DialogTitle>Create Automata</DialogTitle>
            <DialogDescription>
              Define the Automata goal, stages, and initial allowlisted DAGs.
              You can refine the raw spec after creation.
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-4">
            <div className="grid gap-2">
              <Label htmlFor="automata-name">Name</Label>
              <Input
                id="automata-name"
                value={createName}
                onChange={(e) => setCreateName(e.target.value)}
                placeholder="software-dev"
                autoFocus
                disabled={isCreating}
              />
              <div className="text-xs text-muted-foreground">
                Must start with a letter or number. Use letters, numbers,
                underscores, dots, and hyphens only.
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="automata-description">Description</Label>
              <Input
                id="automata-description"
                value={createDescription}
                onChange={(e) => setCreateDescription(e.target.value)}
                placeholder="Automates one software delivery workflow"
                disabled={isCreating}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="automata-purpose">Purpose</Label>
              <Textarea
                id="automata-purpose"
                value={createPurpose}
                onChange={(e) => setCreatePurpose(e.target.value)}
                placeholder="Drive one task from discovery through delivery"
                disabled={isCreating}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="automata-goal">Goal</Label>
              <Textarea
                id="automata-goal"
                value={createGoal}
                onChange={(e) => setCreateGoal(e.target.value)}
                placeholder="Complete the assigned task and leave it ready for review"
                disabled={isCreating}
              />
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor="automata-stages">Stages</Label>
                <Textarea
                  id="automata-stages"
                  value={createStages}
                  onChange={(e) => setCreateStages(e.target.value)}
                  className="min-h-40 font-mono text-sm"
                  placeholder={'research\nplan\nimplement\nreview'}
                  disabled={isCreating}
                />
                <div className="text-xs text-muted-foreground">
                  One stage per line.
                </div>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="automata-dags">Allowed DAG Names</Label>
                <Textarea
                  id="automata-dags"
                  value={createAllowedDAGs}
                  onChange={(e) => setCreateAllowedDAGs(e.target.value)}
                  className="min-h-40 font-mono text-sm"
                  placeholder={'build-app\nrun-tests'}
                  disabled={isCreating}
                />
                <div className="text-xs text-muted-foreground">
                  One DAG name per line. Names must already exist.
                </div>
              </div>
            </div>

            {createError ? (
              <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                {createError}
              </div>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={closeCreateDialog}
              disabled={isCreating}
            >
              Cancel
            </Button>
            <Button type="button" onClick={onCreate} disabled={isCreating}>
              {isCreating ? 'Creating...' : 'Create Automata'}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <div className="grid grid-cols-1 gap-6 p-4 lg:grid-cols-[360px_minmax(0,1fr)]">
        <section className="rounded-xl border bg-card">
          <div className="flex items-center justify-between gap-3 border-b px-4 py-3">
            <h2 className="text-sm font-semibold tracking-wide text-muted-foreground uppercase">
              Automata
            </h2>
            <Button size="sm" onClick={openCreateDialog}>
              Create
            </Button>
          </div>
          {listQuery.isLoading ? (
            <LoadingIndicator />
          ) : (
            <div className="max-h-[calc(100vh-12rem)] overflow-y-auto p-2">
              {(listQuery.data?.automata || []).length === 0 ? (
                <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                  No Automata defined yet.
                  <div className="mt-3">
                    <Button size="sm" onClick={openCreateDialog}>
                      Create Automata
                    </Button>
                  </div>
                </div>
              ) : null}
              {(listQuery.data?.automata || []).map((item) => (
                <button
                  key={item.name}
                  onClick={() =>
                    navigate(`/automata/${encodeURIComponent(item.name)}`)
                  }
                  className={`mb-2 w-full rounded-lg border p-3 text-left transition ${
                    name === item.name
                      ? 'border-primary bg-primary/5'
                      : 'border-border hover:bg-muted/50'
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div>
                      <div className="font-medium">{item.name}</div>
                      <div className="mt-1 text-xs text-muted-foreground">
                        {item.instruction || item.purpose}
                      </div>
                    </div>
                    <span
                      className={`rounded-full px-2 py-1 text-[11px] font-medium ${statusClass(item.state)}`}
                    >
                      {item.state}
                    </span>
                  </div>
                  <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                    <span>Stage: {item.stage || 'n/a'}</span>
                    {item.currentRun ? (
                      <span>{item.currentRun.status}</span>
                    ) : null}
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="rounded-xl border bg-card">
          {!name ? (
            <div className="space-y-4 p-8 text-sm text-muted-foreground">
              <div>
                Select an Automata to inspect its state, stage, transcript, and
                recent DAG runs.
              </div>
              <div>
                <Button onClick={openCreateDialog}>Create Automata</Button>
              </div>
            </div>
          ) : detailQuery.isLoading ? (
            <LoadingIndicator />
          ) : detail ? (
            <div className="space-y-6 p-4">
              <div className="flex flex-wrap items-start justify-between gap-4">
                <div>
                  <h1 className="text-2xl font-semibold">
                    {detail.definition.name}
                  </h1>
                  {detail.definition.description ? (
                    <p className="mt-1 text-sm text-muted-foreground">
                      {detail.definition.description}
                    </p>
                  ) : null}
                </div>
                <div className="flex items-center gap-2">
                  {canPause || canResume ? (
                    <Button variant="outline" size="sm" onClick={onPauseResume}>
                      {canResume ? 'Resume' : 'Pause'}
                    </Button>
                  ) : null}
                  <span
                    className={`rounded-full px-3 py-1 text-xs font-medium ${statusClass(detail.state.state)}`}
                  >
                    {detail.state.state}
                  </span>
                  <span className="rounded-full bg-muted px-3 py-1 text-xs font-medium">
                    {detail.state.currentStage || 'no stage'}
                  </span>
                </div>
              </div>

              {error ? (
                <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                  {error}
                </div>
              ) : null}

              <div className="rounded-lg border p-4">
                <div className="mb-3 flex items-center justify-between gap-3">
                  <h2 className="text-sm font-semibold">Session Messages</h2>
                  {detail.state.sessionId ? (
                    <span className="text-[11px] text-muted-foreground">
                      Session: {detail.state.sessionId}
                    </span>
                  ) : null}
                </div>
                {(detail.messages || []).length ? (
                  <div className="space-y-2">
                    {(detail.messages || []).slice(-5).map((message) => (
                      <div
                        key={message.id}
                        className="rounded-md border p-2 text-sm"
                      >
                        <div className="mb-1 flex items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                          <span>{message.type}</span>
                          {message.created_at ? (
                            <span className="normal-case tracking-normal">
                              {new Date(message.created_at).toLocaleString()}
                            </span>
                          ) : null}
                        </div>
                        {message.content ? (
                          <div className="whitespace-pre-wrap">
                            {message.content}
                          </div>
                        ) : null}
                        {message.user_prompt?.question ? (
                          <div className="whitespace-pre-wrap">
                            {message.user_prompt.question}
                          </div>
                        ) : null}
                        {message.tool_results?.length ? (
                          <div className="mt-2 space-y-1">
                            {message.tool_results.map((result, index) => (
                              <div
                                key={index}
                                className="rounded bg-muted p-2 text-xs whitespace-pre-wrap"
                              >
                                {result.content}
                              </div>
                            ))}
                          </div>
                        ) : null}
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="text-sm text-muted-foreground">
                    No session messages yet.
                  </div>
                )}
              </div>

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="rounded-lg border p-4">
                  <h2 className="mb-2 text-sm font-semibold">Mission</h2>
                  <div className="space-y-2 text-sm">
                    <p>
                      <span className="font-medium">Purpose:</span>{' '}
                      {detail.definition.purpose}
                    </p>
                    <p>
                      <span className="font-medium">Goal:</span>{' '}
                      {detail.definition.goal}
                    </p>
                    <p>
                      <span className="font-medium">Instruction:</span>{' '}
                      {detail.state.instruction || 'None yet'}
                    </p>
                    {detail.state.lastSummary ? (
                      <p>
                        <span className="font-medium">Last Summary:</span>{' '}
                        {detail.state.lastSummary}
                      </p>
                    ) : null}
                    {detail.state.lastError ? (
                      <p className="text-destructive">
                        <span className="font-medium">Last Error:</span>{' '}
                        {detail.state.lastError}
                      </p>
                    ) : null}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    {detail.state.state === 'finished'
                      ? 'Start New Task'
                      : 'Start Instruction'}
                  </h2>
                  <div className="space-y-3">
                    <Textarea
                      value={instructionDraft}
                      onChange={(e) => setInstructionDraft(e.target.value)}
                      placeholder="Tell this Automata what task to work on before starting it."
                      disabled={!canStartTask}
                    />
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-xs text-muted-foreground">
                        {canStartTask
                          ? 'Automata stays idle until an instruction is provided.'
                          : detail.state.state === 'paused'
                            ? 'This Automata is paused. Resume it to continue the current task.'
                            : 'This Automata already has an active task. Use an operator message to steer it.'}
                      </div>
                      <Button
                        onClick={onStart}
                        disabled={!instructionDraft.trim() || !canStartTask}
                      >
                        {detail.state.state === 'finished'
                          ? 'Start New Task'
                          : 'Start'}
                      </Button>
                    </div>
                    {scheduleConfigured && detail.state.state === 'idle' ? (
                      <div className="text-xs text-muted-foreground">
                        A schedule is defined in the spec, but idle Automata are
                        not auto-started. Schedules do not create work by
                        themselves in this MVP.
                      </div>
                    ) : null}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">Stage Override</h2>
                  <div className="space-y-3">
                    <select
                      className="w-full rounded-md border bg-background px-3 py-2 text-sm"
                      value={stageOverride}
                      onChange={(e) => setStageOverride(e.target.value)}
                    >
                      {detail.definition.stages.map((stage) => (
                        <option key={stage} value={stage}>
                          {stage}
                        </option>
                      ))}
                    </select>
                    <Input
                      value={stageNote}
                      onChange={(e) => setStageNote(e.target.value)}
                      placeholder="Optional note"
                    />
                    <Button variant="outline" onClick={onOverrideStage}>
                      Update Stage
                    </Button>
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Operator Message
                  </h2>
                  <div className="space-y-3">
                    <Textarea
                      value={operatorMessageDraft}
                      onChange={(e) => setOperatorMessageDraft(e.target.value)}
                      placeholder="Add context, change priority, or clarify the current task."
                      disabled={!canSendOperatorMessage}
                    />
                    <div className="flex items-center justify-between gap-3">
                      <div className="text-xs text-muted-foreground">
                        {detail.state.pendingPrompt
                          ? 'Respond to the pending prompt before sending a general operator message.'
                          : canSendOperatorMessage
                            ? detail.state.state === 'paused'
                              ? 'This queues a user message, but the Automata will stay paused until you resume it.'
                              : 'This queues a user message into the active Automata task.'
                            : 'Operator messages are only accepted while the Automata has an active task.'}
                      </div>
                      <Button
                        variant="outline"
                        onClick={onSendOperatorMessage}
                        disabled={
                          !canSendOperatorMessage ||
                          !operatorMessageDraft.trim()
                        }
                      >
                        Send Message
                      </Button>
                    </div>
                  </div>
                </div>
              </div>

              {detail.state.pendingPrompt ? (
                <div className="rounded-lg border border-amber-400/40 bg-amber-50/50 p-4 dark:bg-amber-950/20">
                  <h2 className="mb-2 text-sm font-semibold">
                    Waiting For Human Input
                  </h2>
                  <p className="mb-3 text-sm">
                    {detail.state.pendingPrompt.question}
                  </p>
                  <div className="space-y-2">
                    {detail.state.state === 'paused' ? (
                      <div className="rounded-md border border-slate-300/60 bg-slate-100/70 px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-200">
                        Response will be queued, and the Automata will remain
                        paused until you resume it.
                      </div>
                    ) : null}
                    {(detail.state.pendingPrompt.options || []).map(
                      (option) => {
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
                              <span className="font-medium">
                                {option.label}
                              </span>
                              {option.description ? (
                                <span className="block text-xs text-muted-foreground">
                                  {option.description}
                                </span>
                              ) : null}
                            </span>
                          </label>
                        );
                      }
                    )}
                    {detail.state.pendingPrompt.allowFreeText ? (
                      <Textarea
                        value={freeTextResponse}
                        onChange={(e) => setFreeTextResponse(e.target.value)}
                        placeholder={
                          detail.state.pendingPrompt.freeTextPlaceholder ||
                          'Enter response'
                        }
                      />
                    ) : null}
                    <Button onClick={onRespond}>Submit Response</Button>
                  </div>
                </div>
              ) : null}

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">Allowed DAGs</h2>
                  <div className="space-y-2 text-sm">
                    {detail.allowedDags.map((dag) => (
                      <div key={dag.name} className="rounded-md border p-2">
                        <div className="font-medium">{dag.name}</div>
                        {dag.description ? (
                          <div className="text-xs text-muted-foreground">
                            {dag.description}
                          </div>
                        ) : null}
                        {dag.tags?.length ? (
                          <div className="mt-1 text-[11px] text-muted-foreground">
                            {dag.tags.join(', ')}
                          </div>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">Current Run</h2>
                  {detail.currentRun ? (
                    <div className="space-y-2 text-sm">
                      <div>
                        <span className="font-medium">DAG:</span>{' '}
                        {detail.currentRun.name}
                      </div>
                      <div>
                        <span className="font-medium">Run ID:</span>{' '}
                        {detail.currentRun.dagRunId}
                      </div>
                      <div>
                        <span className="font-medium">Status:</span>{' '}
                        {detail.currentRun.status}
                      </div>
                      <Link
                        className="text-primary underline underline-offset-2"
                        to={`/dag-runs/${encodeURIComponent(detail.currentRun.name)}/${encodeURIComponent(detail.currentRun.dagRunId)}`}
                      >
                        Open DAG run
                      </Link>
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      No active child DAG run.
                    </div>
                  )}
                </div>
              </div>

              <div className="rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Recent Runs</h2>
                <div className="space-y-2">
                  {(detail.recentRuns || []).map((run) => (
                    <div
                      key={`${run.name}:${run.dagRunId}`}
                      className="grid gap-1 rounded-md border p-2 text-sm lg:grid-cols-[1fr_160px_140px]"
                    >
                      <div>
                        <div className="font-medium">{run.name}</div>
                        <div className="text-xs text-muted-foreground">
                          {run.dagRunId}
                        </div>
                      </div>
                      <div>{run.status}</div>
                      <div className="text-xs text-muted-foreground">
                        {run.finishedAt || run.startedAt || ''}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Full Session Transcript
                  </h2>
                  {(detail.messages || []).length ? (
                    <div className="max-h-[28rem] space-y-2 overflow-y-auto">
                      {(detail.messages || []).slice(-40).map((message) => (
                        <div
                          key={message.id}
                          className="rounded-md border p-2 text-sm"
                        >
                          <div className="mb-1 flex items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                            <span>{message.type}</span>
                            {message.created_at ? (
                              <span className="normal-case tracking-normal">
                                {new Date(message.created_at).toLocaleString()}
                              </span>
                            ) : null}
                          </div>
                          {message.content ? (
                            <div className="whitespace-pre-wrap">
                              {message.content}
                            </div>
                          ) : null}
                          {message.user_prompt?.question ? (
                            <div className="whitespace-pre-wrap">
                              {message.user_prompt.question}
                            </div>
                          ) : null}
                          {message.tool_results?.length ? (
                            <div className="mt-2 space-y-1">
                              {message.tool_results.map((result, index) => (
                                <div
                                  key={index}
                                  className="rounded bg-muted p-2 text-xs whitespace-pre-wrap"
                                >
                                  {result.content}
                                </div>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      ))}
                    </div>
                  ) : (
                    <div className="text-sm text-muted-foreground">
                      No session messages yet.
                    </div>
                  )}
                </div>

                <div className="rounded-lg border p-4">
                  <div className="mb-3 flex items-center justify-between gap-2">
                    <h2 className="text-sm font-semibold">Raw Spec</h2>
                    <div className="flex items-center gap-2">
                      {isEditingSpec ? (
                        <>
                          <Button
                            size="sm"
                            variant="ghost"
                            onClick={() => {
                              setIsEditingSpec(false);
                              setSpecDraft(specQuery.data?.spec || '');
                              setSpecError('');
                            }}
                            disabled={isSavingSpec}
                          >
                            Cancel
                          </Button>
                          <Button
                            size="sm"
                            onClick={onSaveSpec}
                            disabled={isSavingSpec}
                          >
                            {isSavingSpec ? 'Saving...' : 'Save'}
                          </Button>
                        </>
                      ) : (
                        <Button
                          size="sm"
                          variant="outline"
                          onClick={() => {
                            setSpecDraft(specQuery.data?.spec || '');
                            setSpecError('');
                            setIsEditingSpec(true);
                          }}
                        >
                          Edit Spec
                        </Button>
                      )}
                    </div>
                  </div>
                  {specError ? (
                    <div className="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
                      {specError}
                    </div>
                  ) : null}
                  {isEditingSpec ? (
                    <Textarea
                      value={specDraft}
                      onChange={(e) => setSpecDraft(e.target.value)}
                      className="min-h-[28rem] font-mono text-xs"
                    />
                  ) : (
                    <pre className="max-h-[28rem] overflow-auto rounded-md bg-muted p-3 text-xs">
                      {specQuery.data?.spec || ''}
                    </pre>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="p-8 text-sm text-muted-foreground">
              Automata definition not found.
            </div>
          )}
        </section>
      </div>
    </>
  );
}

export default AutomataPage;
