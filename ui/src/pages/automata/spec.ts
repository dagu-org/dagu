// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { parse, stringify } from 'yaml';

export function updateAutomataDescriptionInSpec(
  spec: string,
  description: string
): string {
  const parsed = parse(spec);
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error('Automata spec must be a YAML object');
  }

  const nextSpec = { ...(parsed as Record<string, unknown>) };

  const nextDescription = description.trim();
  if (nextDescription) {
    nextSpec.description = nextDescription;
  } else {
    delete nextSpec.description;
  }

  return stringify(nextSpec);
}
