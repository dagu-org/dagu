// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, expect, it } from 'vitest';
import { legibleAxisStep } from '../timelineAxis';

const DAY_MS = 24 * 60 * 60 * 1000;

describe('legibleAxisStep', () => {
  it('keeps hourly labels when a day fits the width', () => {
    expect(legibleAxisStep('hour', 1, DAY_MS, 1150)).toBe(1);
  });

  it('widens the step so labels do not overlap on a phone', () => {
    expect(legibleAxisStep('hour', 1, DAY_MS, 340)).toBe(3);
  });

  it('uses the widest step when even that is crowded', () => {
    expect(legibleAxisStep('hour', 1, DAY_MS, 60)).toBe(12);
  });

  it('leaves the step alone before the width is known', () => {
    expect(legibleAxisStep('minute', 5, DAY_MS, 0)).toBe(5);
  });
});
