// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { Search } from 'lucide-react';

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
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  isValidAutomataIconUrl,
  parseAutomataScheduleText,
  validateAutomataScheduleExpressions,
} from '@/features/automata/detail-utils';
import {
  applySelectedWorkspaceToAutomataTags,
  workspaceTagForAutomataSelection,
} from '@/features/automata/workspace';
import { useClient, useQuery } from '@/hooks/api';

const AUTOMATA_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_]*$/;
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

function quoteYAML(value: string): string {
  return JSON.stringify(value.trim());
}

function buildAutomataSpec(input: {
  nickname: string;
  iconUrl: string;
  description: string;
  goal: string;
  standingInstruction: string;
  resetOnFinish: boolean;
  schedule: string[];
  tags: string[];
  allowedDAGNames: string[];
}): string {
  const nickname = input.nickname.trim();
  const iconUrl = input.iconUrl.trim();
  const description = input.description.trim();
  const standingInstruction = input.standingInstruction.trim();
  const lines: string[] = [];

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
  if (standingInstruction) {
    lines.push(`standing_instruction: ${quoteYAML(standingInstruction)}`);
  }
  if (input.resetOnFinish) {
    lines.push('reset_on_finish: true');
  }
  if (input.schedule.length === 1) {
    lines.push(`schedule: ${quoteYAML(input.schedule[0] || '')}`);
  } else if (input.schedule.length > 1) {
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
  const scheduleError = validateAutomataScheduleExpressions(input.schedule);
  if (scheduleError) {
    return scheduleError;
  }
  if (input.allowedDAGNames.length === 0) {
    return 'Select at least one allowed DAG.';
  }
  return null;
}

function DAGNamePicker({
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
  const [searchQuery, setSearchQuery] = React.useState('');
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

  const toggleSelection = (fileName: string) => {
    if (selectedNameSet.has(fileName)) {
      onChange(selectedNames.filter((name) => name !== fileName));
      return;
    }
    onChange([...selectedNames, fileName]);
  };

  return (
    <div className="space-y-3">
      <div className="flex min-h-8 flex-wrap gap-1">
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
      <div className="relative">
        <Search className="pointer-events-none absolute left-2 top-2 h-4 w-4 text-muted-foreground" />
        <Input
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.target.value)}
          placeholder="Search DAGs"
          disabled={disabled}
          className="pl-8"
        />
      </div>
      <div className="max-h-64 overflow-y-auto rounded-md border p-1">
        {filteredDAGs.length ? (
          filteredDAGs.map((dag) => {
            const selected = selectedNameSet.has(dag.fileName);
            return (
              <button
                key={dag.fileName}
                type="button"
                onClick={() => toggleSelection(dag.fileName)}
                disabled={disabled}
                className={`flex w-full items-start justify-between gap-3 rounded px-3 py-2 text-left text-sm hover:bg-accent ${selected ? 'bg-accent' : ''}`}
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
    </div>
  );
}

export function AutomataCreateModal({
  open,
  onClose,
  selectedWorkspace,
  remoteNode,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  selectedWorkspace: string;
  remoteNode: string;
  onCreated: (name: string) => void | Promise<void>;
}): React.ReactElement {
  const client = useClient();
  const selectedWorkspaceTag =
    workspaceTagForAutomataSelection(selectedWorkspace);
  const [createName, setCreateName] = React.useState('');
  const [createNickname, setCreateNickname] = React.useState('');
  const [createIconUrl, setCreateIconUrl] = React.useState('');
  const [createDescription, setCreateDescription] = React.useState('');
  const [createGoal, setCreateGoal] = React.useState('');
  const [createStandingInstruction, setCreateStandingInstruction] =
    React.useState('');
  const [createResetOnFinish, setCreateResetOnFinish] = React.useState(false);
  const [createSchedule, setCreateSchedule] = React.useState('');
  const [createTags, setCreateTags] = React.useState('');
  const [createAllowedDAGNames, setCreateAllowedDAGNames] = React.useState<
    string[]
  >([]);
  const [createError, setCreateError] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);

  const dagListQuery = useQuery(
    '/dags',
    open
      ? {
          params: {
            query: {
              perPage: 500,
              remoteNode: remoteNode || undefined,
              labels: selectedWorkspaceTag,
            },
          },
        }
      : null,
    { refreshInterval: 15000 }
  );

  const availableDAGOptions = React.useMemo<DAGOption[]>(() => {
    return (dagListQuery.data?.dags || []).map((dag) => ({
      fileName: dag.fileName,
      name: dag.dag?.name || dag.fileName,
    }));
  }, [dagListQuery.data?.dags]);

  const resetForm = React.useCallback(() => {
    setCreateName('');
    setCreateNickname('');
    setCreateIconUrl('');
    setCreateDescription('');
    setCreateGoal('');
    setCreateStandingInstruction('');
    setCreateResetOnFinish(false);
    setCreateSchedule('');
    setCreateTags(selectedWorkspaceTag || '');
    setCreateAllowedDAGNames([]);
    setCreateError('');
    setIsCreating(false);
  }, [selectedWorkspaceTag]);

  React.useEffect(() => {
    if (open) {
      resetForm();
    }
  }, [open, resetForm]);

  const handleClose = () => {
    if (isCreating) {
      return;
    }
    onClose();
  };

  const onCreate = async () => {
    const parsedSchedule = parseAutomataScheduleText(createSchedule);
    const validationError = validateAutomataCreateForm({
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
            resetOnFinish: createResetOnFinish,
            schedule: parsedSchedule,
            tags: applySelectedWorkspaceToAutomataTags(
              parseTagInput(createTags),
              selectedWorkspace
            ),
            allowedDAGNames: createAllowedDAGNames,
          }),
        },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to create automata');
      }
      await onCreated(automataName);
      onClose();
      resetForm();
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : 'Failed to create automata'
      );
      setIsCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && handleClose()}>
      <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create Automata</DialogTitle>
          <DialogDescription>
            Configure the Automata, its task scope, and its allowlisted DAGs.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="grid gap-2">
            <Label htmlFor="automata-create-name">Name</Label>
            <Input
              id="automata-create-name"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder="software_dev"
              autoFocus
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="automata-create-nickname">Nickname</Label>
            <Input
              id="automata-create-nickname"
              value={createNickname}
              onChange={(event) => setCreateNickname(event.target.value)}
              placeholder="Build Captain"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="automata-create-icon-url">Image URL</Label>
            <Input
              id="automata-create-icon-url"
              value={createIconUrl}
              onChange={(event) => setCreateIconUrl(event.target.value)}
              placeholder="https://cdn.example.com/automata/build-captain.png"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="automata-create-description">Description</Label>
            <Input
              id="automata-create-description"
              value={createDescription}
              onChange={(event) => setCreateDescription(event.target.value)}
              placeholder="Automates one software delivery workflow"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="automata-create-goal">Goal</Label>
            <Textarea
              id="automata-create-goal"
              value={createGoal}
              onChange={(event) => setCreateGoal(event.target.value)}
              placeholder="Complete the assigned task and leave it ready for review"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="automata-create-standing-instruction">
                Standing Instruction
              </Label>
              <Textarea
                id="automata-create-standing-instruction"
                value={createStandingInstruction}
                onChange={(event) =>
                  setCreateStandingInstruction(event.target.value)
                }
                placeholder="Handle each cycle and work through the task list."
                disabled={isCreating}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="automata-create-schedule">Schedule</Label>
              <Textarea
                id="automata-create-schedule"
                value={createSchedule}
                onChange={(event) => setCreateSchedule(event.target.value)}
                placeholder={'0 * * * *\n30 9 * * 1-5'}
                disabled={isCreating}
                rows={4}
              />
            </div>
          </div>

          <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
            <Label htmlFor="automata-create-reset-on-finish">
              Reset on finish
            </Label>
            <Switch
              id="automata-create-reset-on-finish"
              checked={createResetOnFinish}
              onCheckedChange={setCreateResetOnFinish}
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <Label htmlFor="automata-create-tags">Tags</Label>
            <Textarea
              id="automata-create-tags"
              value={createTags}
              onChange={(event) => setCreateTags(event.target.value)}
              placeholder={'workspace=engineering, owner=team-ai'}
              disabled={isCreating}
              rows={2}
            />
          </div>

          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <Label>Allowed DAGs</Label>
              <span className="text-xs text-muted-foreground">
                {dagListQuery.isLoading
                  ? 'Loading DAGs...'
                  : `${availableDAGOptions.length} available`}
              </span>
            </div>
            <DAGNamePicker
              availableDAGs={availableDAGOptions}
              selectedNames={createAllowedDAGNames}
              onChange={setCreateAllowedDAGNames}
              disabled={isCreating}
            />
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
            onClick={handleClose}
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
  );
}
