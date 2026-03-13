// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import dayjs from './dayjs';

type DAGRunTiming = {
  scheduleTime?: string;
  queuedAt?: string;
};

export function getDAGRunScheduleSortValue(run: DAGRunTiming): number {
  const timestamp = run.scheduleTime || run.queuedAt;
  if (!timestamp) {
    return 0;
  }

  const value = dayjs(timestamp).valueOf();
  return Number.isFinite(value) ? value : 0;
}
