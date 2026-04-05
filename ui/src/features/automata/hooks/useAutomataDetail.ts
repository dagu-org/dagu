// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';
import { useNavigate } from 'react-router-dom';

import { useAvailableModels } from '@/features/agent/hooks/useAvailableModels';
import { useClient, useQuery } from '@/hooks/api';
import { whenEnabled } from '@/hooks/queryUtils';
import { updateAutomataMetadataInSpec } from '@/features/automata/spec';
import {
  buildAutomataThread,
  type AutomataDetail,
  type AutomataPendingTurnMessage,
  type AutomataRunSummary,
  type AutomataTask,
  type AutomataTaskTemplate,
  formatAutomataScheduleText,
  isServiceKind,
  isValidAutomataIconUrl,
  parseAutomataScheduleText,
  taskCounts,
  validateAutomataScheduleExpressions,
} from '@/features/automata/detail-utils';

type MutationCallback = (() => void | Promise<void>) | undefined;
type AutomataConfirmationState =
  | {
      kind: 'deleteTask';
      title: string;
      buttonText: string;
      message: string;
      task: AutomataTaskTemplate;
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

export type AutomataDetailController = ReturnType<
  typeof useAutomataDetailController
>;

export function useAutomataDetailController({
  name,
  enabled = true,
  onUpdated,
}: {
  name?: string;
  enabled?: boolean;
  onUpdated?: () => void | Promise<void>;
}) {
  const client = useClient();
  const navigate = useNavigate();
  const { models: availableModels } = useAvailableModels();

  const detailQuery = useQuery(
    '/automata/{name}',
    whenEnabled(enabled && !!name, {
      params: { path: { name: name || '' } },
    }),
    {
      refreshInterval: (data?: AutomataDetail) =>
        data?.state?.state === 'running' ||
        data?.state?.state === 'waiting' ||
        data?.state?.state === 'paused'
          ? 2000
          : 15000,
    }
  );

  const specQuery = useQuery(
    '/automata/{name}/spec',
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
  const [scheduleDraft, setScheduleDraft] = React.useState('');
  const [modelDraft, setModelDraft] = React.useState('');
  const [isEditingMetadata, setIsEditingMetadata] = React.useState(false);
  const [freeTextResponse, setFreeTextResponse] = React.useState('');
  const [selectedOptions, setSelectedOptions] = React.useState<string[]>([]);
  const [actionError, setActionError] = React.useState('');
  const [busyAction, setBusyAction] = React.useState<string | null>(null);
  const [confirmation, setConfirmation] =
    React.useState<AutomataConfirmationState | null>(null);

  const [isEditingSpec, setIsEditingSpec] = React.useState(false);
  const [specDraft, setSpecDraft] = React.useState('');
  const [specError, setSpecError] = React.useState('');
  const [isSavingSpec, setIsSavingSpec] = React.useState(false);

  React.useEffect(() => {
    setInstructionDraft(detail?.state?.instruction || '');
  }, [detail?.state?.instruction, name]);

  React.useEffect(() => {
    setOperatorMessageDraft('');
    setNewTaskDescription('');
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
      setScheduleDraft(
        formatAutomataScheduleText(detail?.definition?.schedule)
      );
      setModelDraft(detail?.definition?.agent?.model || '');
    }
  }, [
    detail?.definition?.description,
    detail?.definition?.iconUrl,
    detail?.definition?.goal,
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
      return [] as Array<AutomataRunSummary & { isCurrent?: boolean }>;
    }
    const seen = new Set<string>();
    const items: Array<AutomataRunSummary & { isCurrent?: boolean }> = [];
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

  const queuedTurnMessages = React.useMemo<AutomataPendingTurnMessage[]>(
    () => detail?.state?.pendingTurnMessages || [],
    [detail?.state?.pendingTurnMessages]
  );

  const threadItems = React.useMemo(
    () => buildAutomataThread(detail?.messages, queuedTurnMessages),
    [detail?.messages, queuedTurnMessages]
  );

  const lifecycleState = detail?.state?.state ?? '';
  const automataKind = detail?.definition?.kind;
  const serviceKind = isServiceKind(automataKind);
  const serviceActivated = serviceKind && !!detail?.state?.activatedAt;
  const automataController = detail?.automataController;
  const runtimeControllerReady = automataController?.state === 'ready';
  const displayStatus =
    detail?.state?.displayStatus ?? detail?.state?.state ?? '';
  const taskSummary = React.useMemo(
    () => taskCounts(detail?.state?.tasks),
    [detail?.state?.tasks]
  );
  const scheduleExpressions = React.useMemo(
    () => parseAutomataScheduleText(scheduleDraft),
    [scheduleDraft]
  );
  const canStartTask = serviceKind
    ? runtimeControllerReady &&
      lifecycleState === 'idle' &&
      !serviceActivated
    : runtimeControllerReady &&
      (lifecycleState === 'idle' || lifecycleState === 'finished');
  const canSendOperatorMessage =
    !!detail &&
    runtimeControllerReady &&
    !detail.state.pendingPrompt &&
    (serviceKind
      ? serviceActivated && lifecycleState !== 'paused'
      : lifecycleState === 'running' ||
        lifecycleState === 'waiting' ||
        lifecycleState === 'paused');
  const canPause = serviceKind
    ? runtimeControllerReady && serviceActivated && lifecycleState !== 'paused'
    : runtimeControllerReady &&
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
  const scheduleChanged =
    formatAutomataScheduleText(scheduleExpressions) !==
    formatAutomataScheduleText(detail?.definition?.schedule);
  const modelChanged =
    modelDraft.trim() !== (detail?.definition?.agent?.model || '').trim();
  const metadataChanged =
    descriptionChanged ||
    iconUrlChanged ||
    goalChanged ||
    standingInstructionChanged ||
    scheduleChanged ||
    modelChanged;
  const metadataValidationError =
    !isValidAutomataIconUrl(iconUrlDraft)
      ? 'Icon URL must be an absolute http(s) URL or a root-relative path.'
      : iconUrlDraft.trim().length > 2048
        ? 'Icon URL must be 2048 characters or fewer.'
        : serviceKind
          ? validateAutomataScheduleExpressions(scheduleExpressions)
          : null;
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
      const { error: apiError } = await client.POST('/automata/{name}/start', {
        params: { path: { name } },
        body: serviceKind
          ? {}
          : { instruction: instructionDraft || undefined },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to start automata');
      }
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to start automata'
      );
    }
  }, [client, instructionDraft, name, refreshAfterAction, serviceKind]);

  const onCreateTask = React.useCallback(async () => {
    if (!name || !newTaskDescription.trim()) return;
    setActionError('');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/tasks', {
        params: { path: { name } },
        body: { description: newTaskDescription },
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
    async (task: AutomataTask, done: boolean) => {
      if (!name) return;
      setActionError('');
      try {
        const { error: apiError } = await client.PATCH(
          '/automata/{name}/tasks/{taskId}',
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
    async (task: AutomataTaskTemplate) => {
      if (!name) return;
      const nextDescription = window.prompt(
        'Update task description.',
        task.description
      );
      if (nextDescription == null) return;
      const trimmed = nextDescription.trim();
      if (!trimmed || trimmed === task.description) return;
      setActionError('');
      try {
        const { error: apiError } = await client.PATCH(
          '/automata/{name}/tasks/{taskId}',
          {
            params: { path: { name, taskId: task.id } },
            body: { description: trimmed },
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

  const onDeleteTask = React.useCallback(
    async (task: AutomataTaskTemplate) => {
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
    async (task: AutomataTaskTemplate, direction: -1 | 1) => {
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
          '/automata/{name}/tasks/reorder',
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
          '/automata/{name}/response',
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
        '/automata/{name}/message',
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
      await refreshAfterAction();
    } catch (err) {
      setActionError(
        err instanceof Error
          ? err.message
          : paused
            ? 'Failed to resume automata'
            : 'Failed to pause automata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [client, detail, name, refreshAfterAction]);

  const onRename = React.useCallback(async () => {
    if (!name || !detail || busyAction) return;
    const nextName = window.prompt(
      'Enter the new Automata name.',
      detail.definition.name
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed || trimmed === detail.definition.name) return;
    setActionError('');
    setBusyAction('rename');
    try {
      const { error: apiError } = await client.POST('/automata/{name}/rename', {
        params: { path: { name } },
        body: { newName: trimmed },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to rename automata');
      }
      await refreshAfterAction(() =>
        navigate(`/automata/${encodeURIComponent(trimmed)}`)
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to rename automata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [busyAction, client, detail, name, navigate, refreshAfterAction]);

  const onDuplicate = React.useCallback(async () => {
    if (!name || !detail || busyAction) return;
    const nextName = window.prompt(
      'Enter the new name for the duplicate Automata.',
      `${detail.definition.name}-copy`
    );
    if (nextName == null) return;
    const trimmed = nextName.trim();
    if (!trimmed) return;
    setActionError('');
    setBusyAction('duplicate');
    try {
      const { error: apiError } = await client.POST(
        '/automata/{name}/duplicate',
        {
          params: { path: { name } },
          body: { newName: trimmed },
        }
      );
      if (apiError) {
        throw new Error(apiError.message || 'Failed to duplicate automata');
      }
      await refreshAfterAction(() =>
        navigate(`/automata/${encodeURIComponent(trimmed)}`)
      );
    } catch (err) {
      setActionError(
        err instanceof Error ? err.message : 'Failed to duplicate automata'
      );
    } finally {
      setBusyAction(null);
    }
  }, [busyAction, client, detail, name, navigate, refreshAfterAction]);

  const onResetState = React.useCallback(async () => {
    if (!name || busyAction) return;
    setConfirmation({
      kind: 'reset',
      title: 'Reset Automata State',
      buttonText: 'Reset State',
      message:
        'Reset this Automata state? This clears the active task, session transcript binding, and tracked runtime state.',
    });
  }, [busyAction, name]);

  const onDelete = React.useCallback(async () => {
    if (!name || busyAction) return;
    setConfirmation({
      kind: 'delete',
      title: 'Delete Automata',
      buttonText: 'Delete Automata',
      message:
        'Delete this Automata? This removes the definition and runtime state.',
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
        return busyAction === 'delete' ? 'Deleting...' : 'Delete Automata';
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
            '/automata/{name}/tasks/{taskId}',
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
          const { error: apiError } = await client.POST('/automata/{name}/reset', {
            params: { path: { name } },
          });
          if (apiError) {
            throw new Error(apiError.message || 'Failed to reset automata state');
          }
          await refreshAfterAction();
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error ? err.message : 'Failed to reset automata state'
          );
        } finally {
          setBusyAction(null);
        }
        return;
      case 'delete':
        setBusyAction('delete');
        try {
          const { error: apiError } = await client.DELETE('/automata/{name}', {
            params: { path: { name } },
          });
          if (apiError) {
            throw new Error(apiError.message || 'Failed to delete automata');
          }
          await refreshAfterAction(() => navigate('/automata'));
          setConfirmation(null);
        } catch (err) {
          setActionError(
            err instanceof Error ? err.message : 'Failed to delete automata'
          );
        } finally {
          setBusyAction(null);
        }
        return;
    }
  }, [busyAction, client, confirmation, name, navigate, refreshAfterAction]);

  const onSaveSpec = React.useCallback(async () => {
    if (!name) return;
    setSpecError('');
    setIsSavingSpec(true);
    try {
      const { error: apiError } = await client.PUT('/automata/{name}/spec', {
        params: { path: { name } },
        body: { spec: specDraft },
      });
      if (apiError) {
        throw new Error(apiError.message || 'Failed to save spec');
      }
      await refreshAfterAction();
      setIsEditingSpec(false);
    } catch (err) {
      setSpecError(
        err instanceof Error ? err.message : 'Failed to save spec'
      );
    } finally {
      setIsSavingSpec(false);
    }
  }, [client, name, refreshAfterAction, specDraft]);

  const onSaveMetadata = React.useCallback(async () => {
    if (!name || !detail || metadataSaveDisabled) return;
    const currentSpec = specQuery.data?.spec;
    if (!currentSpec) {
      setActionError('Automata spec is not loaded yet.');
      return;
    }

    setActionError('');
    setBusyAction('metadata');
    try {
      const nextSpec = updateAutomataMetadataInSpec(currentSpec, {
        description: descriptionDraft,
        iconUrl: iconUrlDraft,
        goal: goalDraft,
        model: modelDraft,
        standingInstruction: standingInstructionDraft,
        schedule: scheduleExpressions,
      });
      const { error: apiError } = await client.PUT('/automata/{name}/spec', {
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
    scheduleExpressions,
    specQuery.data?.spec,
    standingInstructionDraft,
  ]);

  const onCancelMetadata = React.useCallback(() => {
    setDescriptionDraft(detail?.definition?.description || '');
    setIconUrlDraft(detail?.definition?.iconUrl || '');
    setGoalDraft(detail?.definition?.goal || '');
    setStandingInstructionDraft(detail?.definition?.standingInstruction || '');
    setScheduleDraft(formatAutomataScheduleText(detail?.definition?.schedule));
    setModelDraft(detail?.definition?.agent?.model || '');
    setIsEditingMetadata(false);
  }, [
    detail?.definition?.agent?.model,
    detail?.definition?.description,
    detail?.definition?.goal,
    detail?.definition?.iconUrl,
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
    iconUrlDraft,
    setIconUrlDraft,
    descriptionDraft,
    setDescriptionDraft,
    goalDraft,
    setGoalDraft,
    standingInstructionDraft,
    setStandingInstructionDraft,
    scheduleDraft,
    setScheduleDraft,
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
    automataKind,
    serviceKind,
    serviceActivated,
    automataController,
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
    onDeleteTask,
    onMoveTask,
    onRespond,
    onSendOperatorMessage,
    onPauseResume,
    onRename,
    onDuplicate,
    onResetState,
    onDelete,
    dismissConfirmation,
    onConfirmAction,
    onSaveSpec,
    onSaveMetadata,
    onCancelMetadata,
  };
}
