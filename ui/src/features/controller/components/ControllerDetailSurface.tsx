// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import React from 'react';

import {
  ControllerDocument,
  AgentMessageType,
  type components,
} from '@/api/v1/schema';
import { Button } from '@/components/ui/button';
import ConfirmModal from '@/components/ui/confirm-dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import LoadingIndicator from '@/components/ui/loading-indicator';
import { Tab, Tabs } from '@/components/ui/tabs';
import { Textarea } from '@/components/ui/textarea';
import { Switch } from '@/components/ui/switch';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { ControllerAvatar } from '@/features/controller/components/ControllerAvatar';
import { DAGNamePicker } from '@/features/controller/components/ControllerCreateModal';
import { ControllerDocumentSection } from '@/features/controller/components/ControllerMemorySection';
import type { ControllerDetailController } from '@/features/controller/hooks/useControllerDetail';
import {
  agentMessageLabel,
  controllerDisplayName,
  dagRunStatusToStatus,
  displayStatusClass,
  formatAbsoluteTime,
  type ControllerRunSummary,
  type ControllerTask,
  type ControllerTaskTemplate,
} from '@/features/controller/detail-utils';
import { cn } from '@/lib/utils';
import StatusChip from '@/components/ui/status-chip';
import DAGDetailsModal from '@/features/dags/components/dag-details/DAGDetailsModal';
import DAGRunDetailsModal from '@/features/dag-runs/components/dag-run-details/DAGRunDetailsModal';

type DetailTab = 'status' | 'config';

function controllerStatusMessage(
  status?: components['schemas']['ControllerStatus']
): string | undefined {
  if (!status) {
    return 'Scheduler controller readiness is unknown.';
  }
  if (status.message) {
    return status.message;
  }
  switch (status.state) {
    case 'ready':
      return 'Controller is ready.';
    case 'disabled':
      return 'Controller is disabled in agent settings.';
    case 'unavailable':
      return 'No active scheduler with a ready Controller is available.';
    default:
      return 'Scheduler controller readiness is unknown.';
  }
}

function RunRow({
  run,
  onOpen,
}: {
  run: ControllerRunSummary & { isCurrent?: boolean };
  onOpen: (run: ControllerRunSummary) => void;
}): React.ReactElement {
  return (
    <button
      type="button"
      onClick={() => onOpen(run)}
      className="grid w-full gap-2 rounded-md border p-3 text-left transition hover:bg-muted/40 lg:grid-cols-[1fr_180px_160px]"
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <div className="truncate text-sm font-medium">{run.name}</div>
          {run.isCurrent ? (
            <span className="rounded-full bg-primary/10 px-2 py-0.5 text-[11px] font-medium text-primary">
              current
            </span>
          ) : null}
        </div>
        <div className="mt-1 text-xs text-muted-foreground">{run.dagRunId}</div>
      </div>
      <div>
        <StatusChip status={dagRunStatusToStatus(run.status)} size="xs">
          {run.status}
        </StatusChip>
      </div>
      <div className="text-xs text-muted-foreground">
        {run.finishedAt || run.startedAt || run.createdAt || ''}
      </div>
    </button>
  );
}

function ThreadBubble({
  item,
}: {
  item: ControllerDetailController['threadItems'][number];
}): React.ReactElement {
  if (item.kind === 'queued') {
    return (
      <div className="flex items-start">
        <div className="max-w-[90%] rounded-2xl border border-amber-300/40 bg-amber-50 px-4 py-3 text-sm dark:border-amber-700/40 dark:bg-amber-950/20">
          <div className="mb-1 flex items-center justify-between gap-4 text-[11px] font-medium uppercase tracking-wide text-amber-800 dark:text-amber-200">
            <span>{item.queuedKind.replace(/_/g, ' ')} queued</span>
            {item.createdAt ? (
              <span className="normal-case tracking-normal">
                {formatAbsoluteTime(item.createdAt)}
              </span>
            ) : null}
          </div>
          <div className="whitespace-pre-wrap break-words">{item.message}</div>
        </div>
      </div>
    );
  }

  const message = item.message;
  const isUser = message.type === AgentMessageType.user;
  const isError = message.type === AgentMessageType.error;

  return (
    <div className={cn('flex', isUser ? 'justify-end' : 'justify-start')}>
      <div
        className={cn(
          'max-w-[90%] rounded-2xl border px-4 py-3 text-sm',
          isUser
            ? 'border-primary/20 bg-primary/5'
            : isError
              ? 'border-destructive/30 bg-destructive/5'
              : 'bg-muted/40'
        )}
      >
        <div className="mb-1 flex items-center justify-between gap-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
          <span>{agentMessageLabel(message.type)}</span>
          {message.createdAt ? (
            <span className="normal-case tracking-normal">
              {formatAbsoluteTime(message.createdAt)}
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
          <div className="mt-3 space-y-2">
            {message.toolResults.map((result, index) => (
              <div
                key={`${message.id}-tool-${index}`}
                className="rounded-md border bg-background/80 p-2 text-xs whitespace-pre-wrap break-words"
              >
                {result.content}
              </div>
            ))}
          </div>
        ) : null}
      </div>
    </div>
  );
}

function TalkThread({
  items,
  sessionId,
  active,
}: {
  items: ControllerDetailController['threadItems'];
  sessionId?: string;
  active: boolean;
}): React.ReactElement {
  const containerRef = React.useRef<HTMLDivElement | null>(null);
  const shouldFollowRef = React.useRef(true);

  const scrollToBottom = React.useCallback(() => {
    const node = containerRef.current;
    if (!node) {
      return;
    }
    node.scrollTop = node.scrollHeight;
  }, []);

  React.useEffect(() => {
    const node = containerRef.current;
    if (!node) {
      return;
    }

    const onScroll = () => {
      const remaining = node.scrollHeight - node.scrollTop - node.clientHeight;
      shouldFollowRef.current = remaining < 48;
    };

    onScroll();
    node.addEventListener('scroll', onScroll);
    return () => node.removeEventListener('scroll', onScroll);
  }, []);

  React.useLayoutEffect(() => {
    if (!active) {
      return;
    }
    shouldFollowRef.current = true;
    requestAnimationFrame(scrollToBottom);
  }, [active, scrollToBottom, sessionId]);

  React.useLayoutEffect(() => {
    if (!active || !shouldFollowRef.current) {
      return;
    }
    requestAnimationFrame(scrollToBottom);
  }, [active, items.length, scrollToBottom]);

  return (
    <div className="min-w-0 rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-sm font-semibold">Talk Thread</h2>
        {sessionId ? (
          <span className="text-[11px] text-muted-foreground">
            Session: {sessionId}
          </span>
        ) : null}
      </div>
      {items.length ? (
        <div
          ref={containerRef}
          className="max-h-[34rem] space-y-3 overflow-y-auto pr-1"
        >
          {items.map((item) => (
            <ThreadBubble key={item.id} item={item} />
          ))}
        </div>
      ) : (
        <div className="text-sm text-muted-foreground">
          No session or queued messages yet.
        </div>
      )}
    </div>
  );
}

function TaskList({
  tasks,
  newTaskDescription,
  setNewTaskDescription,
  onCreateTask,
  onMoveTask,
  onEditTask,
  editingTaskId,
  taskEditDescription,
  setTaskEditDescription,
  onCancelTaskEdit,
  onSaveTaskEdit,
  onDeleteTask,
  disabled,
  busyAction,
}: {
  tasks?: ControllerTaskTemplate[];
  newTaskDescription: string;
  setNewTaskDescription: (value: string) => void;
  onCreateTask: () => void | Promise<void>;
  onMoveTask: (
    task: ControllerTaskTemplate,
    direction: -1 | 1
  ) => void | Promise<void>;
  onEditTask: (task: ControllerTaskTemplate) => void | Promise<void>;
  editingTaskId?: string;
  taskEditDescription: string;
  setTaskEditDescription: (value: string) => void;
  onCancelTaskEdit: () => void;
  onSaveTaskEdit: () => void | Promise<void>;
  onDeleteTask: (task: ControllerTaskTemplate) => void | Promise<void>;
  disabled: boolean;
  busyAction?: string | null;
}): React.ReactElement {
  const items = tasks || [];

  return (
    <div className="min-w-0 rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold">Task List</h2>
          <div className="text-xs text-muted-foreground">
            Operators manage the persistent task template for each cycle.
          </div>
        </div>
        <span className="rounded-full border px-3 py-1 text-xs text-muted-foreground">
          {items.length} template{items.length === 1 ? '' : 's'}
        </span>
      </div>

      <div className="mb-4 grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto]">
        <Textarea
          value={newTaskDescription}
          onChange={(event) => setNewTaskDescription(event.target.value)}
          placeholder="Add a task description"
          disabled={disabled}
          rows={3}
          className="min-h-20"
        />
        <Button
          onClick={() => void onCreateTask()}
          disabled={!newTaskDescription.trim() || disabled}
          className="sm:self-start"
        >
          Add Task
        </Button>
      </div>

      {items.length ? (
        <div className="space-y-2">
          {items.map((task, index) => {
            const isEditing = editingTaskId === task.id;
            const isEditingAnyTask = !!editingTaskId;
            const editSaveDisabled =
              !taskEditDescription.trim() || !!busyAction;
            return (
              <div key={task.id} className="rounded-md border p-3">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="min-w-0 flex-1">
                    <div className="rounded-full bg-slate-100 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-slate-900 dark:bg-slate-800 dark:text-slate-100 inline-flex">
                      Task {index + 1}
                    </div>
                    {isEditing ? (
                      <div className="mt-2 space-y-2">
                        <Textarea
                          value={taskEditDescription}
                          onChange={(event) =>
                            setTaskEditDescription(event.target.value)
                          }
                          rows={5}
                          className="min-h-28 text-sm"
                          disabled={!!busyAction}
                        />
                        <div className="flex justify-end gap-2">
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            onClick={onCancelTaskEdit}
                            disabled={!!busyAction}
                          >
                            Cancel
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            onClick={() => void onSaveTaskEdit()}
                            disabled={editSaveDisabled}
                          >
                            {busyAction === 'edit-task'
                              ? 'Saving...'
                              : 'Save Task'}
                          </Button>
                        </div>
                      </div>
                    ) : (
                      <div className="mt-2 whitespace-pre-wrap break-words text-sm">
                        {task.description}
                      </div>
                    )}
                  </div>
                  <div className="flex flex-wrap items-center gap-2">
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void onMoveTask(task, -1)}
                      disabled={disabled || isEditingAnyTask || index === 0}
                    >
                      Up
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void onMoveTask(task, 1)}
                      disabled={
                        disabled ||
                        isEditingAnyTask ||
                        index === items.length - 1
                      }
                    >
                      Down
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void onEditTask(task)}
                      disabled={disabled || isEditingAnyTask}
                    >
                      Edit
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => void onDeleteTask(task)}
                      disabled={disabled || isEditingAnyTask}
                    >
                      Delete
                    </Button>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      ) : (
        <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
          No task templates yet. Add at least one task before starting this
          controller.
        </div>
      )}
    </div>
  );
}

function TaskProgress({
  tasks,
  summary,
}: {
  tasks?: ControllerTask[];
  summary: { open: number; done: number };
}): React.ReactElement {
  const items = tasks || [];
  const openTasks = items
    .map((task, index) => ({ task, number: index + 1 }))
    .filter((item) => item.task.state === 'open');

  return (
    <div className="min-w-0 rounded-lg border p-4">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold">Task Progress</h2>
          <div className="text-xs text-muted-foreground">
            Status only shows the current cycle. Edit task templates in Config.
          </div>
        </div>
        <span className="rounded-full border px-3 py-1 text-xs text-muted-foreground">
          {summary.done} done / {summary.open} open
        </span>
      </div>

      {items.length ? (
        <div className="space-y-2 text-sm">
          {items.length ? (
            <div className="text-xs text-muted-foreground">
              Current cycle task state resets from the Config task template when
              a new cycle starts.
            </div>
          ) : null}
          {openTasks.length ? (
            <>
              <div className="font-medium">Open tasks</div>
              <div className="space-y-2">
                {openTasks.map(({ task, number }) => (
                  <div
                    key={task.id}
                    className="rounded-md border bg-muted/30 px-3 py-2"
                  >
                    <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                      Task {number}
                    </div>
                    <div className="whitespace-pre-wrap break-words">
                      {task.description}
                    </div>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="text-muted-foreground">
              All task list items are currently done.
            </div>
          )}
        </div>
      ) : (
        <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
          No current cycle is active. Start the controller or wait for the next
          scheduled cycle.
        </div>
      )}
    </div>
  );
}

function StatusTab({
  controller,
  active,
  onOpenRun,
}: {
  controller: ControllerDetailController;
  active: boolean;
  onOpenRun: (run: ControllerRunSummary) => void;
}): React.ReactElement {
  const conversationTextareaRef = React.useRef<HTMLTextAreaElement | null>(
    null
  );
  const hasTaskTemplates = (controller.detail?.taskTemplates?.length || 0) > 0;
  const hasStandingInstruction =
    !!controller.detail?.definition?.standingInstruction?.trim();
  const startMode =
    controller.lifecycleState === 'idle' ||
    controller.lifecycleState === 'finished';
  const conversationDraft = startMode
    ? controller.instructionDraft
    : controller.operatorMessageDraft;
  const canStartCycle =
    controller.canStartTask &&
    hasTaskTemplates &&
    (!!controller.instructionDraft.trim() || hasStandingInstruction);
  const canSubmitConversation = startMode
    ? canStartCycle
    : !!controller.operatorMessageDraft.trim() &&
      controller.canSendOperatorMessage &&
      !controller.busyAction;

  React.useEffect(() => {
    const node = conversationTextareaRef.current;
    if (!node) {
      return;
    }
    node.style.height = '0px';
    node.style.height = `${node.scrollHeight}px`;
  }, [conversationDraft]);

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="min-w-0 rounded-lg border p-4">
          <h2 className="mb-3 text-sm font-semibold">Runtime State</h2>
          <div className="space-y-2 text-sm">
            <p>
              <span className="font-medium">Last updated:</span>{' '}
              {formatAbsoluteTime(controller.detail?.state.lastUpdatedAt)}
            </p>
            {controller.detail?.state.waitingReason ? (
              <p>
                <span className="font-medium">Waiting reason:</span>{' '}
                {controller.detail.state.waitingReason}
              </p>
            ) : null}
            {controller.detail?.state.lastSummary ? (
              <p className="whitespace-pre-wrap break-words">
                <span className="font-medium">Summary:</span>{' '}
                {controller.detail.state.lastSummary}
              </p>
            ) : null}
            {controller.detail?.state.lastError ? (
              <p className="whitespace-pre-wrap break-words text-destructive">
                <span className="font-medium">Error:</span>{' '}
                {controller.detail.state.lastError}
              </p>
            ) : null}
          </div>
        </div>

        <div className="min-w-0 rounded-lg border p-4">
          <h2 className="mb-3 text-sm font-semibold">Status Notes</h2>
          <div className="space-y-2 text-sm text-muted-foreground">
            <p>
              {controller.scheduleConfigured
                ? hasStandingInstruction && hasTaskTemplates
                  ? 'Due schedule ticks can start a fresh cycle automatically from the Config task template.'
                  : 'Schedule is configured, but it cannot start clean cycles until Standing Instruction and at least one task template are set in Config.'
                : 'No schedule is configured for this controller.'}
            </p>
            {controller.detail?.state.pendingPrompt ? (
              <p>
                Human input is currently required before the Controller can
                continue.
              </p>
            ) : controller.canSendOperatorMessage ? (
              <p>
                Operator messages will be appended to the current live thread.
              </p>
            ) : (
              <p>
                Live actions are gated by the current lifecycle state and
                scheduler controller readiness.
              </p>
            )}
          </div>
        </div>
      </div>

      <TalkThread
        items={controller.threadItems}
        sessionId={controller.detail?.state.sessionId || undefined}
        active={active}
      />

      {controller.detail?.state.pendingPrompt ? (
        <div className="rounded-lg border border-amber-400/40 bg-amber-50/50 p-4 dark:bg-amber-950/20">
          <h2 className="mb-2 text-sm font-semibold">
            Waiting For Human Input
          </h2>
          <p className="mb-3 text-sm">
            {controller.detail.state.pendingPrompt.question}
          </p>
          <div className="space-y-2">
            {controller.lifecycleState === 'paused' ? (
              <div className="rounded-md border border-slate-300/60 bg-slate-100/70 px-3 py-2 text-sm text-slate-800 dark:border-slate-700 dark:bg-slate-900/40 dark:text-slate-200">
                Response will be queued, and the Controller will remain paused
                until you resume it.
              </div>
            ) : null}
            {(controller.detail.state.pendingPrompt.options || []).map(
              (option) => {
                const selected = controller.selectedOptions.includes(option.id);
                return (
                  <label
                    key={option.id}
                    className="flex cursor-pointer items-start gap-2 rounded-md border p-2 text-sm"
                  >
                    <input
                      type="checkbox"
                      checked={selected}
                      onChange={(event) => {
                        controller.setSelectedOptions((prev) =>
                          event.target.checked
                            ? [...prev, option.id]
                            : prev.filter((id) => id !== option.id)
                        );
                      }}
                    />
                    <span>
                      <span className="font-medium">{option.label}</span>
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
              value={controller.freeTextResponse}
              onChange={(event) =>
                controller.setFreeTextResponse(event.target.value)
              }
              placeholder={
                controller.detail.state.pendingPrompt.freeTextPlaceholder ||
                'Add an optional note or free-text response'
              }
              disabled={!!controller.busyAction}
            />
            <Button
              onClick={() => void controller.onRespond()}
              disabled={!!controller.busyAction}
            >
              {controller.busyAction === 'respond'
                ? 'Submitting...'
                : 'Submit Response'}
            </Button>
          </div>
        </div>
      ) : null}

      <div className="min-w-0 rounded-lg border p-4">
        <h2 className="mb-3 text-sm font-semibold">Conversation</h2>
        <div className="space-y-3">
          {hasStandingInstruction ? (
            <div className="space-y-3 rounded-md border bg-muted/20 p-3">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Standing Instruction
              </div>
              {hasStandingInstruction ? (
                <div className="whitespace-pre-wrap break-words text-sm">
                  {controller.detail?.definition?.standingInstruction}
                </div>
              ) : (
                <div className="text-sm text-muted-foreground">
                  No standing instruction is configured yet.
                </div>
              )}
            </div>
          ) : null}

          <Textarea
            ref={conversationTextareaRef}
            value={conversationDraft}
            onChange={(event) => {
              if (startMode) {
                controller.setInstructionDraft(event.target.value);
                return;
              }
              controller.setOperatorMessageDraft(event.target.value);
            }}
            className="min-h-24 overflow-hidden resize-none"
            placeholder={
              startMode
                ? hasStandingInstruction
                  ? 'Add an optional one-off instruction for this cycle.'
                  : 'Tell this Controller what task to work on.'
                : 'Add context, change priority, or clarify the current task.'
            }
            disabled={
              startMode
                ? !controller.canStartTask || !!controller.busyAction
                : !controller.canSendOperatorMessage || !!controller.busyAction
            }
          />

          <div className="flex items-center justify-between gap-3">
            <div className="text-xs text-muted-foreground">
              {startMode
                ? controller.canStartTask
                  ? hasTaskTemplates
                    ? hasStandingInstruction
                      ? 'Starting creates a fresh cycle from the Config task template. The standing instruction is used unless you enter a one-off instruction.'
                      : 'Starting creates a fresh cycle from the Config task template.'
                    : 'Add at least one task template in Config before starting.'
                  : 'Starting is gated by scheduler controller readiness.'
                : controller.detail?.state.pendingPrompt
                  ? 'Respond to the pending prompt before sending a general operator message.'
                  : controller.canSendOperatorMessage
                    ? controller.detail?.state.state === 'paused'
                      ? 'This records your message now, but the Controller will stay paused until you resume it.'
                      : controller.detail?.state.currentRunRef
                        ? 'This records your message now and the Controller will pick it up after the current child DAG changes state.'
                        : 'This queues a user message into the active Controller task.'
                    : 'Operator messages are only accepted while the Controller has an active task.'}
            </div>
            <Button
              onClick={() => {
                if (startMode) {
                  void controller.onStart();
                  return;
                }
                void controller.onSendOperatorMessage();
              }}
              disabled={!canSubmitConversation || !!controller.busyAction}
            >
              {startMode
                ? controller.lifecycleState === 'finished'
                  ? 'Start New Task'
                  : 'Start'
                : controller.busyAction === 'message'
                  ? 'Sending...'
                  : 'Send Message'}
            </Button>
          </div>
        </div>
      </div>

      <div className="min-w-0 rounded-lg border p-4">
        <h2 className="mb-3 text-sm font-semibold">Recent Runs</h2>
        {controller.mergedRuns.length ? (
          <div className="space-y-2">
            {controller.mergedRuns.map((run) => (
              <RunRow
                key={`${run.name}:${run.dagRunId}`}
                run={run}
                onOpen={onOpenRun}
              />
            ))}
          </div>
        ) : (
          <div className="rounded-md border border-dashed p-3 text-sm text-muted-foreground">
            No child DAG runs yet.
          </div>
        )}
      </div>

      <TaskProgress
        tasks={controller.detail?.state.tasks}
        summary={controller.taskSummary}
      />
    </div>
  );
}

function ConfigTab({
  controller,
  onOpenDAG,
}: {
  controller: ControllerDetailController;
  onOpenDAG: (dagName: string) => void;
}): React.ReactElement {
  const metadataFieldDisabled =
    !!controller.busyAction ||
    controller.isSavingSpec ||
    controller.isEditingSpec;
  const allowedDAGTags = controller.detail?.definition.allowedDAGs?.tags || [];
  const selectedAllowedDAGs = React.useMemo(() => {
    const infoByName = new Map(
      (controller.detail?.allowedDags || []).map((dag) => [dag.name, dag])
    );
    return controller.allowedDAGNamesDraft.map((name) => ({
      name,
      info: infoByName.get(name),
    }));
  }, [controller.allowedDAGNamesDraft, controller.detail?.allowedDags]);

  return (
    <div className="space-y-4">
      <div className="grid gap-4 lg:grid-cols-2">
        <div className="min-w-0 rounded-lg border p-4">
          <h2 className="mb-3 text-sm font-semibold">Metadata</h2>
          <div className="space-y-3 text-sm">
            <div className="grid gap-2">
              <Label htmlFor="controller-detail-goal">Goal</Label>
              <Textarea
                id="controller-detail-goal"
                value={controller.goalDraft}
                onChange={(event) => {
                  controller.setGoalDraft(event.target.value);
                  controller.setIsEditingMetadata(true);
                }}
                placeholder="Complete the assigned task and leave it ready for review"
                disabled={metadataFieldDisabled}
              />
              <div className="text-xs text-muted-foreground">
                Optional. Leave blank if this Controller should work from the
                instruction, task list, and runtime context.
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="controller-detail-standing-instruction">
                Standing Instruction
              </Label>
              <Textarea
                id="controller-detail-standing-instruction"
                value={controller.standingInstructionDraft}
                onChange={(event) => {
                  controller.setStandingInstructionDraft(event.target.value);
                  controller.setIsEditingMetadata(true);
                }}
                placeholder="Handle each scheduled cycle and use the task list as the default operating checklist."
                disabled={metadataFieldDisabled}
              />
              <div className="text-xs text-muted-foreground">
                Optional. Used as the default instruction for starts and
                scheduled cycles.
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="controller-detail-schedule">Schedule</Label>
              <Textarea
                id="controller-detail-schedule"
                value={controller.scheduleDraft}
                onChange={(event) => {
                  controller.setScheduleDraft(event.target.value);
                  controller.setIsEditingMetadata(true);
                }}
                placeholder={'0 * * * *\n30 9 * * 1-5'}
                disabled={metadataFieldDisabled}
                rows={3}
              />
              <div className="text-xs text-muted-foreground">
                Optional. Use one cron expression per line. Due ticks start a
                fresh cycle by reopening the Config task template.
              </div>
            </div>

            <div className="flex items-center justify-between gap-4 rounded-md border px-3 py-2">
              <Label htmlFor="controller-detail-reset-on-finish">
                Reset on finish
              </Label>
              <Switch
                id="controller-detail-reset-on-finish"
                checked={controller.resetOnFinishDraft}
                onCheckedChange={(checked) => {
                  controller.setResetOnFinishDraft(checked);
                  controller.setIsEditingMetadata(true);
                }}
                disabled={metadataFieldDisabled}
              />
            </div>

            <div className="grid gap-2">
              <Label htmlFor="controller-detail-icon-url">Image URL</Label>
              <Input
                id="controller-detail-icon-url"
                value={controller.iconUrlDraft}
                onChange={(event) => {
                  controller.setIconUrlDraft(event.target.value);
                  controller.setIsEditingMetadata(true);
                }}
                placeholder="https://cdn.example.com/controller/build-captain.png"
                disabled={metadataFieldDisabled}
              />
              <div className="text-xs text-muted-foreground">
                Optional. Use an absolute http(s) URL or a root-relative path
                like /assets/controller/build-captain.png.
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="controller-detail-model">Model</Label>
              <Select
                value={controller.modelDraft || '__inherit__'}
                onValueChange={(value) => {
                  controller.setModelDraft(
                    value === '__inherit__' ? '' : value
                  );
                  controller.setIsEditingMetadata(true);
                }}
                disabled={metadataFieldDisabled}
              >
                <SelectTrigger id="controller-detail-model">
                  <SelectValue placeholder="Use global default model" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="__inherit__">
                    Use global default model
                  </SelectItem>
                  {controller.detail?.definition.agent?.model &&
                  !controller.availableModels.some(
                    (model) =>
                      model.id === controller.detail?.definition.agent?.model
                  ) ? (
                    <SelectItem
                      value={controller.detail.definition.agent.model}
                    >
                      {controller.detail.definition.agent.model} (missing)
                    </SelectItem>
                  ) : null}
                  {controller.availableModels.map((model) => (
                    <SelectItem key={model.id} value={model.id}>
                      {model.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <div className="text-xs text-muted-foreground">
                Set a Controller-specific model here instead of inheriting the
                global default used by fresh sessions.
              </div>
            </div>

            <div className="grid gap-2">
              <Label htmlFor="controller-detail-description">Description</Label>
              <Input
                id="controller-detail-description"
                value={controller.descriptionDraft}
                onChange={(event) => {
                  controller.setDescriptionDraft(event.target.value);
                  controller.setIsEditingMetadata(true);
                }}
                placeholder="Optional description"
                disabled={metadataFieldDisabled}
              />
            </div>

            {controller.detail?.definition.tags?.length ? (
              <div>
                <span className="font-medium">Tags:</span>
                <div className="mt-1 flex flex-wrap gap-1">
                  {controller.detail.definition.tags.map((tag) => (
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

            <div className="space-y-3 border-t pt-3">
              <div className="text-xs text-muted-foreground">
                {controller.isEditingSpec
                  ? 'Save or cancel raw spec edits before updating metadata here.'
                  : 'This updates the top-level metadata fields in the Controller spec.'}
              </div>
              {controller.metadataValidationError ? (
                <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {controller.metadataValidationError}
                </div>
              ) : null}
              <div className="flex items-center justify-end gap-2">
                {controller.metadataChanged ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={controller.onCancelMetadata}
                    disabled={
                      !!controller.busyAction || controller.isSavingSpec
                    }
                  >
                    Cancel
                  </Button>
                ) : null}
                <Button
                  type="button"
                  size="sm"
                  onClick={() => void controller.onSaveMetadata()}
                  disabled={controller.metadataSaveDisabled}
                >
                  {controller.busyAction === 'metadata'
                    ? 'Saving...'
                    : 'Save Metadata'}
                </Button>
              </div>
            </div>
          </div>
        </div>

        <div className="min-w-0 rounded-lg border p-4">
          <h2 className="mb-3 text-sm font-semibold">Allowed DAGs</h2>
          <div className="space-y-4 text-sm">
            <div className="grid gap-2">
              <div className="flex items-center justify-between gap-3">
                <Label>Allowed DAG names</Label>
                <span className="text-xs text-muted-foreground">
                  {controller.dagListQuery.isLoading
                    ? 'Loading DAGs...'
                    : `${controller.allowedDAGNamesDraft.length} selected`}
                </span>
              </div>
              <DAGNamePicker
                availableDAGs={controller.availableDAGOptions}
                selectedNames={controller.allowedDAGNamesDraft}
                onChange={(names) => {
                  controller.setAllowedDAGNamesDraft(names);
                  controller.setIsEditingMetadata(true);
                }}
                searchQuery={controller.allowedDAGSearchQuery}
                onSearchQueryChange={controller.setAllowedDAGSearchQuery}
                isLoading={controller.dagListQuery.isLoading}
                disabled={metadataFieldDisabled}
              />
              <div className="text-xs text-muted-foreground">
                Remove selected DAGs from the chips above. Use the list below
                to inspect each DAG.
              </div>
              {controller.dagListQuery.error ? (
                <div className="rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                  {controller.dagListQuery.error instanceof Error
                    ? controller.dagListQuery.error.message
                    : 'Failed to load DAGs'}
                </div>
              ) : null}
              {allowedDAGTags.length ? (
                <div className="rounded-md border bg-muted/20 px-3 py-2 text-xs text-muted-foreground">
                  The raw spec also allows DAGs matching tags:{' '}
                  <span className="font-mono">{allowedDAGTags.join(', ')}</span>
                </div>
              ) : null}
            </div>

            <div className="space-y-2">
              <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                Selected DAGs
              </div>
              {selectedAllowedDAGs.length ? (
                selectedAllowedDAGs.map(({ name, info }) => (
                  <button
                    key={name}
                    type="button"
                    onClick={() => onOpenDAG(name)}
                    className="w-full rounded-md border p-3 text-left transition hover:bg-muted/50"
                  >
                    <div className="font-medium">{name}</div>
                    {info?.description ? (
                      <div className="text-xs text-muted-foreground">
                        {info.description}
                      </div>
                    ) : null}
                    {info?.tags?.length ? (
                      <div className="mt-1 text-[11px] text-muted-foreground">
                        {info.tags.join(', ')}
                      </div>
                    ) : null}
                  </button>
                ))
              ) : (
                <div className="rounded-md border border-dashed p-3 text-muted-foreground">
                  No allowed DAG names are selected.
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      <TaskList
        tasks={controller.detail?.taskTemplates}
        newTaskDescription={controller.newTaskDescription}
        setNewTaskDescription={controller.setNewTaskDescription}
        onCreateTask={controller.onCreateTask}
        onMoveTask={controller.onMoveTask}
        onEditTask={controller.onEditTask}
        editingTaskId={controller.taskEditDraft?.id}
        taskEditDescription={controller.taskEditDescription}
        setTaskEditDescription={controller.setTaskEditDescription}
        onCancelTaskEdit={controller.onCancelTaskEdit}
        onSaveTaskEdit={controller.onSaveTaskEdit}
        onDeleteTask={controller.onDeleteTask}
        disabled={!!controller.busyAction}
        busyAction={controller.busyAction}
      />

      <div className="grid gap-4 lg:grid-cols-2">
        <ControllerDocumentSection
          controllerName={controller.detail?.definition.name || ''}
          document={ControllerDocument.MEMORY_md}
        />
        <ControllerDocumentSection
          controllerName={controller.detail?.definition.name || ''}
          document={ControllerDocument.SOUL_md}
        />
      </div>

      <div className="min-w-0 rounded-lg border p-4">
        <div className="mb-3 flex items-center justify-between gap-2">
          <h2 className="text-sm font-semibold">Raw Spec</h2>
          <div className="flex items-center gap-2">
            {controller.isEditingSpec ? (
              <>
                <Button
                  size="sm"
                  variant="ghost"
                  onClick={() => {
                    controller.setIsEditingSpec(false);
                    controller.setSpecDraft(
                      controller.specQuery.data?.spec || ''
                    );
                    controller.setSpecError('');
                  }}
                  disabled={controller.isSavingSpec}
                >
                  Cancel
                </Button>
                <Button
                  size="sm"
                  onClick={() => void controller.onSaveSpec()}
                  disabled={controller.isSavingSpec}
                >
                  {controller.isSavingSpec ? 'Saving...' : 'Save'}
                </Button>
              </>
            ) : (
              <Button
                size="sm"
                variant="outline"
                onClick={() => {
                  controller.setSpecDraft(
                    controller.specQuery.data?.spec || ''
                  );
                  controller.setSpecError('');
                  controller.setIsEditingSpec(true);
                }}
              >
                Edit Spec
              </Button>
            )}
          </div>
        </div>
        {controller.specError ? (
          <div className="mb-3 rounded-md border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {controller.specError}
          </div>
        ) : null}
        {controller.isEditingSpec ? (
          <Textarea
            value={controller.specDraft}
            onChange={(event) => controller.setSpecDraft(event.target.value)}
            className="min-h-[28rem] w-full min-w-0 font-mono text-xs"
          />
        ) : (
          <pre className="max-h-[28rem] max-w-full overflow-auto whitespace-pre-wrap break-words rounded-md bg-muted p-3 text-xs">
            {controller.specQuery.data?.spec || ''}
          </pre>
        )}
      </div>
    </div>
  );
}

export function ControllerDetailSurface({
  controller,
  headerCaption,
  renderHeaderActions,
}: {
  controller: ControllerDetailController;
  headerCaption?: string;
  renderHeaderActions?: (
    controller: ControllerDetailController
  ) => React.ReactNode;
}): React.ReactElement {
  const [activeTab, setActiveTab] = React.useState<DetailTab>('status');
  const [selectedRun, setSelectedRun] =
    React.useState<ControllerRunSummary | null>(null);
  const [selectedDAG, setSelectedDAG] = React.useState<string | null>(null);

  if (controller.isLoading && !controller.detail) {
    return <LoadingIndicator />;
  }

  if (controller.loadError && !controller.detail) {
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/10 p-4 text-sm text-destructive">
        {controller.loadError instanceof Error
          ? controller.loadError.message
          : 'Failed to load Controller details'}
      </div>
    );
  }

  if (!controller.detail) {
    return (
      <div className="p-8 text-sm text-muted-foreground">
        Controller definition not found.
      </div>
    );
  }

  const runtimeControllerMessage = controllerStatusMessage(
    controller.controllerStatus
  );

  return (
    <>
      <div className="space-y-6">
        <div className="flex flex-wrap items-start justify-between gap-4">
          <div className="flex min-w-0 items-start gap-4">
            <ControllerAvatar
              name={controller.detail.definition.name}
              nickname={controller.detail.definition.nickname}
              iconUrl={controller.detail.definition.iconUrl}
              className="h-16 w-16 rounded-2xl"
            />
            <div className="min-w-0">
              {headerCaption ? (
                <div className="text-xs text-muted-foreground">
                  {headerCaption}
                </div>
              ) : null}
              <h1 className="truncate text-2xl font-semibold">
                {controllerDisplayName(controller.detail.definition)}
              </h1>
              {controller.detail.definition.nickname ? (
                <div className="mt-1 truncate font-mono text-xs text-muted-foreground">
                  {controller.detail.definition.name}
                </div>
              ) : null}
              {controller.detail.definition.description ? (
                <p className="mt-1 text-sm text-muted-foreground">
                  {controller.detail.definition.description}
                </p>
              ) : null}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {controller.canPause || controller.canResume ? (
              <Button
                variant="outline"
                size="sm"
                onClick={() => void controller.onPauseResume()}
                disabled={
                  controller.busyAction === 'pause' ||
                  controller.busyAction === 'resume'
                }
              >
                {controller.canResume ? 'Resume' : 'Pause'}
              </Button>
            ) : null}
            {renderHeaderActions ? renderHeaderActions(controller) : null}
          </div>
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <span
            className={`rounded-full px-3 py-1 text-xs font-medium ${displayStatusClass(controller.displayStatus)}`}
          >
            {controller.displayStatus}
          </span>
          {controller.detail.state.busy ? (
            <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-200">
              busy
            </span>
          ) : null}
          {controller.detail.state.needsInput ? (
            <span className="rounded-full bg-rose-100 px-3 py-1 text-xs font-medium text-rose-900 dark:bg-rose-900/40 dark:text-rose-200">
              needs input
            </span>
          ) : null}
          <span className="rounded-full bg-muted px-3 py-1 text-xs font-medium">
            Tasks {controller.taskSummary.done}/
            {controller.taskSummary.done + controller.taskSummary.open}
          </span>
          {controller.detail.definition.disabled ? (
            <span className="rounded-full border px-3 py-1 text-xs font-medium text-muted-foreground">
              disabled
            </span>
          ) : null}
        </div>

        {controller.actionError ? (
          <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-sm text-destructive">
            {controller.actionError}
          </div>
        ) : null}

        {!controller.runtimeControllerReady ? (
          <div className="rounded-lg border border-amber-300/40 bg-amber-100/70 px-3 py-2 text-sm text-amber-950 dark:border-amber-700/40 dark:bg-amber-950/30 dark:text-amber-100">
            {runtimeControllerMessage}
          </div>
        ) : null}

        <div>
          <Tabs className="whitespace-nowrap">
            <Tab
              isActive={activeTab === 'status'}
              onClick={() => setActiveTab('status')}
            >
              Status
            </Tab>
            <Tab
              isActive={activeTab === 'config'}
              onClick={() => setActiveTab('config')}
            >
              Config
            </Tab>
          </Tabs>
        </div>

        {activeTab === 'status' ? (
          <StatusTab
            controller={controller}
            active={activeTab === 'status'}
            onOpenRun={setSelectedRun}
          />
        ) : (
          <ConfigTab controller={controller} onOpenDAG={setSelectedDAG} />
        )}
      </div>

      {selectedRun ? (
        <DAGRunDetailsModal
          name={selectedRun.name}
          dagRunId={selectedRun.dagRunId}
          isOpen={!!selectedRun}
          onClose={() => setSelectedRun(null)}
        />
      ) : null}

      {selectedDAG ? (
        <DAGDetailsModal
          fileName={selectedDAG}
          isOpen={!!selectedDAG}
          onClose={() => setSelectedDAG(null)}
        />
      ) : null}

      {controller.confirmation ? (
        <ConfirmModal
          title={controller.confirmation.title}
          buttonText={controller.confirmButtonText}
          visible={!!controller.confirmation}
          dismissModal={controller.dismissConfirmation}
          onSubmit={() => void controller.onConfirmAction()}
        >
          <p className="text-sm text-muted-foreground">
            {controller.confirmation.message}
          </p>
        </ConfirmModal>
      ) : null}
    </>
  );
}
