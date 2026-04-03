// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { parse, stringify } from 'yaml';

export function updateAutomataMetadataInSpec(
  spec: string,
  metadata: {
    description: string;
    iconUrl: string;
    goal: string;
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

  nextSpec.goal = metadata.goal.trim();

  return stringify(nextSpec);
}
