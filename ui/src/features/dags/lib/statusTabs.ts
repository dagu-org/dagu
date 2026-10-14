// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import type { StatusTab } from '../components/DAGStatus';

const STATUS_TABS: readonly StatusTab[] = [
  'status',
  'timeline',
  'outputs',
  'artifacts',
  'agent',
  'chat',
  'tasks',
  'spec',
  'approval',
  'human-tasks',
];

/** Returns value as a run tab, or undefined when it names none. */
export function parseStatusTab(value: string | null): StatusTab | undefined {
  return STATUS_TABS.find((tab) => tab === value);
}
