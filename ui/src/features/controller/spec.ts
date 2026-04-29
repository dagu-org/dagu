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
    standingInstruction: string;
    resetOnFinish: boolean;
    schedule: string[];
    allowedDAGNames?: string[];
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

  const nextStandingInstruction = metadata.standingInstruction.trim();
  if (nextStandingInstruction) {
    nextSpec.standing_instruction = nextStandingInstruction;
  } else {
    delete nextSpec.standing_instruction;
  }

  if (metadata.resetOnFinish) {
    nextSpec.reset_on_finish = true;
  } else {
    delete nextSpec.reset_on_finish;
    delete nextSpec.resetOnFinish;
  }

  if (metadata.schedule.length === 1) {
    nextSpec.schedule = metadata.schedule[0];
  } else if (metadata.schedule.length > 1) {
    nextSpec.schedule = [...metadata.schedule];
  } else {
    delete nextSpec.schedule;
  }

  if (metadata.allowedDAGNames !== undefined) {
    const allowedDAGNames = normalizeNameList(metadata.allowedDAGNames);
    const allowedDAGs = asRecord(nextSpec.allowed_dags ?? nextSpec.allowedDAGs);
    if (allowedDAGNames.length > 0) {
      allowedDAGs.names = allowedDAGNames;
    } else {
      delete allowedDAGs.names;
    }
    if (Object.keys(allowedDAGs).length > 0) {
      nextSpec.allowed_dags = allowedDAGs;
    } else {
      delete nextSpec.allowed_dags;
    }
    delete nextSpec.allowedDAGs;
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
