// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

export type TimelineAxisScale =
  | 'minute'
  | 'hour'
  | 'day'
  | 'week'
  | 'month'
  | 'year';

export type TimelineAxis = { scale: TimelineAxisScale; step: number };

/** Room a minor axis label such as "HH:mm" needs to stay legible. */
const MIN_LABEL_SPACING_PX = 40;

const SCALE_DURATION_MS: Record<TimelineAxisScale, number> = {
  minute: 60_000,
  hour: 3_600_000,
  day: 86_400_000,
  week: 604_800_000,
  month: 2_592_000_000,
  year: 31_536_000_000,
};

// Fallback axes from finest to coarsest. Each step divides its parent unit,
// so labels land on round times. Multi-day and multi-week steps are left out
// because vis-timeline snaps them to the first of the month or inserts
// month-start ticks, which can put two labels a day apart.
const FALLBACK_AXES: readonly TimelineAxis[] = [
  ...[1, 2, 5, 10, 15, 30].map((step) => ({ scale: 'minute' as const, step })),
  ...[1, 2, 3, 4, 6, 12].map((step) => ({ scale: 'hour' as const, step })),
  { scale: 'day', step: 1 },
  ...[1, 2, 3, 6].map((step) => ({ scale: 'month' as const, step })),
  ...[1, 2, 5, 10].map((step) => ({ scale: 'year' as const, step })),
];

function axisDuration(axis: TimelineAxis): number {
  return SCALE_DURATION_MS[axis.scale] * axis.step;
}

/**
 * Returns `axis` when each of its labels gets MIN_LABEL_SPACING_PX of room
 * when `rangeMs` spans `widthPx`. Otherwise returns the finest coarser axis
 * that does, switching scale if needed, or the coarsest axis when none fits.
 */
export function legibleTimeAxis(
  axis: TimelineAxis,
  rangeMs: number,
  widthPx: number
): TimelineAxis {
  if (widthPx <= 0 || rangeMs <= 0) {
    return axis;
  }
  const fits = (candidate: TimelineAxis) =>
    (widthPx * axisDuration(candidate)) / rangeMs >= MIN_LABEL_SPACING_PX;
  if (fits(axis)) {
    return axis;
  }
  const coarser = FALLBACK_AXES.filter(
    (candidate) => axisDuration(candidate) > axisDuration(axis)
  );
  return coarser.find(fits) ?? coarser[coarser.length - 1] ?? axis;
}
