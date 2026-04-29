// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { parse, stringify } from 'yaml';

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? { ...(value as Record<string, unknown>) }
    : {};
}

function normalizeNameList(items: string[]): string[] {
  return Array.from(new Set(items.map((item) => item.trim()).filter(Boolean)));
}

export function updateControllerMetadataInSpec(
  spec: string,
  metadata: {
    description: string;
    iconUrl: string;
    goal: string;
    model: string;
    triggerPrompt: string;
    resetOnFinish: boolean;
    triggerType: 'manual' | 'cron';
    cronSchedules: string[];
    workflowNames?: string[];
  }
): string {
  const parsed = parse(spec);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Controller spec must be a YAML object');
  }

  const nextSpec = { ...(parsed as Record<string, unknown>) };

  const nextDescription = metadata.description.trim();
  if (nextDescription) {
    nextSpec.description = nextDescription;
  } else {
    delete nextSpec.description;
  }

  const nextIconUrl = metadata.iconUrl.trim();
  if (nextIconUrl) {
    nextSpec.icon_url = nextIconUrl;
  } else {
    delete nextSpec.icon_url;
  }

  const nextGoal = metadata.goal.trim();
  if (nextGoal) {
    nextSpec.goal = nextGoal;
  } else {
    delete nextSpec.goal;
  }

  if (metadata.resetOnFinish) {
    nextSpec.reset_on_finish = true;
  } else {
    delete nextSpec.reset_on_finish;
    delete nextSpec.resetOnFinish;
  }

  const nextTriggerPrompt = metadata.triggerPrompt.trim();
  nextSpec.trigger =
    metadata.triggerType === 'cron'
      ? {
          type: 'cron',
          schedules: [...metadata.cronSchedules],
          ...(nextTriggerPrompt ? { prompt: nextTriggerPrompt } : {}),
        }
      : { type: 'manual' };

  if (metadata.workflowNames !== undefined) {
    const workflowNames = normalizeNameList(metadata.workflowNames);
    const workflows = asRecord(nextSpec.workflows);
    if (workflowNames.length > 0) {
      workflows.names = workflowNames;
    } else {
      delete workflows.names;
    }
    if (Object.keys(workflows).length > 0) {
      nextSpec.workflows = workflows;
    } else {
      delete nextSpec.workflows;
    }
  }

  const nextModel = metadata.model.trim();
  const currentAgent =
    nextSpec.agent &&
    typeof nextSpec.agent === 'object' &&
    !Array.isArray(nextSpec.agent)
      ? { ...(nextSpec.agent as Record<string, unknown>) }
      : {};

  if (nextModel) {
    currentAgent.model = nextModel;
  } else {
    delete currentAgent.model;
  }

  if (Object.keys(currentAgent).length > 0) {
    nextSpec.agent = currentAgent;
  } else {
    delete nextSpec.agent;
  }

  return stringify(nextSpec);
}
