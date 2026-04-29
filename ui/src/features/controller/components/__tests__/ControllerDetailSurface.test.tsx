// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(() => ({})),
}));

vi.mock('@/features/dags/components/dag-details/DAGDetailsModal', () => ({
  default: () => null,
}));

vi.mock(
  '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal',
  () => ({
    default: () => null,
  })
);

import { ControllerDetailSurface } from '@/features/controller/components/ControllerDetailSurface';
import type { ControllerDetailController } from '@/features/controller/hooks/useControllerDetail';

afterEach(() => {
  cleanup();
});

function createController(
  overrides: Partial<ControllerDetailController> = {}
): ControllerDetailController {
  return {
    name: 'researcher',
    detail: {
      artifactDir: '/tmp/controller-artifacts/researcher',
      artifactsAvailable: false,
      definition: {
        name: 'researcher',
        nickname: 'Researcher',
        description: 'Researches and reports.',
        iconUrl: '',
        disabled: false,
        labels: [],
        goal: 'Collect interesting AI news',
        resetOnFinish: false,
        trigger: {
          type: 'manual',
        },
        workflows: {
          names: ['collect-news'],
        },
        agent: {
          model: '',
        },
      },
      state: {
        state: 'idle',
        busy: false,
        needsInput: false,
        lastUpdatedAt: '2026-04-29T13:00:00Z',
        tasks: [],
      },
      taskTemplates: [
        {
          id: 'task-1',
          description: 'Collect AI news',
        },
      ],
      workflows: [],
    },
    detailQuery: {} as ControllerDetailController['detailQuery'],
    specQuery: {
      data: {
        spec: 'trigger:\n  type: manual\n',
      },
    } as ControllerDetailController['specQuery'],
    availableModels: [],
    isLoading: false,
    loadError: null,
    instructionDraft: 'Find AI news',
    setInstructionDraft: vi.fn(),
    operatorMessageDraft: '',
    setOperatorMessageDraft: vi.fn(),
    newTaskDescription: '',
    setNewTaskDescription: vi.fn(),
    taskEditDraft: null,
    taskEditDescription: '',
    setTaskEditDescription: vi.fn(),
    iconUrlDraft: '',
    setIconUrlDraft: vi.fn(),
    descriptionDraft: '',
    setDescriptionDraft: vi.fn(),
    goalDraft: 'Collect interesting AI news',
    setGoalDraft: vi.fn(),
    triggerPromptDraft: '',
    setTriggerPromptDraft: vi.fn(),
    resetOnFinishDraft: false,
    setResetOnFinishDraft: vi.fn(),
    triggerType: 'manual',
    triggerTypeDraft: 'manual',
    setTriggerTypeDraft: vi.fn(),
    cronScheduleDraft: '',
    setCronScheduleDraft: vi.fn(),
    workflowNamesDraft: ['collect-news'],
    setWorkflowNamesDraft: vi.fn(),
    workflowSearchQuery: '',
    setWorkflowSearchQuery: vi.fn(),
    availableDAGOptions: [],
    dagListQuery: {} as ControllerDetailController['dagListQuery'],
    modelDraft: '',
    setModelDraft: vi.fn(),
    isEditingMetadata: false,
    setIsEditingMetadata: vi.fn(),
    freeTextResponse: '',
    setFreeTextResponse: vi.fn(),
    selectedOptions: [],
    setSelectedOptions: vi.fn(),
    actionError: '',
    setActionError: vi.fn(),
    busyAction: null,
    confirmation: null,
    confirmButtonText: '',
    isEditingSpec: false,
    setIsEditingSpec: vi.fn(),
    specDraft: '',
    setSpecDraft: vi.fn(),
    specError: '',
    setSpecError: vi.fn(),
    isSavingSpec: false,
    mergedRuns: [],
    queuedTurnMessages: [],
    threadItems: [],
    lifecycleState: 'idle',
    controllerStatus: undefined,
    runtimeControllerReady: true,
    displayStatus: 'idle',
    taskSummary: { done: 0, open: 0 },
    canStartTask: true,
    canSendOperatorMessage: false,
    canPause: false,
    canResume: false,
    cronSchedulesConfigured: false,
    metadataChanged: false,
    metadataValidationError: '',
    metadataSaveDisabled: false,
    onWorkflowNamesChange: vi.fn(),
    onStart: vi.fn(),
    onCreateTask: vi.fn(),
    onToggleTask: vi.fn(),
    onEditTask: vi.fn(),
    onCancelTaskEdit: vi.fn(),
    onSaveTaskEdit: vi.fn(),
    onDeleteTask: vi.fn(),
    onMoveTask: vi.fn(),
    onRespond: vi.fn(),
    onSendOperatorMessage: vi.fn(),
    onPauseResume: vi.fn(),
    onRename: vi.fn(),
    onClone: vi.fn(),
    onResetState: vi.fn(),
    onDelete: vi.fn(),
    dismissConfirmation: vi.fn(),
    onConfirmAction: vi.fn(),
    onSaveSpec: vi.fn(),
    onSaveMetadata: vi.fn(),
    onCancelMetadata: vi.fn(),
    ...overrides,
  } as unknown as ControllerDetailController;
}

describe('ControllerDetailSurface', () => {
  it('keeps hook order stable when detail loads after the initial loading render', () => {
    const { container, rerender } = render(
      <ControllerDetailSurface
        controller={createController({
          detail: undefined,
          isLoading: true,
        })}
      />
    );

    expect(container.querySelector('.animate-spin')).toBeInTheDocument();

    rerender(
      <ControllerDetailSurface controller={createController()} />
    );

    expect(screen.getByText('Runtime State')).toBeInTheDocument();
    expect(screen.getByText('Artifacts')).toBeInTheDocument();
  });
});
