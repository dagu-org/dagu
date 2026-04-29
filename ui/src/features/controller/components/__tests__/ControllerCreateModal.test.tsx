// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/hooks/api', () => ({
  useClient: vi.fn(),
  useQuery: vi.fn(),
}));

import { useClient, useQuery } from '@/hooks/api';
import { ControllerCreateModal } from '@/features/controller/components/ControllerCreateModal';

const useClientMock = useClient as unknown as {
  mockReturnValue: (value: unknown) => void;
};
const useQueryMock = useQuery as unknown as {
  mockReturnValue: (value: unknown) => void;
};

function renderModal() {
  return render(
    <ControllerCreateModal
      open
      onClose={vi.fn()}
      selectedWorkspace=""
      remoteNode=""
      onCreated={vi.fn()}
    />
  );
}

beforeEach(() => {
  useClientMock.mockReturnValue({
    PUT: vi.fn(),
  });
  useQueryMock.mockReturnValue({
    data: undefined,
    isLoading: false,
  });
  vi.stubGlobal(
    'ResizeObserver',
    class ResizeObserver {
      observe() {}
      unobserve() {}
      disconnect() {}
    }
  );
});

afterEach(() => {
  cleanup();
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe('ControllerCreateModal', () => {
  it('marks only currently required fields', () => {
    renderModal();

    expect(
      screen.getByText(
        'Used as the controller ID. Letters, numbers, and underscores only.'
      )
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Manual controllers start only when someone provides the instruction at run time.'
      )
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Optional. Seed the controller with workflows it should start from.'
      )
    ).toBeInTheDocument();
    expect(screen.getByLabelText(/Name/)).toHaveAttribute('aria-required', 'true');
    expect(screen.getAllByText('*').length).toBeGreaterThanOrEqual(1);
  });

  it('reveals cron-only required fields when cron trigger is selected', () => {
    renderModal();

    fireEvent.click(screen.getByRole('combobox'));
    fireEvent.click(screen.getByText('Cron'));

    expect(screen.getByText('Cron Schedules')).toBeInTheDocument();
    expect(screen.getByText('Trigger Prompt')).toBeInTheDocument();
    expect(
      screen.getByText(
        'Cron controllers start from the schedules below and need a stored prompt for each cycle.'
      )
    ).toBeInTheDocument();
    expect(screen.getAllByText('*').length).toBeGreaterThanOrEqual(3);
  });
});
