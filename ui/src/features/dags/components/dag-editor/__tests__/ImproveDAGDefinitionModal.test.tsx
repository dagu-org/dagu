// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';

import ImproveDAGDefinitionModal from '../ImproveDAGDefinitionModal';

describe('ImproveDAGDefinitionModal', () => {
  it('renders steward-focused copy', () => {
    render(
      <ImproveDAGDefinitionModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        dagFile="example.yaml"
      />
    );

    expect(screen.getByText('Ask Steward to Improve DAG')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Start a fresh steward session with this DAG reference, the latest run details, and your request.'
      )
    ).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Ask Steward' })
    ).toBeInTheDocument();
  });

  it('shows the new empty-submit validation copy', async () => {
    render(
      <ImproveDAGDefinitionModal
        isOpen={true}
        onClose={vi.fn()}
        onSubmit={vi.fn()}
        dagFile="example.yaml"
      />
    );

    fireEvent.click(screen.getByRole('button', { name: 'Ask Steward' }));

    expect(
      await screen.findByText(
        'Describe what should be improved before asking Steward.'
      )
    ).toBeInTheDocument();
  });
});
