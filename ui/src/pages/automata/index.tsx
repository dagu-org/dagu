import React from 'react';
import { createPortal } from 'react-dom';
import { useNavigate, useParams } from 'react-router-dom';

import {
  AutomataDisplayStatus,
  AutomataKind,
  Status,
  type components,
} from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { AppBarContext } from '@/contexts/AppBarContext';
import { AutomataAvatar } from '@/features/automata/components/AutomataAvatar';
import { AutomataDetailSurface } from '@/features/automata/components/AutomataDetailSurface';
import {
  isValidAutomataIconUrl,
  parseAutomataScheduleText,
  validateAutomataScheduleExpressions,
} from '@/features/automata/detail-utils';
import { useAutomataDetailController } from '@/features/automata/hooks/useAutomataDetail';
import { useClient, useQuery } from '@/hooks/api';
import { cn } from '@/lib/utils';
import LoadingIndicator from '@/ui/LoadingIndicator';
import StatusChip from '@/ui/StatusChip';

type AutomataSummary = components['schemas']['AutomataSummary'];
type AutomataKindValue = components['schemas']['AutomataKind'];
type AutomataDisplayState = components['schemas']['AutomataDisplayStatus'];

const AUTOMATA_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_]*$/;
const DEFAULT_AUTOMATA_KIND: AutomataKindValue = AutomataKind.workflow;
const MAX_AUTOMATA_NICKNAME_LENGTH = 80;
const MAX_AUTOMATA_ICON_URL_LENGTH = 2048;

type DAGOption = {
  fileName: string;
  name: string;
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
          <div className="text-xs text-muted-foreground">No DAGs selected.</div>
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
                          <span className="shrink-0 text-primary">
                            Selected
                          </span>
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
  kind: AutomataKindValue;
  nickname: string;
  iconUrl: string;
  description: string;
  goal: string;
  standingInstruction: string;
  schedule: string[];
  tags: string[];
  allowedDAGNames: string[];
}): string {
  const nickname = input.nickname.trim();
  const iconUrl = input.iconUrl.trim();
  const description = input.description.trim();
  const standingInstruction = input.standingInstruction.trim();
  const lines: string[] = [];

  if (input.kind !== DEFAULT_AUTOMATA_KIND) {
    lines.push(`kind: ${input.kind}`);
  }

  if (nickname) {
    lines.push(`nickname: ${quoteYAML(nickname)}`);
  }

  if (iconUrl) {
    lines.push(`icon_url: ${quoteYAML(iconUrl)}`);
  }

  if (description) {
    lines.push(`description: ${quoteYAML(description)}`);
  }

  if (input.goal.trim()) {
    lines.push(`goal: ${quoteYAML(input.goal)}`);
  }

  if (input.kind === AutomataKind.service && standingInstruction) {
    lines.push(`standing_instruction: ${quoteYAML(standingInstruction)}`);
  }

  if (input.kind === AutomataKind.service && input.schedule.length === 1) {
    lines.push(`schedule: ${quoteYAML(input.schedule[0] || '')}`);
  } else if (input.kind === AutomataKind.service && input.schedule.length > 1) {
    lines.push('schedule:');
    input.schedule.forEach((expression) => {
      lines.push(`  - ${quoteYAML(expression)}`);
    });
  }

  if (input.tags.length) {
    lines.push('tags:');
    input.tags.forEach((tag) => {
      lines.push(`  - ${quoteYAML(tag)}`);
    });
  }

  lines.push('allowed_dags:');
  lines.push('  names:');
  Array.from(
    new Set(input.allowedDAGNames.map((name) => name.trim()).filter(Boolean))
  ).forEach((dagName) => {
    lines.push(`    - ${quoteYAML(dagName)}`);
  });

  lines.push('');
  lines.push('agent:');
  lines.push('  safeMode: true');
  lines.push('');

  return lines.join('\n');
}

function validateAutomataCreateForm(input: {
  kind: AutomataKindValue;
  name: string;
  nickname: string;
  iconUrl: string;
  goal: string;
  schedule: string[];
  allowedDAGNames: string[];
}): string | null {
  const name = input.name.trim();
  const nickname = input.nickname.trim();
  const iconUrl = input.iconUrl.trim();
  if (!name) {
    return 'Automata name is required.';
  }
  if (!AUTOMATA_NAME_PATTERN.test(name)) {
    return 'Automata name must start with a letter or number and use only letters, numbers, and underscores.';
  }
  if (nickname.includes('\n') || nickname.includes('\r')) {
    return 'Nickname must be a single line.';
  }
  if (nickname.length > MAX_AUTOMATA_NICKNAME_LENGTH) {
    return `Nickname must be ${MAX_AUTOMATA_NICKNAME_LENGTH} characters or fewer.`;
  }
  if (!isValidAutomataIconUrl(iconUrl)) {
    return 'Icon URL must be an absolute http(s) URL or a root-relative path.';
  }
  if (iconUrl.length > MAX_AUTOMATA_ICON_URL_LENGTH) {
    return `Icon URL must be ${MAX_AUTOMATA_ICON_URL_LENGTH} characters or fewer.`;
  }
  if (input.kind === AutomataKind.service) {
    const scheduleError = validateAutomataScheduleExpressions(input.schedule);
    if (scheduleError) {
      return scheduleError;
    }
  }
  if (input.allowedDAGNames.length === 0) {
    return 'Select at least one allowed DAG.';
  }
  return null;
}

function automataDisplayName(item: {
  name: string;
  nickname?: string | null;
}): string {
  return item.nickname?.trim() || item.name;
}

function displayStatusClass(state?: AutomataDisplayState | string): string {
  switch (state) {
    case AutomataDisplayStatus.running:
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case AutomataDisplayStatus.paused:
      return 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100';
    case AutomataDisplayStatus.finished:
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
  const [createKind, setCreateKind] =
    React.useState<AutomataKindValue>(DEFAULT_AUTOMATA_KIND);
  const [createNickname, setCreateNickname] = React.useState('');
  const [createIconUrl, setCreateIconUrl] = React.useState('');
  const [createDescription, setCreateDescription] = React.useState('');
  const [createGoal, setCreateGoal] = React.useState('');
  const [createStandingInstruction, setCreateStandingInstruction] =
    React.useState('');
  const [createSchedule, setCreateSchedule] = React.useState('');
  const [createTags, setCreateTags] = React.useState('');
  const [createAllowedDAGNames, setCreateAllowedDAGNames] = React.useState<
    string[]
  >([]);
  const [createError, setCreateError] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);

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

  const detailController = useAutomataDetailController({
    name,
    enabled: !!name,
    onUpdated: async () => {
      await listQuery.mutate();
    },
  });
  const availableDAGOptions = React.useMemo<DAGOption[]>(() => {
    return (dagListQuery.data?.dags || []).map((dag) => ({
      fileName: dag.fileName,
      name: dag.dag?.name || dag.fileName,
    }));
  }, [dagListQuery.data?.dags]);
  const listItems = (listQuery.data?.automata || []) as AutomataSummary[];

  const resetCreateForm = () => {
    setCreateName('');
    setCreateKind(DEFAULT_AUTOMATA_KIND);
    setCreateNickname('');
    setCreateIconUrl('');
    setCreateDescription('');
    setCreateGoal('');
    setCreateStandingInstruction('');
    setCreateSchedule('');
    setCreateTags('');
    setCreateAllowedDAGNames([]);
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
    const parsedSchedule = parseAutomataScheduleText(createSchedule);
    const validationError = validateAutomataCreateForm({
      kind: createKind,
      name: createName,
      nickname: createNickname,
      iconUrl: createIconUrl,
      goal: createGoal,
      schedule: parsedSchedule,
      allowedDAGNames: createAllowedDAGNames,
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
            nickname: createNickname,
            iconUrl: createIconUrl,
            description: createDescription,
            goal: createGoal,
            standingInstruction: createStandingInstruction,
            schedule: parsedSchedule,
            kind: createKind,
            tags: parseTagInput(createTags),
            allowedDAGNames: createAllowedDAGNames,
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
                {listItems.length === 0 ? (
                  <div className="rounded-lg border border-dashed p-4 text-sm text-muted-foreground">
                    No Automata defined yet.
                    <div className="mt-3">
                      <Button size="sm" onClick={openCreateDialog}>
                        Create Automata
                      </Button>
                    </div>
                  </div>
                ) : null}
                {listItems.map((item) => (
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
                      <div className="flex min-w-0 items-start gap-3">
                        <AutomataAvatar
                          name={item.name}
                          nickname={item.nickname}
                          iconUrl={item.iconUrl}
                          className="h-10 w-10"
                        />
                        <div className="min-w-0">
                          <div className="truncate font-medium">
                            {automataDisplayName(item)}
                          </div>
                          {item.nickname ? (
                            <div className="mt-0.5 truncate font-mono text-[11px] text-muted-foreground">
                              {item.name}
                            </div>
                          ) : null}
                          <div className="mt-1 text-xs text-muted-foreground">
                            {item.instruction || item.goal}
                          </div>
                        </div>
                      </div>
                      <span
                        className={`rounded-full px-2 py-1 text-[11px] font-medium ${displayStatusClass(item.displayStatus || item.state)}`}
                      >
                        {item.displayStatus || item.state}
                      </span>
                    </div>
                    <div className="mt-2 flex items-center justify-between text-xs text-muted-foreground">
                      <span>
                        Tasks: {item.doneTaskCount || 0}/
                        {(item.doneTaskCount || 0) + (item.openTaskCount || 0)}
                      </span>
                      {item.currentRun ? (
                        <StatusChip
                          status={dagRunStatusToStatus(item.currentRun.status)}
                          size="xs"
                        >
                          {item.currentRun.status}
                        </StatusChip>
                      ) : null}
                    </div>
                    {item.busy ? (
                      <div className="mt-2 inline-flex rounded-full bg-amber-100 px-2 py-1 text-[11px] font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-200">
                        busy
                      </div>
                    ) : null}
                    {item.needsInput ? (
                      <div className="mt-2 inline-flex rounded-full bg-rose-100 px-2 py-1 text-[11px] font-medium text-rose-900 dark:bg-rose-900/40 dark:text-rose-200">
                        needs input
                      </div>
                    ) : null}
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
                      Configure metadata and the automata-wide allowed DAG list.
                      Manage task list items after creation.
                    </p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Only fields marked required must be filled in now.
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
                    <Label htmlFor="automata-name">Name (Required)</Label>
                    <Input
                      id="automata-name"
                      value={createName}
                      onChange={(e) => setCreateName(e.target.value)}
                      placeholder="software_dev"
                      autoFocus
                      disabled={isCreating}
                    />
                    <div className="text-xs text-muted-foreground">
                      Must start with a letter or number. Use letters, numbers,
                      and underscores only.
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="automata-kind">Kind</Label>
                    <Select
                      value={createKind}
                      onValueChange={(value) =>
                        setCreateKind(value as AutomataKindValue)
                      }
                      disabled={isCreating}
                    >
                      <SelectTrigger id="automata-kind">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={AutomataKind.workflow}>
                          Workflow
                        </SelectItem>
                        <SelectItem value={AutomataKind.service}>
                          Service
                        </SelectItem>
                      </SelectContent>
                    </Select>
                    <div className="text-xs text-muted-foreground">
                      `Workflow` finishes when the current mission is complete.
                      `Service` stays live after activation and can wake on
                      operator messages or schedule ticks.
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="automata-nickname">Nickname (Optional)</Label>
                    <Input
                      id="automata-nickname"
                      value={createNickname}
                      onChange={(e) => setCreateNickname(e.target.value)}
                      placeholder="Build Captain"
                      disabled={isCreating}
                    />
                    <div className="text-xs text-muted-foreground">
                      Optional short label shown in cockpit cards and detail
                      headers.
                    </div>
                  </div>

                  <div className="grid gap-3 md:grid-cols-[auto,1fr] md:items-start">
                    <AutomataAvatar
                      name={createName.trim() || 'automata'}
                      nickname={createNickname}
                      iconUrl={createIconUrl}
                      className="h-16 w-16 rounded-2xl"
                    />
                    <div className="grid gap-2">
                      <Label htmlFor="automata-icon-url">
                        Icon Image URL (Optional)
                      </Label>
                      <Input
                        id="automata-icon-url"
                        value={createIconUrl}
                        onChange={(e) => setCreateIconUrl(e.target.value)}
                        placeholder="https://cdn.example.com/automata/build-captain.png"
                        disabled={isCreating}
                      />
                      <div className="text-xs text-muted-foreground">
                        Optional. Use an absolute
                        <span className="mx-1 font-mono text-foreground">
                          http(s)
                        </span>
                        URL or a root-relative path like
                        <span className="mx-1 font-mono text-foreground">
                          /assets/automata/build-captain.png
                        </span>
                        . A placeholder icon is shown when no image is set.
                      </div>
                    </div>
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="automata-description">
                      Description (Optional)
                    </Label>
                    <Input
                      id="automata-description"
                      value={createDescription}
                      onChange={(e) => setCreateDescription(e.target.value)}
                      placeholder="Automates one software delivery workflow"
                      disabled={isCreating}
                    />
                  </div>

                  <div className="grid gap-2">
                    <Label htmlFor="automata-goal">Goal (Optional)</Label>
                    <Textarea
                      id="automata-goal"
                      value={createGoal}
                      onChange={(e) => setCreateGoal(e.target.value)}
                      placeholder="Complete the assigned task and leave it ready for review"
                      disabled={isCreating}
                    />
                    <div className="text-xs text-muted-foreground">
                      Optional. Leave blank if the Automata should work from the
                      instruction, task list, and runtime context.
                    </div>
                  </div>

                  {createKind === AutomataKind.service ? (
                    <>
                      <div className="grid gap-2">
                        <Label htmlFor="automata-standing-instruction">
                          Standing Instruction (Optional)
                        </Label>
                        <Textarea
                          id="automata-standing-instruction"
                          value={createStandingInstruction}
                          onChange={(e) =>
                            setCreateStandingInstruction(e.target.value)
                          }
                          placeholder="Handle each service cycle and work through the task list."
                          disabled={isCreating}
                        />
                        <div className="text-xs text-muted-foreground">
                          Optional at creation time, but required before a
                          service can activate or run on schedule.
                        </div>
                      </div>

                      <div className="grid gap-2">
                        <Label htmlFor="automata-schedule">
                          Schedule (Optional)
                        </Label>
                        <Textarea
                          id="automata-schedule"
                          value={createSchedule}
                          onChange={(e) => setCreateSchedule(e.target.value)}
                          placeholder={'0 * * * *\n30 9 * * 1-5'}
                          disabled={isCreating}
                          rows={3}
                        />
                        <div className="text-xs text-muted-foreground">
                          Optional. Use one cron expression per line. Each due
                          tick starts a fresh cycle by reopening the Config task
                          template.
                        </div>
                      </div>
                    </>
                  ) : null}

                  <div className="grid gap-2">
                    <Label htmlFor="automata-tags">Tags (Optional)</Label>
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
                    <div>
                      <Label>Allowed DAGs (Required)</Label>
                      <div className="mt-1 text-xs text-muted-foreground">
                        These DAGs are available for the automata across its
                        full runtime.
                      </div>
                    </div>

                    <DAGNameMultiSelect
                      availableDAGs={availableDAGOptions}
                      selectedNames={createAllowedDAGNames}
                      onChange={setCreateAllowedDAGNames}
                      disabled={isCreating}
                    />

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
                  Select an Automata to inspect its status, config, live thread,
                  recent DAG runs, and task list.
                </div>
                <div>
                  <Button onClick={openCreateDialog}>Create Automata</Button>
                </div>
              </div>
            ) : detailController.isLoading ? (
              <LoadingIndicator />
            ) : detailController.detail ? (
              <div className="space-y-6 overflow-x-hidden p-4 md:h-full md:overflow-auto md:p-6">
                <AutomataDetailSurface
                  key={name}
                  controller={detailController}
                  renderHeaderActions={(controller) => (
                    <>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void controller.onDuplicate()}
                        disabled={!!controller.busyAction}
                      >
                        Duplicate
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void controller.onRename()}
                        disabled={!!controller.busyAction}
                      >
                        Rename
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void controller.onResetState()}
                        disabled={!!controller.busyAction}
                      >
                        Reset State
                      </Button>
                      <Button
                        variant="outline"
                        size="sm"
                        onClick={() => void controller.onDelete()}
                        disabled={!!controller.busyAction}
                      >
                        Delete
                      </Button>
                    </>
                  )}
                />
              </div>
            ) : (
              <div className="p-8 text-sm text-muted-foreground md:h-full md:overflow-auto md:p-6">
                Automata definition not found.
              </div>
            )}
          </section>
        </div>
      </div>
    </>
  );
}

export default AutomataPage;
