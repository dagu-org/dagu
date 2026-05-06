// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import type { components } from '@/api/v1/schema';
import { describe, expect, it } from 'vitest';
import { getLogStepMessage } from '../executor-utils';

describe('getLogStepMessage', () => {
  it('returns the configured log message for log steps', () => {
    const step = {
      name: 'announce',
      executorConfig: {
        type: 'log',
        config: {
          message: 'Deploying ${ENVIRONMENT}',
        },
      },
    } as components['schemas']['Step'];

    expect(getLogStepMessage(step)).toBe('Deploying ${ENVIRONMENT}');
  });

  it('ignores non-log steps', () => {
    const step = {
      name: 'run',
      executorConfig: {
        type: 'command',
        config: {},
      },
    } as components['schemas']['Step'];

    expect(getLogStepMessage(step)).toBeNull();
  });
});
