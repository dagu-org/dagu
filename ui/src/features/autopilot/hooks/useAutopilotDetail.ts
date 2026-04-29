// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { useNavigate } from 'react-router-dom';

import { useAvailableModels } from '@/features/agent/hooks/useAvailableModels';
import { useClient, useQuery } from '@/hooks/api';
import { whenEnabled } from '@/hooks/queryUtils';
import { updateAutopilotMetadataInSpec } from '@/features/autopilot/spec';
import { workspaceTagForAutopilotSelection } from '@/features/autopilot/workspace';
import {
  buildAutopilotThread,
  type AutopilotDetail,
  type AutopilotPendingTurnMessage,
  type AutopilotRunSummary,
  type AutopilotTask,
  type AutopilotTaskTemplate,
  formatAutopilotScheduleText,
  isValidAutopilotIconUrl,
  parseAutopilotScheduleText,
  taskCounts,
  validateAutopilotScheduleExpressions,
} from '@/features/autopilot/detail-utils';

type MutationCallback = (() => void | Promise<void>) | undefined;
const CLONE_NAME_SUFFIX_LENGTH = 6;
const CLONE_NAME_SUFFIX_ALPHABET = 'abcdefghijklmnopqrstuvwxyz0123456789';
const DAG_PICKER_SEARCH_LIMIT = 25;

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

type AutopilotConfirmationState =
  | {
      kind: 'deleteTask';
      title: string;
      buttonText: string;
      message: string;
      task: AutopilotTaskTemplate;
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

export type AutopilotDetailController = ReturnType<
  typeof useAutopilotDetailController
>;

export function useAutopilotDetailController({
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
    '/autopilot/{name}',
    whenEnabled(enabled && !!name, {
      params: { path: { name: name || '' } },
    }),
    {
      refreshInterval: (data?: AutopilotDetail) =>
        data?.state?.state === 'running' ||
        data?.state?.state === 'waiting' ||
        data?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useQuery(
    '/autopilot/{name}/spec',
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
  const [standingInstructionDraft, setStandingInstructionDraft] =
    React.useState('');
  const [resetOnFinishDraft, setResetOnFinishDraft] = React.useState(false);
  const [scheduleDraft, setScheduleDraft] = React.useState('');
  const [allowedDAGNamesDraft, setAllowedDAGNamesDraft] = React.useState<
    string[]
  >([]);
  const [allowedDAGSearchQuery, setAllowedDAGSearchQuery] = React.useState('');
  const [taskEditDraft, setTaskEditDraft] =
    React.useState<AutopilotTaskTemplate | null>(null);
  const [taskEditDescription, setTaskEditDescription] = React.useState('');
  const [modelDraft, setModelDraft] = React.useState('');
  const [isEditingMetadata, setIsEditingMetadata] = React.useState(false);
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [actionError, setActionError] = React.useState('');
  const [busyAction, setBusyAction] = React.useState<string | null>(null);
  const [confirmation, setConfirmation] =
    React.useState<AutopilotConfirmationState | null>(null);

  const [isEditingSpec, setIsEditingSpec] = React.useState(false);
  const [specDraft, setSpecDraft] = React.useState('');
  const [specError, setSpecError] = React.useState('');
  const [isSavingSpec, setIsSavingSpec] = React.useState(false);
  const selectedWorkspaceTag =
    workspaceTagForAutopilotSelection(selectedWorkspace);
  const allowedDAGSearchName = allowedDAGSearchQuery.trim();
  const dagListQuery = useQuery(
    '/dags',
    whenEnabled(enabled && !!allowedDAGSearchName, {
      params: {
        query: {
          perPage: DAG_PICKER_SEARCH_LIMIT,
          remoteNode: remoteNode || undefined,
          labels: selectedWorkspaceTag,
          name: allowedDAGSearchName,
        },
      },
    }),
    { refreshInterval: 15000 }
  );

  React.useEffect(() => {
    setInstructionDraft('');
    setOperatorMessageDraft('');
    setNewTaskDescription('');
    setAllowedDAGSearchQuery('');
    setTaskEditDraft(null);
    setTaskEditDescription('');
    setActionError('');
  }, [name]);

  React.useEffect(() => {
    if (!isEditingMetadata) {
      setDescriptionDraft(detail?.definition?.description || '');
      setIconUrlDraft(detail?.definition?.iconUrl || '');
      setGoalDraft(detail?.definition?.goal || '');
      setStandingInstructionDraft(
        detail?.definition?.standingInstruction || ''
      );
      setResetOnFinishDraft(!!detail?.definition?.resetOnFinish);
      setScheduleDraft(
        formatAutopilotScheduleText(detail?.definition?.schedule)
      );
      setAllowedDAGNamesDraft(
        normalizeDAGNameList(detail?.definition?.allowedDAGs?.names)
      );
      setModelDraft(detail?.definition?.agent?.model || '');
    }
  }, [
    detail?.definition?.allowedDAGs?.names,
    detail?.definition?.description,
    detail?.definition?.iconUrl,
    detail?.definition?.goal,
    detail?.definition?.resetOnFinish,
    detail?.definition?.schedule,
    detail?.definition?.standingInstruction,
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
      return [] as Array<AutopilotRunSummary & { isCurrent?: boolean }>;
    }
    const seen = new Set<string>();
    const items: Array<AutopilotRunSummary & { isCurrent?: boolean }> = [];
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

  const queuedTurnMessages = React.useMemo<AutopilotPendingTurnMessage[]>(
    () => detail?.state?.pendingTurnMessages || [],
    [detail?.state?.pendingTurnMessages]
  );

  const threadItems = React.useMemo(
    () => buildAutopilotThread(detail?.messages, queuedTurnMessages),
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
  const autopilotController = detail?.autopilotController;
  const runtimeControllerReady = autopilotController?.state === 'ready';
  const displayStatus =
    detail?.state?.displayStatus ?? detail?.state?.state ?? '';
  const taskSummary = React.useMemo(
    () => taskCounts(detail?.state?.tasks),
    [detail?.state?.tasks]
  );
  const scheduleExpressions = React.useMemo(
    () => parseAutopilotScheduleText(scheduleDraft),
    [scheduleDraft]
  );
  const canStartTask =
    runtimeControllerReady &&
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
  const scheduleConfigured = (detail?.definition?.schedule?.length || 0) > 0;

  const descriptionChanged =
    descriptionDraft.trim() !== (detail?.definition?.description || '').trim();
  const iconUrlChanged =
    iconUrlDraft.trim() !== (detail?.definition?.iconUrl || '').trim();
  const goalChanged =
    goalDraft.trim() !== (detail?.definition?.goal || '').trim();
  const standingInstructionChanged =
    standingInstructionDraft.trim() !==
    (detail?.definition?.standingInstruction || '').trim();
  const resetOnFinishChanged =
    resetOnFinishDraft !== !!detail?.definition?.resetOnFinish;
  const scheduleChanged =
    formatAutopilotScheduleText(scheduleExpressions) !==
    formatAutopilotScheduleText(detail?.definition?.schedule);
  const allowedDAGNamesChanged = !sameDAGNameList(
    allowedDAGNamesDraft,
    detail?.definition?.allowedDAGs?.names
  );
  const modelChanged =
    modelDraft.trim() !== (detail?.definition?.agent?.model || '').trim();
  const metadataChanged =
    descriptionChanged ||
    iconUrlChanged ||
    goalChanged ||
    standingInstructionChanged ||
    resetOnFinishChanged ||
    scheduleChanged ||
    allowedDAGNamesChanged ||
    modelChanged;
  const allowedDAGNames = React.useMemo(
    () => normalizeDAGNameList(allowedDAGNamesDraft),
    [allowedDAGNamesDraft]
  );
  const allowedDAGTagsConfigured =
    (detail?.definition?.allowedDAGs?.tags?.length || 0) > 0;
  const metadataValidationError = !isValidAutopilotIconUrl(iconUrlDraft)
    ? 'Icon URL must be an absolute http(s) URL or a root-relative path.'
    : iconUrlDraft.trim().length > 2048
      ? 'Icon URL must be 2048 characters or fewer.'
      : validateAutopilotScheduleExpressions(scheduleExpressions) ||
        (allowedDAGNames.length === 0 && !allowedDAGTagsConfigured
          ? 'Select at least one allowed DAG, or configure allowed_dags.tags in raw spec.'
          : null);
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
      const { error: apiError } = await client.POST('/autopilot/{name}/start', {
        params: { path: { name } },
        body: { instruction: instructionDraft || undefined },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to start autopilot');
      }
      setInstructionDraft('');
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to start autopilot'
      );
    }
  }, [client, instructionDraft, name, refreshAfterAction]);

  const onCreateTask = React.useCallback(async () => {
    const description = newTaskDescription.trim();
    if (!name || !description) return;
    setActionError('');
    try {
      const { error: apiError } = await client.POST('/autopilot/{name}/tasks', {
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
    async (task: AutopilotTask, done: boolean) => {
      if (!name) return;
      setActionError('');
      try {
        const { error: apiError } = await client.PATCH(
          '/autopilot/{name}/tasks/{taskId}',
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
    (task: AutopilotTaskTemplate) => {
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
        '/autopilot/{name}/tasks/{taskId}',
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
    async (task: AutopilotTaskTemplate) => {
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
    async (task: AutopilotTaskTemplate, direction: -1 | 1) => {
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
          '/autopilot/{name}/tasks/reorder',
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
          '/autopilot/{name}/response',
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
        '/autopilot/{name}/message',
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
        ? await client.POST('/autopilot/{name}/resume', {
            params: { path: { name } },
          })
        : await client.POST('/autopilot/{name}/pause', {
            params: { path: { name } },
          });
      if (response.error) {
        throw new Error(
          response.error.message ||
            (paused ? 'Failed to resume autopilot' : 'Failed to pause autopilot')
        );
      }
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : paused
            ? 'Failed to resume autopilot'
            : 'Failed to pause autopilot'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, detail, name, refreshAfterAction]);

  const onRename = React.useCallback(async () => {
    if (!name || !detail || busyAction) return;
    const nextName = window.prompt(
      'Enter the new Autopilot name.',
      detail.definition.name
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed || trimmed === detail.definition.name) return;
    setActionError('');
    setBusyAction('rename');
    try {
      const { error: apiError } = await client.POST('/autopilot/{name}/rename', {
        params: { path: { name } },
        body: { newName: trimmed },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to rename autopilot');
      }
      await refreshAfterAction(() =>
        onSelectedNameChange
          ? onSelectedNameChange(trimmed)
          : navigate(
              `/cockpit?mode=autopilot&autopilot=${encodeURIComponent(trimmed)}`
            )
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to rename autopilot'
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
        '/autopilot/{name}/duplicate',
        {
          params: { path: { name } },
          body: { newName: clonedName },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || 'Failed to clone autopilot');
      }
      await refreshAfterAction(() =>
        onSelectedNameChange
          ? onSelectedNameChange(clonedName)
          : navigate(
              `/cockpit?mode=autopilot&autopilot=${encodeURIComponent(clonedName)}`
            )
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to clone autopilot'
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
      title: 'Reset Autopilot State',
      buttonText: 'Reset State',
      message:
        'Reset this Autopilot state? This clears the active task, session transcript binding, and tracked runtime state.',
    });
  }, [busyAction, name]);

  const onDelete = React.useCallback(async () => {
    if (!name || busyAction) return;
    setConfirmation({
      kind: 'delete',
      title: 'Delete Autopilot',
      buttonText: 'Delete Autopilot',
      message:
        'Delete this Autopilot? This removes the definition and runtime state.',
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
        return busyAction === 'delete' ? 'Deleting...' : 'Delete Autopilot';
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
            '/autopilot/{name}/tasks/{taskId}',
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
            '/autopilot/{name}/reset',
            {
              params: { path: { name } },
            }
          );
          if (apiError) {
            throw new Error(
              apiError.message || 'Failed to reset autopilot state'
            );
          }
          await refreshAfterAction();
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error
              ? err.message
              : 'Failed to reset autopilot state'
          );
        } finally {
          setBusyAction(null);
        }
        return;
      case 'delete':
        setBusyAction('delete');
        try {
          const { error: apiError } = await client.DELETE('/autopilot/{name}', {
            params: { path: { name } },
          });
          if (apiError) {
            throw new Error(apiError.message || 'Failed to delete autopilot');
          }
          await refreshAfterAction(() =>
            onDeleted ? onDeleted() : navigate('/cockpit?mode=autopilot')
          );
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error ? err.message : 'Failed to delete autopilot'
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
      const { error: apiError } = await client.PUT('/autopilot/{name}/spec', {
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
      setActionError('Autopilot spec is not loaded yet.');
      return;
    }

    setActionError('');
    setBusyAction('metadata');
    try {
      const nextSpec = updateAutopilotMetadataInSpec(currentSpec, {
        description: descriptionDraft,
        iconUrl: iconUrlDraft,
        goal: goalDraft,
        model: modelDraft,
        standingInstruction: standingInstructionDraft,
        resetOnFinish: resetOnFinishDraft,
        schedule: scheduleExpressions,
        allowedDAGNames,
      });
      const { error: apiError } = await client.PUT('/autopilot/{name}/spec', {
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
    allowedDAGNames,
    scheduleExpressions,
    specQuery.data?.spec,
    standingInstructionDraft,
  ]);

  const onCancelMetadata = React.useCallback(() => {
    setDescriptionDraft(detail?.definition?.description || '');
    setIconUrlDraft(detail?.definition?.iconUrl || '');
    setGoalDraft(detail?.definition?.goal || '');
    setStandingInstructionDraft(detail?.definition?.standingInstruction || '');
    setResetOnFinishDraft(!!detail?.definition?.resetOnFinish);
    setScheduleDraft(formatAutopilotScheduleText(detail?.definition?.schedule));
    setAllowedDAGNamesDraft(
      normalizeDAGNameList(detail?.definition?.allowedDAGs?.names)
    );
    setModelDraft(detail?.definition?.agent?.model || '');
    setIsEditingMetadata(false);
  }, [
    detail?.definition?.agent?.model,
    detail?.definition?.allowedDAGs?.names,
    detail?.definition?.description,
    detail?.definition?.goal,
    detail?.definition?.iconUrl,
    detail?.definition?.resetOnFinish,
    detail?.definition?.schedule,
    detail?.definition?.standingInstruction,
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
    standingInstructionDraft,
    setStandingInstructionDraft,
    resetOnFinishDraft,
    setResetOnFinishDraft,
    scheduleDraft,
    setScheduleDraft,
    allowedDAGNamesDraft,
    setAllowedDAGNamesDraft,
    allowedDAGSearchQuery,
    setAllowedDAGSearchQuery,
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
    autopilotController,
    runtimeControllerReady,
    displayStatus,
    taskSummary,
    canStartTask,
    canSendOperatorMessage,
    canPause,
    canResume,
    scheduleConfigured,
    metadataChanged,
    metadataValidationError,
    metadataSaveDisabled,
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
