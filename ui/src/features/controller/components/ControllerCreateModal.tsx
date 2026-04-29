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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import {
  isValidControllerIconUrl,
  parseControllerCronScheduleText,
  validateControllerCronScheduleExpressions,
} from '@/features/controller/detail-utils';
import {
  applySelectedWorkspaceToControllerLabels,
  workspaceTagForControllerSelection,
} from '@/features/controller/workspace';
import { useClient, useQuery } from '@/hooks/api';

const CONTROLLER_NAME_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9_]*$/;
const MAX_CONTROLLER_NICKNAME_LENGTH = 80;
const MAX_CONTROLLER_ICON_URL_LENGTH = 2048;
const MAX_DAG_PICKER_MATCHES = 25;
type ControllerTriggerType = 'manual' | 'cron';

export type DAGOption = {
  fileName: string;
  name: string;
};

function FieldLabel({
  htmlFor,
  label,
  required,
}: {
  htmlFor?: string;
  label: string;
  required?: boolean;
}): React.ReactElement {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Label htmlFor={htmlFor}>
        {label}
        {required ? (
          <span className="ml-1 text-destructive" aria-hidden="true">
            *
          </span>
        ) : null}
      </Label>
    </div>
  );
}

function parseLabelInput(value: string): string[] {
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

function buildControllerSpec(input: {
  nickname: string;
  iconUrl: string;
  description: string;
  goal: string;
  triggerPrompt: string;
  resetOnFinish: boolean;
  triggerType: ControllerTriggerType;
  cronSchedules: string[];
  labels: string[];
  workflowNames: string[];
}): string {
  const nickname = input.nickname.trim();
  const iconUrl = input.iconUrl.trim();
  const description = input.description.trim();
  const triggerPrompt = input.triggerPrompt.trim();
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
  if (input.resetOnFinish) {
    lines.push('reset_on_finish: true');
  }
  lines.push('trigger:');
  lines.push(`  type: ${quoteYAML(input.triggerType)}`);
  if (input.triggerType === 'cron') {
    lines.push('  schedules:');
    input.cronSchedules.forEach((expression) => {
      lines.push(`    - ${quoteYAML(expression)}`);
    });
    lines.push(`  prompt: ${quoteYAML(triggerPrompt)}`);
  }
  if (input.labels.length) {
    lines.push('labels:');
    input.labels.forEach((label) => {
      lines.push(`  - ${quoteYAML(label)}`);
    });
  }
  const workflowNames = Array.from(
    new Set(input.workflowNames.map((name) => name.trim()).filter(Boolean))
  );
  if (workflowNames.length > 0) {
    lines.push('workflows:');
    lines.push('  names:');
    workflowNames.forEach((workflowName) => {
      lines.push(`    - ${quoteYAML(workflowName)}`);
    });
  }

  lines.push('');
  lines.push('agent:');
  lines.push('  safeMode: true');
  lines.push('');

  return lines.join('\n');
}

function validateControllerCreateForm(input: {
  name: string;
  nickname: string;
  iconUrl: string;
  goal: string;
  triggerType: ControllerTriggerType;
  triggerPrompt: string;
  cronSchedules: string[];
  workflowNames: string[];
}): string | null {
  const name = input.name.trim();
  const nickname = input.nickname.trim();
  const iconUrl = input.iconUrl.trim();
  if (!name) {
    return 'Controller name is required.';
  }
  if (!CONTROLLER_NAME_PATTERN.test(name)) {
    return 'Controller name must start with a letter or number and use only letters, numbers, and underscores.';
  }
  if (nickname.includes('\n') || nickname.includes('\r')) {
    return 'Nickname must be a single line.';
  }
  if (nickname.length > MAX_CONTROLLER_NICKNAME_LENGTH) {
    return `Nickname must be ${MAX_CONTROLLER_NICKNAME_LENGTH} characters or fewer.`;
  }
  if (!isValidControllerIconUrl(iconUrl)) {
    return 'Icon URL must be an absolute http(s) URL or a root-relative path.';
  }
  if (iconUrl.length > MAX_CONTROLLER_ICON_URL_LENGTH) {
    return `Icon URL must be ${MAX_CONTROLLER_ICON_URL_LENGTH} characters or fewer.`;
  }
  if (input.triggerType === 'cron') {
    const scheduleError = validateControllerCronScheduleExpressions(
      input.cronSchedules
    );
    if (scheduleError) {
      return scheduleError;
    }
    if (input.cronSchedules.length === 0) {
      return 'Add at least one cron schedule for a cron-triggered Controller.';
    }
    if (!input.triggerPrompt.trim()) {
      return 'Add a trigger prompt for a cron-triggered Controller.';
    }
  }
  return null;
}

export function DAGNamePicker({
  availableDAGs,
  selectedNames,
  onChange,
  searchQuery,
  onSearchQueryChange,
  isLoading,
  disabled,
}: {
  availableDAGs: DAGOption[];
  selectedNames: string[];
  onChange: (names: string[]) => void;
  searchQuery?: string;
  onSearchQueryChange?: (query: string) => void;
  isLoading?: boolean;
  disabled?: boolean;
}): React.ReactElement {
  const [internalSearchQuery, setInternalSearchQuery] = React.useState('');
  const currentSearchQuery = searchQuery ?? internalSearchQuery;
  const setCurrentSearchQuery = onSearchQueryChange ?? setInternalSearchQuery;
  const selectedNameSet = React.useMemo(
    () => new Set(selectedNames),
    [selectedNames]
  );
  const searchText = currentSearchQuery.trim();
  const filteredDAGs = React.useMemo(() => {
    const query = searchText.toLowerCase();
    if (!query) {
      return [];
    }
    return availableDAGs
      .filter(
        (dag) =>
          dag.fileName.toLowerCase().includes(query) ||
          dag.name.toLowerCase().includes(query)
      )
      .slice(0, MAX_DAG_PICKER_MATCHES);
  }, [availableDAGs, searchText]);

  const toggleSelection = (fileName: string) => {
    if (selectedNameSet.has(fileName)) {
      onChange(selectedNames.filter((name) => name !== fileName));
      return;
    }
    onChange([...selectedNames, fileName]);
    setCurrentSearchQuery('');
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
                aria-label={`Remove ${dagName}`}
                title={`Remove ${dagName}`}
                className="rounded-sm px-1 py-0.5 text-[11px] font-medium text-muted-foreground transition hover:bg-background hover:text-foreground disabled:cursor-not-allowed disabled:opacity-50"
              >
                Remove
              </button>
            </span>
          ))
        ) : (
          <div className="text-xs text-muted-foreground">No workflows selected.</div>
        )}
      </div>
      <div className="relative">
        <Search className="pointer-events-none absolute left-2 top-2 h-4 w-4 text-muted-foreground" />
        <Input
          value={currentSearchQuery}
          onChange={(event) => setCurrentSearchQuery(event.target.value)}
          placeholder="Search workflows"
          disabled={disabled}
          className="pl-8"
        />
      </div>
      {searchText ? (
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
              {isLoading ? 'Loading workflows...' : 'No workflows found.'}
            </div>
          )}
        </div>
      ) : null}
    </div>
  );
}

export function ControllerCreateModal({
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
    workspaceTagForControllerSelection(selectedWorkspace);
  const [createName, setCreateName] = React.useState('');
  const [createNickname, setCreateNickname] = React.useState('');
  const [createIconUrl, setCreateIconUrl] = React.useState('');
  const [createDescription, setCreateDescription] = React.useState('');
  const [createGoal, setCreateGoal] = React.useState('');
  const [createTriggerPrompt, setCreateTriggerPrompt] =
    React.useState('');
  const [createResetOnFinish, setCreateResetOnFinish] = React.useState(false);
  const [createTriggerType, setCreateTriggerType] =
    React.useState<ControllerTriggerType>('manual');
  const [createSchedule, setCreateSchedule] = React.useState('');
  const [createLabels, setCreateLabels] = React.useState('');
  const [createWorkflowNames, setCreateWorkflowNames] = React.useState<
    string[]
  >([]);
  const [dagSearchQuery, setDagSearchQuery] = React.useState('');
  const [createError, setCreateError] = React.useState('');
  const [isCreating, setIsCreating] = React.useState(false);
  const dagSearchName = dagSearchQuery.trim();

  const dagListQuery = useQuery(
    '/dags',
    open && dagSearchName
      ? {
          params: {
            query: {
              perPage: MAX_DAG_PICKER_MATCHES,
              remoteNode: remoteNode || undefined,
              labels: selectedWorkspaceTag,
              name: dagSearchName,
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
    setCreateTriggerPrompt('');
    setCreateResetOnFinish(false);
    setCreateTriggerType('manual');
    setCreateSchedule('');
    setCreateLabels(selectedWorkspaceTag || '');
    setCreateWorkflowNames([]);
    setDagSearchQuery('');
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
    const parsedCronSchedules = parseControllerCronScheduleText(createSchedule);
    const validationError = validateControllerCreateForm({
      name: createName,
      nickname: createNickname,
      iconUrl: createIconUrl,
      goal: createGoal,
      triggerType: createTriggerType,
      triggerPrompt: createTriggerPrompt,
      cronSchedules: parsedCronSchedules,
      workflowNames: createWorkflowNames,
    });
    if (validationError) {
      setCreateError(validationError);
      return;
    }

    const controllerName = createName.trim();
    setCreateError('');
    setIsCreating(true);
    try {
      const { error: apiError } = await client.PUT('/controller/{name}/spec', {
        params: { path: { name: controllerName } },
        body: {
          spec: buildControllerSpec({
            nickname: createNickname,
            iconUrl: createIconUrl,
            description: createDescription,
            goal: createGoal,
            triggerPrompt: createTriggerPrompt,
            resetOnFinish: createResetOnFinish,
            triggerType: createTriggerType,
            cronSchedules: parsedCronSchedules,
            labels: applySelectedWorkspaceToControllerLabels(
              parseLabelInput(createLabels),
              selectedWorkspace
            ),
            workflowNames: createWorkflowNames,
          }),
        },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to create controller');
      }
      await onCreated(controllerName);
      onClose();
      resetForm();
    } catch (err) {
      setCreateError(
        err instanceof Error ? err.message : 'Failed to create controller'
      );
      setIsCreating(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(nextOpen) => !nextOpen && handleClose()}>
      <DialogContent className="max-h-[90vh] max-w-4xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>Create Controller</DialogTitle>
          <DialogDescription>
            Configure the Controller, its task scope, and its initial workflow context.
          </DialogDescription>
        </DialogHeader>

        <div className="grid gap-4">
          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-name"
              label="Name"
              required
            />
            <Input
              id="controller-create-name"
              value={createName}
              onChange={(event) => setCreateName(event.target.value)}
              placeholder="software_dev"
              autoFocus
              disabled={isCreating}
              aria-required={true}
            />
            <div className="text-xs text-muted-foreground">
              Used as the controller ID. Letters, numbers, and underscores only.
            </div>
          </div>

          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-nickname"
              label="Nickname"
            />
            <Input
              id="controller-create-nickname"
              value={createNickname}
              onChange={(event) => setCreateNickname(event.target.value)}
              placeholder="Build Captain"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-icon-url"
              label="Image URL"
            />
            <Input
              id="controller-create-icon-url"
              value={createIconUrl}
              onChange={(event) => setCreateIconUrl(event.target.value)}
              placeholder="https://cdn.example.com/controller/build-captain.png"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-description"
              label="Description"
            />
            <Input
              id="controller-create-description"
              value={createDescription}
              onChange={(event) => setCreateDescription(event.target.value)}
              placeholder="Automates one software delivery workflow"
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-goal"
              label="Goal"
            />
            <Textarea
              id="controller-create-goal"
              value={createGoal}
              onChange={(event) => setCreateGoal(event.target.value)}
              placeholder="Complete the assigned task and leave it ready for review"
              disabled={isCreating}
            />
            <div className="text-xs text-muted-foreground">
              Leave blank if the controller should work from the task list and the start-time instruction alone.
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-trigger"
              label="Trigger"
              required
            />
              <Select
                value={createTriggerType}
                onValueChange={(value) =>
                  setCreateTriggerType(value as ControllerTriggerType)
                }
                disabled={isCreating}
              >
                <SelectTrigger id="controller-create-trigger">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="manual">Manual</SelectItem>
                  <SelectItem value="cron">Cron</SelectItem>
                </SelectContent>
              </Select>
              <div className="text-xs text-muted-foreground">
                {createTriggerType === 'cron'
                  ? 'Cron controllers start from the schedules below and need a stored prompt for each cycle.'
                  : 'Manual controllers start only when someone provides the instruction at run time.'}
              </div>
            </div>
          </div>

          {createTriggerType === 'cron' ? (
            <>
              <div className="grid gap-2">
                <FieldLabel
                  htmlFor="controller-create-schedule"
                  label="Cron Schedules"
                  required
                />
                <Textarea
                  id="controller-create-schedule"
                  value={createSchedule}
                  onChange={(event) => setCreateSchedule(event.target.value)}
                  placeholder={'0 * * * *\n30 9 * * 1-5'}
                  disabled={isCreating}
                  rows={4}
                  aria-required={true}
                />
                <div className="text-xs text-muted-foreground">
                  Use one cron expression per line.
                </div>
              </div>
              <div className="grid gap-2">
                <FieldLabel
                  htmlFor="controller-create-trigger-prompt"
                  label="Trigger Prompt"
                  required
                />
                <Textarea
                  id="controller-create-trigger-prompt"
                  value={createTriggerPrompt}
                  onChange={(event) => setCreateTriggerPrompt(event.target.value)}
                  placeholder="Handle each cycle and work through the task list."
                  disabled={isCreating}
                  rows={4}
                  aria-required={true}
                />
                <div className="text-xs text-muted-foreground">
                  Reused every time a cron tick starts a fresh controller cycle.
                </div>
              </div>
            </>
          ) : null}

          <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
            <FieldLabel
              htmlFor="controller-create-reset-on-finish"
              label="Reset on finish"
            />
            <Switch
              id="controller-create-reset-on-finish"
              checked={createResetOnFinish}
              onCheckedChange={setCreateResetOnFinish}
              disabled={isCreating}
            />
          </div>

          <div className="grid gap-2">
            <FieldLabel
              htmlFor="controller-create-labels"
              label="Labels"
            />
            <Textarea
              id="controller-create-labels"
              value={createLabels}
              onChange={(event) => setCreateLabels(event.target.value)}
              placeholder={'workspace=engineering, owner=team-ai'}
              disabled={isCreating}
              rows={2}
            />
            <div className="text-xs text-muted-foreground">
              {selectedWorkspaceTag
                ? `The current workspace label ${selectedWorkspaceTag} is included by default.`
                : 'Optional controller labels.'}
            </div>
          </div>

          <div className="grid gap-2">
            <div className="flex items-center justify-between gap-3">
              <FieldLabel
                label="Workflows"
              />
              <span className="text-xs text-muted-foreground">
                {dagListQuery.isLoading
                  ? 'Loading workflows...'
                  : `${createWorkflowNames.length} selected`}
              </span>
            </div>
            <div className="text-xs text-muted-foreground">
              Optional. Seed the controller with workflows it should start from.
            </div>
            <DAGNamePicker
              availableDAGs={availableDAGOptions}
              selectedNames={createWorkflowNames}
              onChange={setCreateWorkflowNames}
              searchQuery={dagSearchQuery}
              onSearchQueryChange={setDagSearchQuery}
              isLoading={dagListQuery.isLoading}
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
            {isCreating ? 'Creating...' : 'Create Controller'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
