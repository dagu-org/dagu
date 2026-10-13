// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

export type TimelineAxisScale = 'minute' | 'hour' | 'day' | 'week' | 'month';

/** Room a minor axis label such as "HH:mm" needs to stay legible. */
const MIN_LABEL_SPACING_PX = 40;

const SCALE_DURATION_MS: Record<TimelineAxisScale, number> = {
  minute: 60_000,
  hour: 3_600_000,
  day: 86_400_000,
  week: 604_800_000,
  month: 2_592_000_000,
};

// Steps that divide their parent unit evenly, so labels land on round times.
const STEP_CHOICES: Record<TimelineAxisScale, readonly number[]> = {
  minute: [1, 2, 5, 10, 15, 30],
  hour: [1, 2, 3, 4, 6, 12],
  day: [1, 2, 7, 14],
  week: [1, 2, 4],
  month: [1, 2, 3, 6],
};

/**
 * Returns the smallest step at least `step` that leaves each axis label
 * MIN_LABEL_SPACING_PX of room when `rangeMs` spans `widthPx`. Labels at
 * the largest available step are returned when none fits.
 */
export function legibleAxisStep(
  scale: TimelineAxisScale,
  step: number,
  rangeMs: number,
  widthPx: number
): number {
  if (widthPx <= 0 || rangeMs <= 0) {
    return step;
  }
  const fits = (candidate: number) =>
    widthPx / (rangeMs / (SCALE_DURATION_MS[scale] * candidate)) >=
    MIN_LABEL_SPACING_PX;
  if (fits(step)) {
    return step;
  }
  const choices = STEP_CHOICES[scale].filter((candidate) => candidate > step);
  return choices.find(fits) ?? choices[choices.length - 1] ?? step;
}
