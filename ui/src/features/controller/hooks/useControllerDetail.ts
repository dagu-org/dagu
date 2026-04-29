// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { useNavigate } from 'react-router-dom';

import { useAvailableModels } from '@/features/agent/hooks/useAvailableModels';
import { useClient, useQuery } from '@/hooks/api';
import { whenEnabled } from '@/hooks/queryUtils';
import { updateControllerMetadataInSpec } from '@/features/controller/spec';
import { workspaceTagForControllerSelection } from '@/features/controller/workspace';
import {
  buildControllerThread,
  type ControllerDetail,
  type ControllerPendingTurnMessage,
  type ControllerRunSummary,
  type ControllerTask,
  type ControllerTaskTemplate,
  formatControllerCronScheduleText,
  isValidControllerIconUrl,
  parseControllerCronScheduleText,
  taskCounts,
  validateControllerCronScheduleExpressions,
} from '@/features/controller/detail-utils';

type MutationCallback = (() => void | Promise<void>) | undefined;
const CLONE_NAME_SUFFIX_LENGTH = 6;
const CLONE_NAME_SUFFIX_ALPHABET = 'abcdefghijklmnopqrstuvwxyz0123456789';
const DAG_PICKER_SEARCH_LIMIT = 25;
type ControllerTriggerType = 'manual' | 'cron';

function normalizeDAGNameList(items?: string[]): string[] {
  return Array.from(
    new Set((items || []).map((item) => item.trim()).filter(Boolean))
  );
}

function sameDAGNameList(a?: string[], b?: string[]): boolean {
  const left = normalizeDAGNameList(a);
  const right = normalizeDAGNameList(b);
  return (
    left.length === right.length &&
    left.every((item, index) => item === right[index])
  );
}

function randomCloneNameSuffix(): string {
  const cryptoObj = globalThis.crypto;
  const values =
    cryptoObj && 'getRandomValues' in cryptoObj
      ? cryptoObj.getRandomValues(new Uint8Array(CLONE_NAME_SUFFIX_LENGTH))
      : undefined;

  if (values) {
    return Array.from(
      values,
      (value) =>
        CLONE_NAME_SUFFIX_ALPHABET[value % CLONE_NAME_SUFFIX_ALPHABET.length]
    ).join('');
  }

  return Array.from({ length: CLONE_NAME_SUFFIX_LENGTH }, () => {
    const index = Math.floor(Math.random() * CLONE_NAME_SUFFIX_ALPHABET.length);
    return CLONE_NAME_SUFFIX_ALPHABET[index];
  }).join('');
}

function buildDefaultCloneName(name: string): string {
  return `${name}_${randomCloneNameSuffix()}`;
}

type ControllerConfirmationState =
  | {
      kind: 'deleteTask';
      title: string;
      buttonText: string;
      message: string;
      task: ControllerTaskTemplate;
    }
  | {
      kind: 'reset';
      title: string;
      buttonText: string;
      message: string;
    }
  | {
      kind: 'delete';
      title: string;
      buttonText: string;
      message: string;
    };

export type ControllerDetailController = ReturnType<
  typeof useControllerDetailController
>;

export function useControllerDetailController({
  name,
  enabled = true,
  onUpdated,
  onSelectedNameChange,
  onDeleted,
  selectedWorkspace = '',
  remoteNode = '',
}: {
  name?: string;
  enabled?: boolean;
  onUpdated?: () => void | Promise<void>;
  onSelectedNameChange?: (name: string) => void | Promise<void>;
  onDeleted?: () => void | Promise<void>;
  selectedWorkspace?: string;
  remoteNode?: string;
}) {
  const client = useClient();
  const navigate = useNavigate();
  const { models: availableModels } = useAvailableModels();

  const detailQuery = useQuery(
    '/controller/{name}',
    whenEnabled(enabled && !!name, {
      params: { path: { name: name || '' } },
    }),
    {
      refreshInterval: (data?: ControllerDetail) =>
        data?.state?.state === 'running' ||
        data?.state?.state === 'waiting' ||
        data?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useQuery(
    '/controller/{name}/spec',
    whenEnabled(enabled && !!name, {
      params: { path: { name: name || '' } },
    }),
    { refreshInterval: 15000 }
  );

  const detail = detailQuery.data;

  const [instructionDraft, setInstructionDraft] = React.useState('');
  const [operatorMessageDraft, setOperatorMessageDraft] = React.useState('');
  const [newTaskDescription, setNewTaskDescription] = React.useState('');
  const [iconUrlDraft, setIconUrlDraft] = React.useState('');
  const [descriptionDraft, setDescriptionDraft] = React.useState('');
  const [goalDraft, setGoalDraft] = React.useState('');
  const [triggerPromptDraft, setTriggerPromptDraft] =
    React.useState('');
  const [resetOnFinishDraft, setResetOnFinishDraft] = React.useState(false);
  const [triggerTypeDraft, setTriggerTypeDraft] =
    React.useState<ControllerTriggerType>('manual');
  const [cronScheduleDraft, setCronScheduleDraft] = React.useState('');
  const [workflowNamesDraft, setWorkflowNamesDraft] = React.useState<
    string[]
  >([]);
  const [workflowSearchQuery, setWorkflowSearchQuery] = React.useState('');
  const [taskEditDraft, setTaskEditDraft] =
    React.useState<ControllerTaskTemplate | null>(null);
  const [taskEditDescription, setTaskEditDescription] = React.useState('');
  const [modelDraft, setModelDraft] = React.useState('');
  const [isEditingMetadata, setIsEditingMetadata] = React.useState(false);
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [actionError, setActionError] = React.useState('');
  const [busyAction, setBusyAction] = React.useState<string | null>(null);
  const [confirmation, setConfirmation] =
    React.useState<ControllerConfirmationState | null>(null);

  const [isEditingSpec, setIsEditingSpec] = React.useState(false);
  const [specDraft, setSpecDraft] = React.useState('');
  const [specError, setSpecError] = React.useState('');
  const [isSavingSpec, setIsSavingSpec] = React.useState(false);
  const selectedWorkspaceTag =
    workspaceTagForControllerSelection(selectedWorkspace);
  const workflowSearchName = workflowSearchQuery.trim();
  const dagListQuery = useQuery(
    '/dags',
    whenEnabled(enabled && !!workflowSearchName, {
      params: {
        query: {
          perPage: DAG_PICKER_SEARCH_LIMIT,
          remoteNode: remoteNode || undefined,
          labels: selectedWorkspaceTag,
          name: workflowSearchName,
        },
      },
    }),
    { refreshInterval: 15000 }
  );

  React.useEffect(() => {
    setInstructionDraft('');
    setOperatorMessageDraft('');
    setNewTaskDescription('');
    setWorkflowSearchQuery('');
    setTaskEditDraft(null);
    setTaskEditDescription('');
    setActionError('');
  }, [name]);

  React.useEffect(() => {
    if (!isEditingMetadata) {
      const persistedTriggerType =
        detail?.definition?.trigger?.type === 'cron' ? 'cron' : 'manual';
      const persistedSchedules =
        persistedTriggerType === 'cron'
          ? detail?.definition?.trigger?.schedules
          : [];
      const persistedTriggerPrompt =
        persistedTriggerType === 'cron'
          ? detail?.definition?.trigger?.prompt || ''
          : '';
      setDescriptionDraft(detail?.definition?.description || '');
      setIconUrlDraft(detail?.definition?.iconUrl || '');
      setGoalDraft(detail?.definition?.goal || '');
      setTriggerPromptDraft(persistedTriggerPrompt);
      setResetOnFinishDraft(!!detail?.definition?.resetOnFinish);
      setTriggerTypeDraft(persistedTriggerType);
      setCronScheduleDraft(formatControllerCronScheduleText(persistedSchedules));
      setWorkflowNamesDraft(
        normalizeDAGNameList(detail?.definition?.workflows?.names)
      );
      setModelDraft(detail?.definition?.agent?.model || '');
    }
  }, [
    detail?.definition?.workflows?.names,
    detail?.definition?.description,
    detail?.definition?.iconUrl,
    detail?.definition?.goal,
    detail?.definition?.resetOnFinish,
    detail?.definition?.trigger?.prompt,
    detail?.definition?.trigger?.schedules,
    detail?.definition?.trigger?.type,
    detail?.definition?.agent?.model,
    isEditingMetadata,
  ]);

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

  const mergedRuns = React.useMemo(() => {
    if (!detail) {
      return [] as Array<ControllerRunSummary & { isCurrent?: boolean }>;
    }
    const seen = new Set<string>();
    const items: Array<ControllerRunSummary & { isCurrent?: boolean }> = [];
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

  const queuedTurnMessages = React.useMemo<ControllerPendingTurnMessage[]>(
    () => detail?.state?.pendingTurnMessages || [],
    [detail?.state?.pendingTurnMessages]
  );

  const threadItems = React.useMemo(
    () => buildControllerThread(detail?.messages, queuedTurnMessages),
    [detail?.messages, queuedTurnMessages]
  );

  const availableDAGOptions = React.useMemo(
    () =>
      (dagListQuery.data?.dags || []).map((dag) => ({
        fileName: dag.fileName,
        name: dag.dag?.name || dag.fileName,
      })),
    [dagListQuery.data?.dags]
  );

  const lifecycleState = detail?.state?.state ?? '';
  const controllerStatus = detail?.controllerStatus;
  const runtimeControllerReady = controllerStatus?.state === 'ready';
  const displayStatus =
    detail?.state?.displayStatus ?? detail?.state?.state ?? '';
  const triggerType =
    detail?.definition?.trigger?.type === 'cron' ? 'cron' : 'manual';
  const persistedTriggerPrompt =
    triggerType === 'cron' ? detail?.definition?.trigger?.prompt || '' : '';
  const persistedCronSchedules =
    triggerType === 'cron' ? detail?.definition?.trigger?.schedules || [] : [];
  const taskSummary = React.useMemo(
    () => taskCounts(detail?.state?.tasks),
    [detail?.state?.tasks]
  );
  const cronScheduleExpressions = React.useMemo(
    () => parseControllerCronScheduleText(cronScheduleDraft),
    [cronScheduleDraft]
  );
  const effectiveCronSchedules =
    triggerTypeDraft === 'cron' ? cronScheduleExpressions : [];
  const canStartTask =
    runtimeControllerReady &&
    triggerType === 'manual' &&
    (lifecycleState === 'idle' || lifecycleState === 'finished');
  const canSendOperatorMessage =
    !!detail &&
    runtimeControllerReady &&
    !detail.state.pendingPrompt &&
    (lifecycleState === 'running' ||
      lifecycleState === 'waiting' ||
      lifecycleState === 'paused');
  const canPause =
    runtimeControllerReady &&
    (lifecycleState === 'running' || lifecycleState === 'waiting');
  const canResume = runtimeControllerReady && lifecycleState === 'paused';
  const cronSchedulesConfigured =
    triggerType === 'cron' && persistedCronSchedules.length > 0;

  const descriptionChanged =
    descriptionDraft.trim() !== (detail?.definition?.description || '').trim();
  const iconUrlChanged =
    iconUrlDraft.trim() !== (detail?.definition?.iconUrl || '').trim();
  const goalChanged =
    goalDraft.trim() !== (detail?.definition?.goal || '').trim();
  const triggerPromptChanged =
    triggerPromptDraft.trim() !== persistedTriggerPrompt.trim();
  const resetOnFinishChanged =
    resetOnFinishDraft !== !!detail?.definition?.resetOnFinish;
  const triggerTypeChanged = triggerTypeDraft !== triggerType;
  const cronSchedulesChanged =
    formatControllerCronScheduleText(effectiveCronSchedules) !==
    formatControllerCronScheduleText(persistedCronSchedules);
  const workflowNamesChanged = !sameDAGNameList(
    workflowNamesDraft,
    detail?.definition?.workflows?.names
  );
  const modelChanged =
    modelDraft.trim() !== (detail?.definition?.agent?.model || '').trim();
  const metadataChanged =
    descriptionChanged ||
    iconUrlChanged ||
    goalChanged ||
    triggerPromptChanged ||
    resetOnFinishChanged ||
    triggerTypeChanged ||
    cronSchedulesChanged ||
    workflowNamesChanged ||
    modelChanged;
  const workflowNames = React.useMemo(
    () => normalizeDAGNameList(workflowNamesDraft),
    [workflowNamesDraft]
  );
  const metadataValidationError = !isValidControllerIconUrl(iconUrlDraft)
    ? 'Icon URL must be an absolute http(s) URL or a root-relative path.'
    : iconUrlDraft.trim().length > 2048
      ? 'Icon URL must be 2048 characters or fewer.'
      : (triggerTypeDraft === 'cron' &&
          (validateControllerCronScheduleExpressions(cronScheduleExpressions) ||
            (cronScheduleExpressions.length === 0
              ? 'Add at least one cron schedule.'
              : !triggerPromptDraft.trim()
                ? 'Add a trigger prompt.'
                : null))) ||
        null;
  const metadataSaveDisabled =
    !name ||
    !detail ||
    !metadataChanged ||
    !!metadataValidationError ||
    !!busyAction ||
    isSavingSpec ||
    isEditingSpec;

  const refreshAfterAction = React.useCallback(
    async (...callbacks: MutationCallback[]) => {
      await Promise.all([detailQuery.mutate(), specQuery.mutate()]);
      if (onUpdated) {
        await onUpdated();
      }
      for (const callback of callbacks) {
        if (callback) {
          await callback();
        }
      }
    },
    [detailQuery, onUpdated, specQuery]
  );

  const onStart = React.useCallback(async () => {
    if (!name) return;
    setActionError('');
    try {
      const { error: apiError } = await client.POST('/controller/{name}/start', {
        params: { path: { name } },
        body: { instruction: instructionDraft || undefined },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to start controller');
      }
      setInstructionDraft('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to start controller'
      );
    }
  }, [client, instructionDraft, name, refreshAfterAction]);

  const onCreateTask = React.useCallback(async () => {
    const description = newTaskDescription.trim();
    if (!name || !description) return;
    setActionError('');
    try {
      const { error: apiError } = await client.POST('/controller/{name}/tasks', {
        params: { path: { name } },
        body: { description },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to create task');
      }
      setNewTaskDescription('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to create task'
      );
    }
  }, [client, name, newTaskDescription, refreshAfterAction]);

  const onToggleTask = React.useCallback(
    async (task: ControllerTask, done: boolean) => {
      if (!name) return;
      setActionError('');
      try {
        const { error: apiError } = await client.PATCH(
          '/controller/{name}/tasks/{taskId}',
          {
            params: { path: { name, taskId: task.id } },
            body: { done },
          }
        );
        if (apiError) {
          throw new Error(apiError.message || 'Failed to update task');
        }
        await refreshAfterAction();
      } catch (err) {
        setActionError(
          err instanceof Error ? err.message : 'Failed to update task'
        );
      }
    },
    [client, name, refreshAfterAction]
  );

  const onEditTask = React.useCallback(
    (task: ControllerTaskTemplate) => {
      if (!name || busyAction) return;
      setTaskEditDraft(task);
      setTaskEditDescription(task.description);
    },
    [busyAction, name]
  );

  const onCancelTaskEdit = React.useCallback(() => {
    if (busyAction) {
      return;
    }
    setTaskEditDraft(null);
    setTaskEditDescription('');
  }, [busyAction]);

  const onSaveTaskEdit = React.useCallback(async () => {
    if (!name || !taskEditDraft || busyAction) return;
    const trimmed = taskEditDescription.trim();
    if (!trimmed) return;
    if (trimmed === taskEditDraft.description.trim()) {
      setTaskEditDraft(null);
      setTaskEditDescription('');
      return;
    }
    setActionError('');
    setBusyAction('edit-task');
    try {
      const { error: apiError } = await client.PATCH(
        '/controller/{name}/tasks/{taskId}',
        {
          params: { path: { name, taskId: taskEditDraft.id } },
          body: { description: trimmed },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || 'Failed to update task');
      }
      setTaskEditDraft(null);
      setTaskEditDescription('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to update task'
      );
    } finally {
      setBusyAction(null);
    }
  }, [
    busyAction,
    client,
    name,
    refreshAfterAction,
    taskEditDescription,
    taskEditDraft,
  ]);

  const onDeleteTask = React.useCallback(
    async (task: ControllerTaskTemplate) => {
      if (!name || busyAction) return;
      setConfirmation({
        kind: 'deleteTask',
        title: 'Delete Task',
        buttonText: 'Delete Task',
        message: `Delete task "${task.description}"?`,
        task,
      });
    },
    [busyAction, name]
  );

  const onMoveTask = React.useCallback(
    async (task: ControllerTaskTemplate, direction: -1 | 1) => {
      if (!name || !detail?.taskTemplates?.length) return;
      const tasks = [...detail.taskTemplates];
      const index = tasks.findIndex((item) => item.id === task.id);
      const nextIndex = index + direction;
      if (index < 0 || nextIndex < 0 || nextIndex >= tasks.length) return;
      const [moved] = tasks.splice(index, 1);
      if (!moved) return;
      tasks.splice(nextIndex, 0, moved);
      setActionError('');
      try {
        const { error: apiError } = await client.POST(
          '/controller/{name}/tasks/reorder',
          {
            params: { path: { name } },
            body: { taskIds: tasks.map((item) => item.id) },
          }
        );
        if (apiError) {
          throw new Error(apiError.message || 'Failed to reorder tasks');
        }
        await refreshAfterAction();
      } catch (err) {
        setActionError(
          err instanceof Error ? err.message : 'Failed to reorder tasks'
        );
      }
    },
    [client, detail?.taskTemplates, name, refreshAfterAction]
  );

  const submitHumanResponse = React.useCallback(
    async (selectedOptionIds: string[], freeText: string) => {
      if (!name || !detail?.state?.pendingPrompt) return;
      setActionError('');
      try {
        const { error: apiError } = await client.POST(
          '/controller/{name}/response',
          {
            params: { path: { name } },
            body: {
              promptId: detail.state.pendingPrompt.id,
              selectedOptionIds:
                selectedOptionIds.length > 0 ? selectedOptionIds : undefined,
              freeTextResponse: freeText || undefined,
            },
          }
        );
        if (apiError) {
          throw new Error(apiError.message || 'Failed to respond');
        }
        setSelectedOptions([]);
        setFreeTextResponse('');
        await refreshAfterAction();
      } catch (err) {
        setActionError(
          err instanceof Error ? err.message : 'Failed to respond'
        );
      }
    },
    [client, detail?.state?.pendingPrompt, name, refreshAfterAction]
  );

  const onRespond = React.useCallback(async () => {
    await submitHumanResponse(selectedOptions, freeTextResponse);
  }, [freeTextResponse, selectedOptions, submitHumanResponse]);

  const onSendOperatorMessage = React.useCallback(async () => {
    if (!name || !operatorMessageDraft.trim()) return;
    setActionError('');
    try {
      const { error: apiError } = await client.POST(
        '/controller/{name}/message',
        {
          params: { path: { name } },
          body: { message: operatorMessageDraft },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || 'Failed to send operator message');
      }
      setOperatorMessageDraft('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to send operator message'
      );
    }
  }, [client, name, operatorMessageDraft, refreshAfterAction]);

  const onPauseResume = React.useCallback(async () => {
    if (!name || !detail) return;
    const paused = detail.state.state === 'paused';
    setActionError('');
    setBusyAction(paused ? 'resume' : 'pause');
    try {
      const response = paused
        ? await client.POST('/controller/{name}/resume', {
            params: { path: { name } },
          })
        : await client.POST('/controller/{name}/pause', {
            params: { path: { name } },
          });
      if (response.error) {
        throw new Error(
          response.error.message ||
            (paused ? 'Failed to resume controller' : 'Failed to pause controller')
        );
      }
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : paused
            ? 'Failed to resume controller'
            : 'Failed to pause controller'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, detail, name, refreshAfterAction]);

  const onRename = React.useCallback(async () => {
    if (!name || !detail || busyAction) return;
    const nextName = window.prompt(
      'Enter the new Controller name.',
      detail.definition.name
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed || trimmed === detail.definition.name) return;
    setActionError('');
    setBusyAction('rename');
    try {
      const { error: apiError } = await client.POST('/controller/{name}/rename', {
        params: { path: { name } },
        body: { newName: trimmed },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to rename controller');
      }
      await refreshAfterAction(() =>
        onSelectedNameChange
          ? onSelectedNameChange(trimmed)
          : navigate(
              `/cockpit?mode=controller&controller=${encodeURIComponent(trimmed)}`
            )
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to rename controller'
      );
    } finally {
      setBusyAction(null);
    }
  }, [
    busyAction,
    client,
    detail,
    name,
    navigate,
    onSelectedNameChange,
    refreshAfterAction,
  ]);

  const onClone = React.useCallback(async () => {
    if (!name || !detail || busyAction) return;
    const clonedName = buildDefaultCloneName(detail.definition.name);
    setActionError('');
    setBusyAction('clone');
    try {
      const { error: apiError } = await client.POST(
        '/controller/{name}/duplicate',
        {
          params: { path: { name } },
          body: { newName: clonedName },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || 'Failed to clone controller');
      }
      await refreshAfterAction(() =>
        onSelectedNameChange
          ? onSelectedNameChange(clonedName)
          : navigate(
              `/cockpit?mode=controller&controller=${encodeURIComponent(clonedName)}`
            )
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to clone controller'
      );
    } finally {
      setBusyAction(null);
    }
  }, [
    busyAction,
    client,
    detail,
    name,
    navigate,
    onSelectedNameChange,
    refreshAfterAction,
  ]);

  const onResetState = React.useCallback(async () => {
    if (!name || busyAction) return;
    setConfirmation({
      kind: 'reset',
      title: 'Reset Controller State',
      buttonText: 'Reset State',
      message:
        'Reset this Controller state? This clears the active task, session transcript binding, and tracked runtime state.',
    });
  }, [busyAction, name]);

  const onDelete = React.useCallback(async () => {
    if (!name || busyAction) return;
    setConfirmation({
      kind: 'delete',
      title: 'Delete Controller',
      buttonText: 'Delete Controller',
      message:
        'Delete this Controller? This removes the definition and runtime state.',
    });
  }, [busyAction, name]);

  const dismissConfirmation = React.useCallback(() => {
    if (busyAction) {
      return;
    }
    setConfirmation(null);
  }, [busyAction]);

  const confirmButtonText = React.useMemo(() => {
    if (!confirmation) {
      return '';
    }
    switch (confirmation.kind) {
      case 'deleteTask':
        return busyAction === 'delete-task' ? 'Deleting...' : 'Delete Task';
      case 'reset':
        return busyAction === 'reset' ? 'Resetting...' : 'Reset State';
      case 'delete':
        return busyAction === 'delete' ? 'Deleting...' : 'Delete Controller';
    }
  }, [busyAction, confirmation]);

  const onConfirmAction = React.useCallback(async () => {
    if (!name || !confirmation || busyAction) {
      return;
    }

    setActionError('');

    switch (confirmation.kind) {
      case 'deleteTask':
        setBusyAction('delete-task');
        try {
          const { error: apiError } = await client.DELETE(
            '/controller/{name}/tasks/{taskId}',
            {
              params: { path: { name, taskId: confirmation.task.id } },
            }
          );
          if (apiError) {
            throw new Error(apiError.message || 'Failed to delete task');
          }
          await refreshAfterAction();
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error ? err.message : 'Failed to delete task'
          );
        } finally {
          setBusyAction(null);
        }
        return;
      case 'reset':
        setBusyAction('reset');
        try {
          const { error: apiError } = await client.POST(
            '/controller/{name}/reset',
            {
              params: { path: { name } },
            }
          );
          if (apiError) {
            throw new Error(
              apiError.message || 'Failed to reset controller state'
            );
          }
          await refreshAfterAction();
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error
              ? err.message
              : 'Failed to reset controller state'
          );
        } finally {
          setBusyAction(null);
        }
        return;
      case 'delete':
        setBusyAction('delete');
        try {
          const { error: apiError } = await client.DELETE('/controller/{name}', {
            params: { path: { name } },
          });
          if (apiError) {
            throw new Error(apiError.message || 'Failed to delete controller');
          }
          await refreshAfterAction(() =>
            onDeleted ? onDeleted() : navigate('/cockpit?mode=controller')
          );
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error ? err.message : 'Failed to delete controller'
          );
        } finally {
          setBusyAction(null);
        }
        return;
    }
  }, [
    busyAction,
    client,
    confirmation,
    name,
    navigate,
    onDeleted,
    refreshAfterAction,
  ]);

  const onSaveSpec = React.useCallback(async () => {
    if (!name) return;
    setSpecError('');
    setIsSavingSpec(true);
    try {
      const { error: apiError } = await client.PUT('/controller/{name}/spec', {
        params: { path: { name } },
        body: { spec: specDraft },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to save spec');
      }
      await refreshAfterAction();
      setIsEditingSpec(false);
    } catch (err) {
      setSpecError(err instanceof Error ? err.message : 'Failed to save spec');
    } finally {
      setIsSavingSpec(false);
    }
  }, [client, name, refreshAfterAction, specDraft]);

  const onSaveMetadata = React.useCallback(async () => {
    if (!name || !detail || metadataSaveDisabled) return;
    const currentSpec = specQuery.data?.spec;
    if (!currentSpec) {
      setActionError('Controller spec is not loaded yet.');
      return;
    }

    setActionError('');
    setBusyAction('metadata');
    try {
      const nextSpec = updateControllerMetadataInSpec(currentSpec, {
        description: descriptionDraft,
        iconUrl: iconUrlDraft,
        goal: goalDraft,
        model: modelDraft,
        triggerPrompt: triggerPromptDraft,
        resetOnFinish: resetOnFinishDraft,
        triggerType: triggerTypeDraft,
        cronSchedules: effectiveCronSchedules,
        workflowNames,
      });
      const { error: apiError } = await client.PUT('/controller/{name}/spec', {
        params: { path: { name } },
        body: { spec: nextSpec },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to save metadata');
      }
      await refreshAfterAction();
      setIsEditingMetadata(false);
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to save metadata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [
    client,
    descriptionDraft,
    detail,
    goalDraft,
    iconUrlDraft,
    metadataSaveDisabled,
    modelDraft,
    name,
    refreshAfterAction,
    resetOnFinishDraft,
    workflowNames,
    effectiveCronSchedules,
    specQuery.data?.spec,
    triggerPromptDraft,
    triggerTypeDraft,
  ]);

  const onWorkflowNamesChange = React.useCallback(
    async (names: string[]) => {
      const nextWorkflowNames = normalizeDAGNameList(names);
      setWorkflowNamesDraft(nextWorkflowNames);

      if (
        !name ||
        !detail ||
        !specQuery.data?.spec ||
        !!busyAction ||
        isSavingSpec ||
        isEditingSpec ||
        sameDAGNameList(nextWorkflowNames, detail.definition.workflows?.names)
      ) {
        return;
      }

      setActionError('');
      setBusyAction('workflow-names');
      try {
        const nextSpec = updateControllerMetadataInSpec(specQuery.data.spec, {
          description: detail.definition.description || '',
          iconUrl: detail.definition.iconUrl || '',
          goal: detail.definition.goal || '',
          model: detail.definition.agent?.model || '',
          triggerPrompt: persistedTriggerPrompt,
          resetOnFinish: !!detail.definition.resetOnFinish,
          triggerType,
          cronSchedules: persistedCronSchedules,
          workflowNames: nextWorkflowNames,
        });
        const { error: apiError } = await client.PUT('/controller/{name}/spec', {
          params: { path: { name } },
          body: { spec: nextSpec },
        });
        if (apiError) {
          throw new Error(apiError.message || 'Failed to save workflows');
        }
        await refreshAfterAction();
      } catch (err) {
        setWorkflowNamesDraft(
          normalizeDAGNameList(detail.definition.workflows?.names)
        );
        setActionError(
          err instanceof Error ? err.message : 'Failed to save workflows'
        );
      } finally {
        setBusyAction(null);
      }
    },
    [
      busyAction,
      client,
      detail,
      isEditingSpec,
      isSavingSpec,
      name,
      persistedCronSchedules,
      persistedTriggerPrompt,
      refreshAfterAction,
      specQuery.data?.spec,
      triggerType,
    ]
  );

  const onCancelMetadata = React.useCallback(() => {
    setDescriptionDraft(detail?.definition?.description || '');
    setIconUrlDraft(detail?.definition?.iconUrl || '');
    setGoalDraft(detail?.definition?.goal || '');
    setTriggerPromptDraft(persistedTriggerPrompt);
    setResetOnFinishDraft(!!detail?.definition?.resetOnFinish);
    setTriggerTypeDraft(triggerType);
    setCronScheduleDraft(
      formatControllerCronScheduleText(persistedCronSchedules)
    );
    setWorkflowNamesDraft(
      normalizeDAGNameList(detail?.definition?.workflows?.names)
    );
    setModelDraft(detail?.definition?.agent?.model || '');
    setIsEditingMetadata(false);
  }, [
    detail?.definition?.agent?.model,
    detail?.definition?.workflows?.names,
    detail?.definition?.description,
    detail?.definition?.goal,
    detail?.definition?.iconUrl,
    detail?.definition?.resetOnFinish,
    persistedTriggerPrompt,
    persistedCronSchedules,
    triggerType,
  ]);

  return {
    name,
    detail,
    detailQuery,
    specQuery,
    availableModels,
    isLoading: detailQuery.isLoading,
    loadError: detailQuery.error,
    instructionDraft,
    setInstructionDraft,
    operatorMessageDraft,
    setOperatorMessageDraft,
    newTaskDescription,
    setNewTaskDescription,
    taskEditDraft,
    taskEditDescription,
    setTaskEditDescription,
    iconUrlDraft,
    setIconUrlDraft,
    descriptionDraft,
    setDescriptionDraft,
    goalDraft,
    setGoalDraft,
    triggerPromptDraft,
    setTriggerPromptDraft,
    resetOnFinishDraft,
    setResetOnFinishDraft,
    triggerType,
    triggerTypeDraft,
    setTriggerTypeDraft,
    cronScheduleDraft,
    setCronScheduleDraft,
    workflowNamesDraft,
    setWorkflowNamesDraft,
    workflowSearchQuery,
    setWorkflowSearchQuery,
    availableDAGOptions,
    dagListQuery,
    modelDraft,
    setModelDraft,
    isEditingMetadata,
    setIsEditingMetadata,
    freeTextResponse,
    setFreeTextResponse,
    selectedOptions,
    setSelectedOptions,
    actionError,
    setActionError,
    busyAction,
    confirmation,
    confirmButtonText,
    isEditingSpec,
    setIsEditingSpec,
    specDraft,
    setSpecDraft,
    specError,
    setSpecError,
    isSavingSpec,
    mergedRuns,
    queuedTurnMessages,
    threadItems,
    lifecycleState,
    controllerStatus,
    runtimeControllerReady,
    displayStatus,
    taskSummary,
    canStartTask,
    canSendOperatorMessage,
    canPause,
    canResume,
    cronSchedulesConfigured,
    metadataChanged,
    metadataValidationError,
    metadataSaveDisabled,
    onWorkflowNamesChange,
    onStart,
    onCreateTask,
    onToggleTask,
    onEditTask,
    onCancelTaskEdit,
    onSaveTaskEdit,
    onDeleteTask,
    onMoveTask,
    onRespond,
    onSendOperatorMessage,
    onPauseResume,
    onRename,
    onClone,
    onResetState,
    onDelete,
    dismissConfirmation,
    onConfirmAction,
    onSaveSpec,
    onSaveMetadata,
    onCancelMetadata,
  };
}
