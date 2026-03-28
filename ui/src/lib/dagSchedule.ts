// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { components } from '../api/v1/schema';

type Schedule = components['schemas']['Schedule'];

export function getScheduleLabel(schedule: Schedule): string {
  if (schedule.kind === 'at') {
    return schedule.at ? `At ${schedule.at}` : 'At';
  }

  return schedule.expression || '';
}

export function getScheduleKey(schedule: Schedule, index: number): string {
  const base =
    schedule.kind === 'at'
      ? schedule.at || 'at'
      : schedule.expression || 'cron';

  return `${base}-${index}`;
}

export function parseNextRun(nextRun?: string): Date | null {
  if (!nextRun) {
    return null;
  }

  const parsed = new Date(nextRun);
  if (Number.isNaN(parsed.getTime())) {
    return null;
  }

  return parsed;
}
