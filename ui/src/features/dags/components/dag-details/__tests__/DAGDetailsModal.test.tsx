// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import React from 'react';
import { describe, expect, it, vi } from 'vitest';
import DAGDetailsModal from '../DAGDetailsModal';

const mockSidePanel = vi.fn((props?: unknown) => {
  void props;
  return <div>shared dag modal</div>;
});

vi.mock('../DAGDetailsSidePanel', () => ({
  default: (props: unknown) => mockSidePanel(props),
}));

describe('DAGDetailsModal', () => {
  it('passes the standard DAG modal configuration to the shared side panel', () => {
    render(
      <DAGDetailsModal fileName="example" isOpen={true} onClose={vi.fn()} />
    );

    expect(screen.getByText('shared dag modal')).toBeInTheDocument();
    expect(mockSidePanel).toHaveBeenCalledWith(
      expect.objectContaining({
        fileName: 'example',
        isOpen: true,
        initialTab: 'status',
        onClose: expect.any(Function),
        toolbarHint: expect.any(Object),
      })
    );
  });
});
