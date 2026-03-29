import React from 'react';
import { createPortal } from 'react-dom';
import { useNavigate, useParams } from 'react-router-dom';

import { Status, type components } from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { AppBarContext } from '@/contexts/AppBarContext';
import { DAGRunDetailsModal } from '@/features/dag-runs/components/dag-run-details';
import { DAGDetailsModal } from '@/features/dags/components/dag-details';
import { useClient, useQuery } from '@/hooks/api';
import { cn } from '@/lib/utils';
import LoadingIndicator from '@/ui/LoadingIndicator';
import StatusChip from '@/ui/StatusChip';

type AutomataDetail = components['schemas']['AutomataDetailResponse'];

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

function parseTagInput(value: string): string[] {
  return Array.from(
    new Set(
      value
        .split(/[\n,]/)
        .map((item) => item.trim())
        .filter(Boolean)
    )
  );
}

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
  const containerRef = React.useRef<HTMLDivElement>(null);
  const dropdownRef = React.useRef<HTMLDivElement>(null);
  const triggerRef = React.useRef<HTMLButtonElement>(null);
  const inputRef = React.useRef<HTMLInputElement>(null);
  const [dropdownStyle, setDropdownStyle] = React.useState<React.CSSProperties>(
    {}
  );

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
        !containerRef.current?.contains(event.target as Node) &&
        !dropdownRef.current?.contains(event.target as Node)
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

  React.useLayoutEffect(() => {
    if (!isOpen) {
      return;
    }

    function updateDropdownPosition(): void {
      const rect = triggerRef.current?.getBoundingClientRect();
      if (!rect) {
        return;
      }
      const viewportPadding = 16;
      const availableWidth = Math.max(
        320,
        window.innerWidth - viewportPadding * 2
      );
      const desiredWidth = Math.max(rect.width, 520);
      const width = Math.min(desiredWidth, availableWidth);
      const left = Math.min(
        Math.max(viewportPadding, rect.left),
        window.innerWidth - viewportPadding - width
      );
      setDropdownStyle({
        position: 'fixed',
        top: rect.bottom + 4,
        left: `${left}px`,
        width: `${width}px`,
        minWidth: `${Math.min(520, availableWidth)}px`,
        maxWidth: `${availableWidth}px`,
        zIndex: 60,
      });
    }

    updateDropdownPosition();
    window.addEventListener('resize', updateDropdownPosition);
    window.addEventListener('scroll', updateDropdownPosition, true);
    return () => {
      window.removeEventListener('resize', updateDropdownPosition);
      window.removeEventListener('scroll', updateDropdownPosition, true);
    };
  }, [isOpen]);

  const toggleSelection = (fileName: string) => {
    if (selectedNameSet.has(fileName)) {
      onChange(selectedNames.filter((name) => name !== fileName));
      return;
    }
    onChange([...selectedNames, fileName]);
  };

  return (
    <div className="space-y-2" ref={containerRef}>
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
        ref={triggerRef}
        type="button"
        variant="outline"
        size="sm"
        onClick={() => setIsOpen((open) => !open)}
        disabled={disabled}
      >
        Select DAGs
      </Button>

      {isOpen && typeof document !== 'undefined'
        ? createPortal(
            <div
              ref={dropdownRef}
              style={dropdownStyle}
              className="rounded-md border bg-popover shadow-lg"
            >
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
                          'flex w-full items-start justify-between gap-3 rounded px-3 py-2 text-left text-sm hover:bg-accent',
                          selected && 'bg-accent'
                        )}
                      >
                        <span className="min-w-0 flex-1">
                          <span className="block whitespace-normal break-words font-mono text-xs">
                            {dag.fileName}
                          </span>
                          {dag.name && dag.name !== dag.fileName ? (
                            <span className="mt-0.5 block whitespace-normal break-words text-xs text-muted-foreground">
                              {dag.name}
                            </span>
                          ) : null}
                        </span>
                        {selected ? (
                          <span className="shrink-0 text-primary">Selected</span>
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
            </div>,
            document.body
          )
        : null}
    </div>
  );
}

function quoteYAML(value: string): string {
  return JSON.stringify(value.trim());
}

function buildAutomataSpec(input: {
  description: string;
  goal: string;
  tags: string[];
  stages: StageDraft[];
}): string {
  const lines = [
    `description: ${quoteYAML(input.description || 'Automata workflow')}`,
    `goal: ${quoteYAML(input.goal)}`,
  ];

  if (input.tags.length) {
    lines.push('tags:');
    input.tags.forEach((tag) => {
      lines.push(`  - ${quoteYAML(tag)}`);
    });
  }

  lines.push('');
  lines.push('stages:');

  input.stages.forEach((stage) => {
    const stageName = stage.name.trim();
    const allowedDAGNames = Array.from(
      new Set(stage.allowedDAGNames.map((name) => name.trim()).filter(Boolean))
    );
    lines.push(`  - name: ${quoteYAML(stageName)}`);
    if (allowedDAGNames.length) {
      lines.push('    allowed_dags:');
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

function dagRunStatusToStatus(status: string): Status | undefined {
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

function AutomataPage(): React.ReactElement {
  const appBar = React.useContext(AppBarContext);
  const client = useClient();
  const navigate = useNavigate();
  const { name } = useParams();

  const [showCreateDialog, setShowCreateDialog] = React.useState(false);
  const [createName, setCreateName] = React.useState('');
  const [createDescription, setCreateDescription] = React.useState('');
  const [createGoal, setCreateGoal] = React.useState('');
  const [createTags, setCreateTags] = React.useState('');
  const [createStages, setCreateStages] = React.useState<StageDraft[]>(
    createDefaultStageDrafts()
  );
  const [createError, setCreateError] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);
  const [managementBusy, setManagementBusy] = React.useState<string | null>(
    null
  );

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
  const [selectedDAGRun, setSelectedDAGRun] = React.useState<{
    name: string;
    dagRunId: string;
  } | null>(null);
  const [selectedDAG, setSelectedDAG] = React.useState<string | null>(null);

  React.useEffect(() => {
    appBar.setTitle('Automata');
  }, [appBar]);

  const listQuery = useQuery('/automata', {}, { refreshInterval: 15000 });

  const dagListQuery = useQuery(
    '/dags',
    {
      params: {
        query: {
          perPage: 500,
          remoteNode: appBar.selectedRemoteNode || undefined,
        },
      },
    },
    { refreshInterval: 15000 }
  );

  const detailQuery = useQuery(
    '/automata/{name}',
    name ? { params: { path: { name } } } : null,
    {
      refreshInterval: (data) =>
        data?.state?.state === 'running' ||
        data?.state?.state === 'waiting' ||
        data?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useQuery(
    '/automata/{name}/spec',
    name ? { params: { path: { name } } } : null,
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
  const mergedRuns = React.useMemo(() => {
    if (!detail) {
      return [];
    }
    const seen = new Set<string>();
    const items: Array<
      NonNullable<AutomataDetail['recentRuns']>[number] & { isCurrent?: boolean }
    > = [];
    if (detail.currentRun) {
      const key = `${detail.currentRun.name}:${detail.currentRun.dagRunId}`;
      seen.add(key);
      items.push({
        ...detail.currentRun,
        isCurrent: true,
      });
    }
    for (const run of detail.recentRuns || []) {
      const key = `${run.name}:${run.dagRunId}`;
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      items.push(run);
    }
    return items;
  }, [detail]);
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
      const firstStage = stageNames[0];
      if (firstStage) {
        setStageOverride(firstStage);
      }
    }
  }, [detail?.state?.currentStage, stageNames, stageOverride]);

  React.useEffect(() => {
    setInstructionDraft(detail?.state?.instruction || '');
  }, [detail?.state?.instruction, name]);

  React.useEffect(() => {
    setOperatorMessageDraft('');
  }, [name]);

  React.useEffect(() => {
    setSelectedDAGRun(null);
    setSelectedDAG(null);
  }, [name, showCreateDialog]);

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
    setCreateGoal('');
    setCreateTags('');
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
      const { error: apiError } = await client.PUT('/automata/{name}/spec', {
        params: { path: { name: automataName } },
        body: {
          spec: buildAutomataSpec({
            description: createDescription,
            goal: createGoal,
            tags: parseTagInput(createTags),
            stages: createStages,
          }),
        },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to create automata');
      }
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
      const { error: apiError } = await client.POST('/automata/{name}/start', {
        params: { path: { name } },
        body: { instruction: instructionDraft || undefined },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to start automata');
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
      const { error: apiError } = await client.POST('/automata/{name}/stage', {
        params: { path: { name } },
        body: {
          stage: stageOverride,
          note: stageNote || undefined,
        },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to update stage');
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
      const { error: apiError } = await client.POST('/automata/{name}/response', {
        params: { path: { name } },
        body: {
          promptId: detail.state.pendingPrompt.id,
          selectedOptionIds:
            selectedOptionIds.length > 0 ? selectedOptionIds : undefined,
          freeTextResponse: freeText || undefined,
        },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to respond');
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
      const { error: apiError } = await client.POST('/automata/{name}/message', {
        params: { path: { name } },
        body: { message: operatorMessageDraft },
      });
      if (apiError)
        throw new Error(apiError.message || 'Failed to send operator message');
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
      const response = paused
        ? await client.POST('/automata/{name}/resume', {
            params: { path: { name } },
          })
        : await client.POST('/automata/{name}/pause', {
            params: { path: { name } },
          });
      if (response.error) {
        throw new Error(
          response.error.message ||
            (paused ? 'Failed to resume automata' : 'Failed to pause automata')
        );
      }
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

  const onRename = async () => {
    if (!name || !detail || managementBusy) return;
    const nextName = window.prompt(
      'Enter the new Automata name.',
      detail.definition.name
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed || trimmed === detail.definition.name) return;
    setError('');
    setManagementBusy('rename');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/rename', {
        params: { path: { name } },
        body: { newName: trimmed },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to rename automata');
      await listQuery.mutate();
      navigate(`/automata/${encodeURIComponent(trimmed)}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to rename automata');
    } finally {
      setManagementBusy(null);
    }
  };

  const onDuplicate = async () => {
    if (!name || !detail || managementBusy) return;
    const nextName = window.prompt(
      'Enter the new name for the duplicate Automata.',
      `${detail.definition.name}-copy`
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed) return;
    setError('');
    setManagementBusy('duplicate');
    try {
      const { error: apiError } = await client.POST(
        '/automata/{name}/duplicate',
        {
          params: { path: { name } },
          body: { newName: trimmed },
        }
      );
      if (apiError)
        throw new Error(apiError.message || 'Failed to duplicate automata');
      await listQuery.mutate();
      navigate(`/automata/${encodeURIComponent(trimmed)}`);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to duplicate automata'
      );
    } finally {
      setManagementBusy(null);
    }
  };

  const onResetState = async () => {
    if (!name || managementBusy) return;
    if (
      !window.confirm(
        'Reset this Automata state? This clears the active task, session transcript binding, and tracked runtime state.'
      )
    ) {
      return;
    }
    setError('');
    setManagementBusy('reset');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/reset', {
        params: { path: { name } },
      });
      if (apiError)
        throw new Error(apiError.message || 'Failed to reset automata state');
      await Promise.all([detailQuery.mutate(), listQuery.mutate()]);
    } catch (err) {
      setError(
        err instanceof Error ? err.message : 'Failed to reset automata state'
      );
    } finally {
      setManagementBusy(null);
    }
  };

  const onDelete = async () => {
    if (!name || managementBusy) return;
    if (
      !window.confirm(
        'Delete this Automata? This removes the definition and runtime state.'
      )
    ) {
      return;
    }
    setError('');
    setManagementBusy('delete');
    try {
      const { error: apiError } = await client.DELETE('/automata/{name}', {
        params: { path: { name } },
      });
      if (apiError)
        throw new Error(apiError.message || 'Failed to delete automata');
      await listQuery.mutate();
      navigate('/automata');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to delete automata');
    } finally {
      setManagementBusy(null);
    }
  };

  const onSaveSpec = async () => {
    if (!name) return;
    setSpecError('');
    setIsSavingSpec(true);
    try {
      const { error: apiError } = await client.PUT('/automata/{name}/spec', {
        params: { path: { name } },
        body: { spec: specDraft },
      });
      if (apiError) throw new Error(apiError.message || 'Failed to save spec');
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
      <div className="-m-4 w-[calc(100%+2rem)] md:-m-6 md:h-[calc(100%+3rem)] md:w-[calc(100%+3rem)]">
        <div className="grid min-h-full grid-cols-1 border-border md:h-full md:grid-cols-[360px_minmax(0,1fr)]">
          <section className="border-b bg-background md:flex md:min-h-0 md:flex-col md:border-r md:border-b-0">
            <div className="flex items-center justify-between gap-3 border-b px-4 py-3 md:px-6 md:py-4">
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
              <div className="p-2 md:min-h-0 md:flex-1 md:overflow-y-auto md:pl-4 md:pr-2 md:pt-4 md:pb-6">
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
                    onClick={() => {
                      setShowCreateDialog(false);
                      navigate(`/automata/${encodeURIComponent(item.name)}`);
                    }}
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
                          {item.instruction || item.goal}
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
                        <StatusChip
                          status={dagRunStatusToStatus(item.currentRun.status)}
                          size="xs"
                        >
                          {item.currentRun.status}
                        </StatusChip>
                      ) : null}
                    </div>
                    {item.tags?.length ? (
                      <div className="mt-2 flex flex-wrap gap-1">
                        {item.tags.map((tag) => (
                          <span
                            key={`${item.name}-${tag}`}
                            className="rounded-full border px-2 py-0.5 text-[11px] text-muted-foreground"
                          >
                            {tag}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </button>
                ))}
              </div>
            )}
          </section>

          <section className="min-w-0 bg-background md:min-h-0 md:h-full">
            {showCreateDialog ? (
              <div className="space-y-6 overflow-x-hidden p-4 md:h-full md:overflow-auto md:p-6">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h1 className="text-2xl font-semibold">Create Automata</h1>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Define the Automata goal, stages, and per-stage DAG
                    allowlists. You can still refine the raw spec after
                    creation.
                  </p>
                </div>
                <Button
                  type="button"
                  variant="ghost"
                  onClick={closeCreateDialog}
                  disabled={isCreating}
                >
                  Cancel
                </Button>
              </div>

              <div className="grid gap-4">
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
                  <Label htmlFor="automata-goal">Goal</Label>
                  <Textarea
                    id="automata-goal"
                    value={createGoal}
                    onChange={(e) => setCreateGoal(e.target.value)}
                    placeholder="Complete the assigned task and leave it ready for review"
                    disabled={isCreating}
                  />
                </div>

                <div className="grid gap-2">
                  <Label htmlFor="automata-tags">Tags</Label>
                  <Textarea
                    id="automata-tags"
                    value={createTags}
                    onChange={(e) => setCreateTags(e.target.value)}
                    placeholder={'workspace=engineering, owner=team-ai'}
                    disabled={isCreating}
                    rows={2}
                  />
                  <div className="text-xs text-muted-foreground">
                    Optional. Use comma or newline separated tags such as
                    <span className="mx-1 font-mono text-foreground">
                      workspace=engineering
                    </span>
                    or
                    <span className="mx-1 font-mono text-foreground">
                      owner=team-ai
                    </span>
                    .
                  </div>
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
                              Select the DAGs this Automata can run while it is
                              in the {stage.name.trim() || `stage ${index + 1}`}{' '}
                              stage.
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

                <div className="flex justify-end gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={closeCreateDialog}
                    disabled={isCreating}
                  >
                    Cancel
                  </Button>
                  <Button
                    type="button"
                    onClick={onCreate}
                    disabled={isCreating}
                  >
                    {isCreating ? 'Creating...' : 'Create Automata'}
                  </Button>
                </div>
              </div>
            </div>
          ) : !name ? (
            <div className="space-y-4 p-8 text-sm text-muted-foreground md:h-full md:overflow-auto md:p-6">
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
            <div className="space-y-6 overflow-x-hidden p-4 md:h-full md:overflow-auto md:p-6">
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
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onDuplicate}
                    disabled={!!managementBusy}
                  >
                    Duplicate
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onRename}
                    disabled={!!managementBusy}
                  >
                    Rename
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onResetState}
                    disabled={!!managementBusy}
                  >
                    Reset State
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    onClick={onDelete}
                    disabled={!!managementBusy}
                  >
                    Delete
                  </Button>
                  {canPause || canResume ? (
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={onPauseResume}
                      disabled={!!managementBusy}
                    >
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

              <div className="min-w-0 rounded-lg border p-4">
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
                          {message.createdAt ? (
                            <span className="normal-case tracking-normal">
                              {new Date(message.createdAt).toLocaleString()}
                            </span>
                          ) : null}
                        </div>
                        {message.content ? (
                          <div className="whitespace-pre-wrap break-words">
                            {message.content}
                          </div>
                        ) : null}
                        {message.userPrompt?.question ? (
                          <div className="whitespace-pre-wrap break-words">
                            {message.userPrompt.question}
                          </div>
                        ) : null}
                        {message.toolResults?.length ? (
                          <div className="mt-2 space-y-1">
                            {message.toolResults.map((result, index) => (
                              <div
                                key={index}
                                className="rounded bg-muted p-2 text-xs whitespace-pre-wrap break-words"
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
                <div className="min-w-0 rounded-lg border p-4">
                  <h2 className="mb-2 text-sm font-semibold">Mission</h2>
                  <div className="space-y-2 text-sm">
                    <p>
                      <span className="font-medium">Goal:</span>{' '}
                      {detail.definition.goal}
                    </p>
                    {detail.definition.tags?.length ? (
                      <div>
                        <span className="font-medium">Tags:</span>
                        <div className="mt-1 flex flex-wrap gap-1">
                          {detail.definition.tags.map((tag) => (
                            <span
                              key={tag}
                              className="rounded-full border px-2 py-0.5 text-xs text-muted-foreground"
                            >
                              {tag}
                            </span>
                          ))}
                        </div>
                      </div>
                    ) : null}
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

                <div className="min-w-0 rounded-lg border p-4">
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

                <div className="min-w-0 rounded-lg border p-4">
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

                <div className="min-w-0 rounded-lg border p-4">
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
                              ? 'This records your message now, but the Automata will stay paused until you resume it.'
                              : detail.state.currentRunRef
                                ? 'This records your message now and the Automata will pick it up after the current child DAG changes state.'
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
                          <div className="mt-1 whitespace-pre-wrap break-words">
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
                        <Textarea
                          value={freeTextResponse}
                          onChange={(e) =>
                            setFreeTextResponse(e.target.value)
                          }
                          placeholder={
                            detail.state.pendingPrompt.freeTextPlaceholder ||
                            'Add an optional note or free-text response'
                          }
                        />
                        <Button onClick={onRespond}>Submit Response</Button>
                      </>
                    )}
                  </div>
                </div>
              ) : null}

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="min-w-0 rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Current Stage DAGs
                  </h2>
                  <div className="space-y-2 text-sm">
                    {detail.allowedDags.length ? (
                      detail.allowedDags.map((dag) => (
                        <button
                          key={dag.name}
                          type="button"
                          onClick={() => setSelectedDAG(dag.name)}
                          className="w-full rounded-md border p-2 text-left transition hover:bg-muted/50"
                        >
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
                        </button>
                      ))
                    ) : (
                      <div className="rounded-md border border-dashed p-3 text-muted-foreground">
                        No DAGs are assigned to the current stage.
                      </div>
                    )}
                  </div>
                </div>

                <div className="min-w-0 rounded-lg border p-4">
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
                            <div className="mt-2 flex flex-wrap gap-1">
                              {allowedNames.map((dagName) => (
                                <button
                                  key={dagName}
                                  type="button"
                                  onClick={() => setSelectedDAG(dagName)}
                                  className="rounded bg-muted px-2 py-0.5 text-xs text-foreground transition hover:bg-accent"
                                >
                                  {dagName}
                                </button>
                              ))}
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
              </div>

              <div className="min-w-0 rounded-lg border p-4">
                <h2 className="mb-3 text-sm font-semibold">Recent Runs</h2>
                <div className="space-y-2">
                  {mergedRuns.length ? (
                    mergedRuns.map((run) => (
                      <button
                        key={`${run.name}:${run.dagRunId}`}
                        type="button"
                        onClick={() =>
                          setSelectedDAGRun({
                            name: run.name,
                            dagRunId: run.dagRunId,
                          })
                        }
                        className="grid w-full gap-2 rounded-md border p-2 text-left text-sm transition hover:bg-muted/50 lg:grid-cols-[1fr_180px_160px]"
                      >
                        <div>
                          <div className="flex items-center gap-2">
                            <div className="font-medium">{run.name}</div>
                            {run.isCurrent ? (
                              <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
                                current
                              </span>
                            ) : null}
                          </div>
                          <div className="text-xs text-muted-foreground">
                            {run.dagRunId}
                          </div>
                        </div>
                        <div>
                          <StatusChip
                            status={dagRunStatusToStatus(run.status)}
                            size="xs"
                          >
                            {run.status}
                          </StatusChip>
                        </div>
                        <div className="text-xs text-muted-foreground">
                          {run.finishedAt || run.startedAt || ''}
                        </div>
                      </button>
                    ))
                  ) : (
                    <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
                      No child DAG runs yet.
                    </div>
                  )}
                </div>
              </div>

              <div className="grid gap-4 lg:grid-cols-2">
                <div className="min-w-0 rounded-lg border p-4">
                  <h2 className="mb-3 text-sm font-semibold">
                    Full Session Transcript
                  </h2>
                  {(detail.messages || []).length ? (
                    <div className="max-h-[28rem] min-w-0 space-y-2 overflow-x-auto overflow-y-auto">
                      {(detail.messages || []).slice(-40).map((message) => (
                        <div
                          key={message.id}
                          className="min-w-0 rounded-md border p-2 text-sm"
                        >
                          <div className="mb-1 flex flex-wrap items-center justify-between gap-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                            <span>{message.type}</span>
                            {message.createdAt ? (
                              <span className="normal-case tracking-normal">
                                {new Date(message.createdAt).toLocaleString()}
                              </span>
                            ) : null}
                          </div>
                          {message.content ? (
                            <div className="whitespace-pre-wrap break-words">
                              {message.content}
                            </div>
                          ) : null}
                          {message.userPrompt?.question ? (
                            <div className="whitespace-pre-wrap break-words">
                              {message.userPrompt.question}
                            </div>
                          ) : null}
                          {message.toolResults?.length ? (
                            <div className="mt-2 space-y-1">
                              {message.toolResults.map((result, index) => (
                                <div
                                  key={index}
                                  className="rounded bg-muted p-2 text-xs whitespace-pre-wrap break-words"
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

                <div className="min-w-0 rounded-lg border p-4">
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
                      className="min-h-[28rem] w-full min-w-0 font-mono text-xs"
                    />
                  ) : (
                    <pre className="max-h-[28rem] max-w-full overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-xs">
                      {specQuery.data?.spec || ''}
                    </pre>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="p-8 text-sm text-muted-foreground md:h-full md:overflow-auto md:p-6">
              Automata definition not found.
            </div>
          )}
        </section>
        </div>
      </div>
      {selectedDAGRun ? (
        <DAGRunDetailsModal
          name={selectedDAGRun.name}
          dagRunId={selectedDAGRun.dagRunId}
          isOpen={!!selectedDAGRun}
          onClose={() => setSelectedDAGRun(null)}
        />
      ) : null}
      {selectedDAG ? (
        <DAGDetailsModal
          fileName={selectedDAG}
          isOpen={!!selectedDAG}
          onClose={() => setSelectedDAG(null)}
        />
      ) : null}
    </>
  );
}

export default AutomataPage;
