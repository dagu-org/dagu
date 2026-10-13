// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { legibleTimeAxis } from '../timelineAxis';

const HOUR_MS = 60 * 60 * 1000;
const DAY_MS = 24 * HOUR_MS;

describe('legibleTimeAxis', () => {
  it('keeps hourly labels when a day fits the width', () => {
    expect(legibleTimeAxis({ scale: 'hour', step: 1 }, DAY_MS, 1150)).toEqual({
      scale: 'hour',
      step: 1,
    });
  });

  it('widens the step so labels do not overlap on a phone', () => {
    expect(legibleTimeAxis({ scale: 'hour', step: 1 }, DAY_MS, 340)).toEqual({
      scale: 'hour',
      step: 3,
    });
  });

  it('moves to a coarser scale when no step of the current one fits', () => {
    expect(
      legibleTimeAxis({ scale: 'hour', step: 4 }, 6 * DAY_MS, 340)
    ).toEqual({ scale: 'day', step: 1 });
    expect(
      legibleTimeAxis({ scale: 'minute', step: 15 }, 5 * HOUR_MS, 200)
    ).toEqual({ scale: 'hour', step: 1 });
  });

  // vis-timeline inserts month-start ticks between weekly ticks, so a
  // crowded weekly axis moves to months rather than to multi-week steps.
  it('skips multi-week steps when weekly labels are crowded', () => {
    expect(
      legibleTimeAxis({ scale: 'week', step: 1 }, 300 * DAY_MS, 340)
    ).toEqual({ scale: 'month', step: 2 });
  });

  it('leaves the axis alone before the width is known', () => {
    expect(legibleTimeAxis({ scale: 'minute', step: 5 }, DAY_MS, 0)).toEqual({
      scale: 'minute',
      step: 5,
    });
  });
});
