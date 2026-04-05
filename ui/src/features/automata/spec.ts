// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { parse, stringify } from 'yaml';

export function updateAutomataMetadataInSpec(
  spec: string,
  metadata: {
    description: string;
    iconUrl: string;
    goal: string;
    model: string;
    standingInstruction: string;
    schedule: string[];
  }
): string {
  const parsed = parse(spec);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Automata spec must be a YAML object');
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

  if (metadata.schedule.length === 1) {
    nextSpec.schedule = metadata.schedule[0];
  } else if (metadata.schedule.length > 1) {
    nextSpec.schedule = [...metadata.schedule];
  } else {
    delete nextSpec.schedule;
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
