// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { describe, expect, it } from 'vitest';
import type { components } from '@/api/v1/schema';
import { StepDetails } from '../StepDetails';

describe('StepDetails', () => {
  it('renders log executor messages as a first-class message field', () => {
    const step = {
      name: 'announce',
      executorConfig: {
        type: 'log',
        config: {
          message: 'Deploying ${ENVIRONMENT}',
        },
      },
    } as components['schemas']['Step'];

    render(<StepDetails step={step} />);

    expect(screen.getByText('Executor')).toBeInTheDocument();
    expect(screen.getByText('Message')).toBeInTheDocument();
    expect(
      screen.getByLabelText('Log message: Deploying ${ENVIRONMENT}')
    ).toBeInTheDocument();
  });
});
