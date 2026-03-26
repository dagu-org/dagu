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
import { cn } from '@/lib/utils';
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
    stages: {
      name: string;
      allowedDAGs?: {
        names?: string[];
        tags?: string[];
      };
    }[];
    disabled?: boolean;
  };
  state: {
    state: string;
    instruction?: string;
    currentStage?: string;
    waitingReason?: string;
    pendingStageTransition?: {
      requestedStage: string;
      note?: string;
      requestedBy?: string;
      createdAt?: string;
    };
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
const DEFAULT_STAGE_NAMES = ['research', 'plan', 'implement', 'review'];

type DAGOption = {
  fileName: string;
  name: string;
};

type StageDraft = {
  id: string;
  name: string;
  allowedDAGNames: string[];
};

function makeDraftID(): string {
  return Math.random().toString(36).slice(2, 10);
}

function createDefaultStageDrafts(): StageDraft[] {
  return DEFAULT_STAGE_NAMES.map((name) => ({
    id: makeDraftID(),
    name,
    allowedDAGNames: [],
  }));
}

function DAGNameMultiSelect({
  availableDAGs,
  selectedNames,
  onChange,
  disabled,
}: {
  availableDAGs: DAGOption[];
  selectedNames: string[];
  onChange: (names: string[]) => void;
  disabled?: boolean;
}): React.ReactElement {
  const [isOpen, setIsOpen] = React.useState(false);
  const [searchQuery, setSearchQuery] = React.useState('');
  const dropdownRef = React.useRef<HTMLDivElement>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);

  const selectedNameSet = React.useMemo(
    () => new Set(selectedNames),
    [selectedNames]
  );

  const filteredDAGs = React.useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    if (!query) {
      return availableDAGs;
    }
    return availableDAGs.filter(
      (dag) =>
        dag.fileName.toLowerCase().includes(query) ||
        dag.name.toLowerCase().includes(query)
    );
  }, [availableDAGs, searchQuery]);

  React.useEffect(() => {
    function handleClickOutside(event: MouseEvent): void {
      if (
        dropdownRef.current &&
        !dropdownRef.current.contains(event.target as Node)
      ) {
        setIsOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  React.useEffect(() => {
    if (isOpen) {
      inputRef.current?.focus();
    }
  }, [isOpen]);

  const toggleSelection = (fileName: string) => {
    if (selectedNameSet.has(fileName)) {
      onChange(selectedNames.filter((name) => name !== fileName));
      return;
    }
    onChange([...selectedNames, fileName]);
  };

  return (
    <div className="relative space-y-2" ref={dropdownRef}>
      <div className="flex flex-wrap gap-1">
        {selectedNames.length ? (
          selectedNames.map((dagName) => (
            <span
              key={dagName}
              className="inline-flex items-center gap-1 rounded bg-secondary px-2 py-0.5 text-xs text-secondary-foreground"
            >
              <span className="max-w-[180px] truncate">{dagName}</span>
              <button
                type="button"
                onClick={() =>
                  onChange(selectedNames.filter((name) => name !== dagName))
                }
                disabled={disabled}
                className="text-muted-foreground hover:text-foreground"
              >
                x
              </button>
            </span>
          ))
        ) : (
          <div className="text-xs text-muted-foreground">
            No DAGs selected for this stage.
          </div>
        )}
      </div>

      <Button
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setIsOpen((open) => !open)}
        disabled={disabled}
      >
        Select DAGs
      </Button>

      {isOpen ? (
        <div className="absolute z-50 mt-1 w-full rounded-md border bg-popover shadow-lg">
          <div className="border-b p-2">
            <input
              ref={inputRef}
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search DAGs..."
              className="w-full rounded border bg-background px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-ring"
            />
          </div>
          <div className="max-h-56 overflow-y-auto p-1">
            {filteredDAGs.length ? (
              filteredDAGs.map((dag) => {
                const selected = selectedNameSet.has(dag.fileName);
                return (
                  <button
                    key={dag.fileName}
                    type="button"
                    onClick={() => toggleSelection(dag.fileName)}
                    className={cn(
                      'flex w-full items-center justify-between rounded px-3 py-2 text-left text-sm hover:bg-accent',
                      selected && 'bg-accent'
                    )}
                  >
                    <span className="truncate">
                      {dag.fileName}
                      {dag.name && dag.name !== dag.fileName
                        ? ` (${dag.name})`
                        : ''}
                    </span>
                    {selected ? (
                      <span className="text-primary">Selected</span>
                    ) : null}
                  </button>
                );
              })
            ) : (
              <div className="px-3 py-2 text-sm text-muted-foreground">
                {searchQuery ? 'No DAGs found.' : 'No DAGs available.'}
              </div>
            )}
          </div>
        </div>
      ) : null}
    </div>
  );
}

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

function quoteYAML(value: string): string {
  return JSON.stringify(value.trim());
}

function buildAutomataSpec(input: {
  description: string;
  purpose: string;
  goal: string;
  stages: StageDraft[];
}): string {
  const lines = [
    `description: ${quoteYAML(input.description || 'Automata workflow')}`,
    `purpose: ${quoteYAML(input.purpose)}`,
    `goal: ${quoteYAML(input.goal)}`,
    '',
    'stages:',
  ];

  input.stages.forEach((stage) => {
    const stageName = stage.name.trim();
    const allowedDAGNames = Array.from(
      new Set(stage.allowedDAGNames.map((name) => name.trim()).filter(Boolean))
    );
    lines.push(`  - name: ${quoteYAML(stageName)}`);
    if (allowedDAGNames.length) {
      lines.push('    allowedDAGs:');
      lines.push('      names:');
      allowedDAGNames.forEach((dagName) => {
        lines.push(`        - ${quoteYAML(dagName)}`);
      });
    }
  });

  lines.push('');
  lines.push('agent:');
  lines.push('  safeMode: true');
  lines.push('');

  return lines.join('\n');
}

function validateAutomataCreateForm(input: {
  name: string;
  purpose: string;
  goal: string;
  stages: StageDraft[];
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
  if (input.stages.length === 0) {
    return 'At least one stage is required.';
  }
  const seenStageNames = new Set<string>();
  let totalAllowedDAGs = 0;
  for (const stage of input.stages) {
    const trimmedName = stage.name.trim();
    if (!trimmedName) {
      return 'Every stage needs a name.';
    }
    if (seenStageNames.has(trimmedName)) {
      return `Duplicate stage name: ${trimmedName}`;
    }
    seenStageNames.add(trimmedName);
    totalAllowedDAGs += stage.allowedDAGNames.length;
  }
  if (totalAllowedDAGs === 0) {
    return 'Select at least one DAG across the stage list.';
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
  const [createStages, setCreateStages] = React.useState<StageDraft[]>(
    createDefaultStageDrafts()
  );
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

  const dagListQuery = useSWR<{
    dags: { fileName: string; dag: { name: string } }[];
  }>(
    `/dags?perPage=500${
      appBar.selectedRemoteNode
        ? `&remoteNode=${encodeURIComponent(appBar.selectedRemoteNode)}`
        : ''
    }`,
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
  const availableDAGOptions = React.useMemo<DAGOption[]>(() => {
    return (dagListQuery.data?.dags || []).map((dag) => ({
      fileName: dag.fileName,
      name: dag.dag?.name || dag.fileName,
    }));
  }, [dagListQuery.data?.dags]);
  const stageNames = React.useMemo(
    () => detail?.definition.stages.map((stage) => stage.name) || [],
    [detail?.definition.stages]
  );
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
    if (!stageNames.length) {
      return;
    }
    const currentStage = detail?.state?.currentStage;
    if (currentStage && currentStage !== stageOverride) {
      setStageOverride(currentStage);
      return;
    }
    if (!stageOverride || !stageNames.includes(stageOverride)) {
      setStageOverride(stageNames[0]);
    }
  }, [detail?.state?.currentStage, stageNames, stageOverride]);

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
    setSelectedOptions([]);
    setFreeTextResponse('');
  }, [detail?.state?.pendingPrompt?.id, name]);

  React.useEffect(() => {
    setIsEditingSpec(false);
    setSpecError('');
  }, [name]);

  const resetCreateForm = () => {
    setCreateName('');
    setCreateDescription('');
    setCreatePurpose('');
    setCreateGoal('');
    setCreateStages(createDefaultStageDrafts());
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
      stages: createStages,
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
            stages: createStages,
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

  const updateCreateStage = (
    stageID: string,
    updater: (stage: StageDraft) => StageDraft
  ) => {
    setCreateStages((prev) =>
      prev.map((stage) => (stage.id === stageID ? updater(stage) : stage))
    );
  };

  const addCreateStage = () => {
    setCreateStages((prev) => [
      ...prev,
      { id: makeDraftID(), name: '', allowedDAGNames: [] },
    ]);
  };

  const removeCreateStage = (stageID: string) => {
    setCreateStages((prev) =>
      prev.length === 1 ? prev : prev.filter((stage) => stage.id !== stageID)
    );
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

  const submitHumanResponse = async (
    selectedOptionIds: string[],
    freeText: string
  ) => {
    if (!name || !detail?.state?.pendingPrompt) return;
    setError('');
    try {
      await sendJSON(`/automata/${encodeURIComponent(name)}/response`, 'POST', {
        promptId: detail.state.pendingPrompt.id,
        selectedOptionIds,
        freeTextResponse: freeText,
      });
      setSelectedOptions([]);
      setFreeTextResponse('');
      void detailQuery.mutate();
      void listQuery.mutate();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to respond');
    }
  };

  const onRespond = async () => {
    await submitHumanResponse(selectedOptions, freeTextResponse);
  };

  const onRespondStageTransition = async (decision: string) => {
    await submitHumanResponse([decision], '');
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

            <div className="space-y-3">
              <div className="flex items-center justify-between gap-3">
                <div>
                  <Label>Stages</Label>
                  <div className="text-xs text-muted-foreground">
                    Each stage has its own allowlisted DAG set.
                  </div>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={addCreateStage}
                  disabled={isCreating}
                >
                  Add Stage
                </Button>
              </div>

              <div className="space-y-3">
                {createStages.map((stage, index) => (
                  <div key={stage.id} className="rounded-lg border p-3">
                    <div className="mb-3 flex items-center justify-between gap-3">
                      <div className="text-sm font-medium">
                        Stage {index + 1}
                      </div>
                      <Button
                        type="button"
                        variant="ghost"
                        size="sm"
                        onClick={() => removeCreateStage(stage.id)}
                        disabled={isCreating || createStages.length === 1}
                      >
                        Remove
                      </Button>
                    </div>

                    <div className="grid gap-3 lg:grid-cols-[220px_minmax(0,1fr)]">
                      <div className="grid gap-2">
                        <Label htmlFor={`automata-stage-name-${stage.id}`}>
                          Stage Name
                        </Label>
                        <Input
                          id={`automata-stage-name-${stage.id}`}
                          value={stage.name}
                          onChange={(e) =>
                            updateCreateStage(stage.id, (current) => ({
                              ...current,
                              name: e.target.value,
                            }))
                          }
                          placeholder="research"
                          disabled={isCreating}
                        />
                      </div>

                      <div className="grid gap-2">
                        <Label>DAG Selection</Label>
                        <DAGNameMultiSelect
                          availableDAGs={availableDAGOptions}
                          selectedNames={stage.allowedDAGNames}
                          onChange={(names) =>
                            updateCreateStage(stage.id, (current) => ({
                              ...current,
                              allowedDAGNames: names,
                            }))
                          }
                          disabled={isCreating}
                        />
                        <div className="text-xs text-muted-foreground">
                          Select the DAGs this Automata can run while it is in
                          the {stage.name.trim() || `stage ${index + 1}`} stage.
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>

              <div className="text-xs text-muted-foreground">
                {dagListQuery.isLoading
                  ? 'Loading DAGs for selection...'
                  : 'The dropdown only lists DAGs already available on this node.'}
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
                        <option key={stage.name} value={stage.name}>
                          {stage.name}
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
                    <div className="text-xs text-muted-foreground">
                      This is an immediate operator override. Agent-requested
                      stage changes go through approval.
                    </div>
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
                    {detail.state.pendingStageTransition
                      ? 'Stage Change Approval'
                      : 'Waiting For Human Input'}
                  </h2>
                  {detail.state.pendingStageTransition ? (
                    <div className="mb-3 space-y-2 text-sm">
                      <p>{detail.state.pendingPrompt.question}</p>
                      <div className="grid gap-2 md:grid-cols-2">
                        <div className="rounded-md border bg-background/70 p-3">
                          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Current Stage
                          </div>
                          <div className="mt-1 font-medium">
                            {detail.state.currentStage || 'n/a'}
                          </div>
                        </div>
                        <div className="rounded-md border bg-background/70 p-3">
                          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Requested Stage
                          </div>
                          <div className="mt-1 font-medium">
                            {detail.state.pendingStageTransition.requestedStage}
                          </div>
                        </div>
                      </div>
                      {detail.state.pendingStageTransition.note ? (
                        <div className="rounded-md border bg-background/70 p-3">
                          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                            Agent Note
                          </div>
                          <div className="mt-1 whitespace-pre-wrap">
                            {detail.state.pendingStageTransition.note}
                          </div>
                        </div>
                      ) : null}
                    </div>
                  ) : (
                    <p className="mb-3 text-sm">
                      {detail.state.pendingPrompt.question}
                    </p>
                  )}
                  <div className="space-y-2">
                    {detail.state.state === 'paused' ? (
                      <div className="rounded-md border border-slate-300/60 bg-slate-100/70 px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-200">
                        Response will be queued, and the Automata will remain
                        paused until you resume it.
                      </div>
                    ) : null}
                    {detail.state.pendingStageTransition ? (
                      <div className="flex flex-wrap gap-2">
                        <Button
                          onClick={() => onRespondStageTransition('approve')}
                        >
                          Approve Stage Change
                        </Button>
                        <Button
                          variant="outline"
                          onClick={() => onRespondStageTransition('reject')}
                        >
                          Reject Stage Change
                        </Button>
                      </div>
                    ) : (
                      <>
                        {(detail.state.pendingPrompt.options || []).map(
                          (option) => {
                            const selected = selectedOptions.includes(
                              option.id
                            );
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
                            onChange={(e) =>
                              setFreeTextResponse(e.target.value)
                            }
                            placeholder={
                              detail.state.pendingPrompt.freeTextPlaceholder ||
                              'Enter response'
                            }
                          />
                        ) : null}
                        <Button onClick={onRespond}>Submit Response</Button>
                      </>
                    )}
                  </div>
                </div>
              ) : null}

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Current Stage DAGs
                  </h2>
                  <div className="space-y-2 text-sm">
                    {detail.allowedDags.length ? (
                      detail.allowedDags.map((dag) => (
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
                      ))
                    ) : (
                      <div className="rounded-md border border-dashed p-3 text-muted-foreground">
                        No DAGs are assigned to the current stage.
                      </div>
                    )}
                  </div>
                </div>

                <div className="rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Stage DAG Mapping
                  </h2>
                  <div className="space-y-3 text-sm">
                    {detail.definition.stages.map((stage) => {
                      const allowedNames = stage.allowedDAGs?.names || [];
                      const allowedTags = stage.allowedDAGs?.tags || [];
                      const isCurrentStage =
                        stage.name === detail.state.currentStage;
                      return (
                        <div
                          key={stage.name}
                          className={cn(
                            'rounded-md border p-3',
                            isCurrentStage && 'border-primary bg-primary/5'
                          )}
                        >
                          <div className="flex items-center justify-between gap-3">
                            <div className="font-medium">{stage.name}</div>
                            {isCurrentStage ? (
                              <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
                                current
                              </span>
                            ) : null}
                          </div>
                          {allowedNames.length ? (
                            <div className="mt-2 text-xs text-muted-foreground">
                              DAGs: {allowedNames.join(', ')}
                            </div>
                          ) : null}
                          {allowedTags.length ? (
                            <div className="mt-1 text-xs text-muted-foreground">
                              Tags: {allowedTags.join(', ')}
                            </div>
                          ) : null}
                          {!allowedNames.length && !allowedTags.length ? (
                            <div className="mt-2 text-xs text-muted-foreground">
                              No DAGs assigned to this stage.
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </div>

                <div className="rounded-lg border p-4 lg:col-span-2">
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
