// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  AgentMessageType,
  AutopilotDisplayStatus,
  Status,
  type components,
} from '@/api/v1/schema';
import { CronExpressionParser } from 'cron-parser';
import dayjs from '@/lib/dayjs';

export type AutopilotDetail = components['schemas']['AutopilotDetailResponse'];
export type AgentMessage = components['schemas']['AgentMessage'];
export type AutopilotPendingTurnMessage =
  components['schemas']['AutopilotPendingTurnMessage'];
export type AutopilotRunSummary = components['schemas']['AutopilotRunSummary'];
export type AutopilotTask = components['schemas']['AutopilotTask'];
export type AutopilotTaskTemplate =
  components['schemas']['AutopilotTaskTemplate'];
export type AutopilotDisplayState =
  components['schemas']['AutopilotDisplayStatus'];

export type AutopilotThreadItem =
  | {
      id: string;
      kind: 'queued';
      createdAt?: string;
      queuedKind: string;
      message: string;
    }
  | {
      id: string;
      kind: 'message';
      createdAt?: string;
      message: AgentMessage;
    };

export function autopilotDisplayName(item: {
  name?: string;
  nickname?: string | null;
}): string {
  return item.nickname?.trim() || item.name || '';
}

export function displayStatusClass(
  state?: AutopilotDisplayState | string
): string {
  switch (state) {
    case AutopilotDisplayStatus.running:
      return 'bg-sky-100 text-sky-800 dark:bg-sky-900/40 dark:text-sky-200';
    case AutopilotDisplayStatus.paused:
      return 'bg-slate-200 text-slate-900 dark:bg-slate-800 dark:text-slate-100';
    case AutopilotDisplayStatus.finished:
      return 'bg-emerald-100 text-emerald-900 dark:bg-emerald-900/40 dark:text-emerald-200';
    default:
      return 'bg-muted text-muted-foreground';
  }
}

export function taskCounts(tasks?: AutopilotTask[]): {
  open: number;
  done: number;
} {
  const items = tasks || [];
  return {
    open: items.filter((task) => task.state === 'open').length,
    done: items.filter((task) => task.state === 'done').length,
  };
}

export function parseAutopilotScheduleText(value: string): string[] {
  return value
    .split('\n')
    .map((item) => item.trim())
    .filter(Boolean);
}

export function formatAutopilotScheduleText(items?: string[]): string {
  return (items || []).join('\n');
}

export function validateAutopilotScheduleExpressions(items: string[]): string | null {
  for (const item of items) {
    try {
      CronExpressionParser.parse(item);
    } catch {
      return `Invalid cron schedule: ${item}`;
    }
  }
  return null;
}

export function dagRunStatusToStatus(status?: string): Status | undefined {
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

export function formatAbsoluteTime(value?: string): string {
  if (!value || value === '-') {
    return 'n/a';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.format('MMM D, YYYY HH:mm') : 'n/a';
}

export function formatRelativeTime(value?: string): string {
  if (!value || value === '-') {
    return 'n/a';
  }
  const parsed = dayjs(value);
  return parsed.isValid() ? parsed.fromNow() : 'n/a';
}

export function isValidAutopilotIconUrl(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) {
    return true;
  }
  if (trimmed.startsWith('/') && !trimmed.startsWith('//')) {
    return true;
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === 'http:' || parsed.protocol === 'https:';
  } catch {
    return false;
  }
}

export function buildAutopilotThread(
  messages: AgentMessage[] | undefined,
  queuedMessages: AutopilotPendingTurnMessage[] | undefined
): AutopilotThreadItem[] {
  const merged: Array<
    AutopilotThreadItem & {
      sortTime: number;
      sortSequence: number;
      sortIndex: number;
    }
  > = [];

  for (const [index, queuedMessage] of (queuedMessages || []).entries()) {
    merged.push({
      id: `queued-${queuedMessage.id}`,
      kind: 'queued',
      createdAt: queuedMessage.createdAt,
      queuedKind: queuedMessage.kind,
      message: queuedMessage.message,
      sortTime: parseSortTime(queuedMessage.createdAt),
      sortSequence: Number.MAX_SAFE_INTEGER - 1,
      sortIndex: index,
    });
  }

  for (const [index, message] of (messages || []).entries()) {
    merged.push({
      id: message.id,
      kind: 'message',
      createdAt: message.createdAt,
      message,
      sortTime: parseSortTime(message.createdAt),
      sortSequence: message.sequenceId ?? Number.MAX_SAFE_INTEGER,
      sortIndex: index + merged.length,
    });
  }

  merged.sort((a, b) => {
    if (a.sortTime !== b.sortTime) {
      return a.sortTime - b.sortTime;
    }
    if (a.sortSequence !== b.sortSequence) {
      return a.sortSequence - b.sortSequence;
    }
    return a.sortIndex - b.sortIndex;
  });

  return merged.map((item): AutopilotThreadItem => {
    if (item.kind === 'queued') {
      return {
        id: item.id,
        kind: item.kind,
        createdAt: item.createdAt,
        queuedKind: item.queuedKind,
        message: item.message,
      };
    }
    return {
      id: item.id,
      kind: item.kind,
      createdAt: item.createdAt,
      message: item.message,
    };
  });
}

export function agentMessageLabel(type?: string): string {
  switch (type) {
    case AgentMessageType.user:
      return 'You';
    case AgentMessageType.assistant:
      return 'Autopilot';
    case AgentMessageType.error:
      return 'Error';
    case AgentMessageType.user_prompt:
      return 'Prompt';
    case AgentMessageType.ui_action:
      return 'System';
    default:
      return type || 'Message';
  }
}

function parseSortTime(value?: string): number {
  if (!value) {
    return Number.MAX_SAFE_INTEGER;
  }
  const parsed = dayjs(value);
  if (!parsed.isValid()) {
    return Number.MAX_SAFE_INTEGER;
  }
  return parsed.valueOf();
}
