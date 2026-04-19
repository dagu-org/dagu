// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import React from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AppBarContext } from '@/contexts/AppBarContext';
import DAGSpecReadOnly from '../DAGSpecReadOnly';

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  post: vi.fn(),
  showError: vi.fn(),
  showToast: vi.fn(),
  useQuery: vi.fn(),
}));

vi.mock('@/hooks/api', () => ({
  useClient: () => ({ POST: mocks.post }),
  useQuery: mocks.useQuery,
}));

vi.mock('@/components/ui/error-modal', () => ({
  useErrorModal: () => ({ showError: mocks.showError }),
}));

vi.mock('@/components/ui/simple-toast', () => ({
  useSimpleToast: () => ({ showToast: mocks.showToast }),
}));

vi.mock('react-router-dom', async () => {
  const actual =
    await vi.importActual<typeof import('react-router-dom')>(
      'react-router-dom'
    );
  return {
    ...actual,
    useNavigate: () => mocks.navigate,
  };
});

vi.mock('../DAGEditorWithDocs', () => ({
  default: ({
    value,
    onChange,
    readOnly,
    headerActions,
  }: {
    value: string;
    onChange?: (value?: string) => void;
    readOnly?: boolean;
    headerActions?: React.ReactNode;
  }) => (
    <div>
      <div>{headerActions}</div>
      <textarea
        aria-label="DAG spec"
        readOnly={readOnly}
        value={value}
        onChange={(event) => onChange?.(event.target.value)}
      />
    </div>
  ),
}));

vi.mock('../../visualization/Graph', () => ({
  default: ({
    steps,
  }: {
    steps?: { step: { name: string }; status: number }[];
  }) => (
    <div data-testid="preview-graph">
      {steps?.map((node) => `${node.step.name}:${node.status}`).join(',')}
    </div>
  ),
}));

const appBarValue = {
  title: 'DAGs',
  setTitle: vi.fn(),
  remoteNodes: ['local'],
  setRemoteNodes: vi.fn(),
  selectedRemoteNode: 'local',
  selectRemoteNode: vi.fn(),
};

const originalSpec = `name: example
steps:
  - name: extract
    command: echo old
`;

const editedSpec = `name: example
steps:
  - name: extract
    command: echo new
`;

const previewResponse = {
  dagName: 'example',
  skippedSteps: ['extract'],
  runnableSteps: ['load'],
  steps: [{ name: 'extract' }, { name: 'load', depends: ['extract'] }],
  ineligibleSteps: [],
  errors: [],
  warnings: ['output variables will be copied'],
};

function renderSpec() {
  return render(
    <AppBarContext.Provider value={appBarValue}>
      <DAGSpecReadOnly dagName="example" dagRunId="run-1" />
    </AppBarContext.Provider>
  );
}

afterEach(() => {
  cleanup();
  mocks.navigate.mockReset();
  mocks.post.mockReset();
  mocks.showError.mockReset();
  mocks.showToast.mockReset();
  mocks.useQuery.mockReset();
});

describe('DAGSpecReadOnly', () => {
  it('previews and confirms before creating an edited retry run', async () => {
    mocks.useQuery.mockReturnValue({
      data: { spec: originalSpec },
      isLoading: false,
      error: undefined,
    });
    mocks.post
      .mockResolvedValueOnce({ data: previewResponse })
      .mockResolvedValueOnce({ data: { dagRunId: 'run-2' } });

    renderSpec();

    const editor = screen.getByLabelText('DAG spec');
    await waitFor(() => expect(editor).toHaveValue(originalSpec));
    fireEvent.change(editor, { target: { value: editedSpec } });

    await userEvent.click(
      await screen.findByRole('button', { name: /retry as a new run/i })
    );

    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.post).toHaveBeenLastCalledWith(
      '/dag-runs/{name}/{dagRunId}/edit-retry/preview',
      expect.objectContaining({
        body: {
          spec: editedSpec,
          dagName: 'example',
          persistSpec: false,
        },
      })
    );
    expect(await screen.findByText('Step review')).toBeInTheDocument();
    expect(screen.getByTestId('preview-graph')).toHaveTextContent('extract:4');
    expect(screen.getByTestId('preview-graph')).toHaveTextContent('load:0');
    expect(screen.getAllByText('Reuse previous output').length).toBeGreaterThan(
      0
    );
    expect(screen.getAllByText('extract').length).toBeGreaterThan(0);
    expect(screen.getByText('load')).toBeInTheDocument();

    await userEvent.click(
      screen.getByRole('button', { name: /create new run/i })
    );

    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(2));
    expect(mocks.post).toHaveBeenLastCalledWith(
      '/dag-runs/{name}/{dagRunId}/edit-retry',
      expect.objectContaining({
        body: {
          spec: editedSpec,
          dagName: 'example',
          persistSpec: false,
          skipSteps: ['extract'],
        },
      })
    );
    expect(mocks.navigate).toHaveBeenCalledWith('/dag-runs/example/run-2');
  });

  it('allows eligible reusable steps to run again instead', async () => {
    mocks.useQuery.mockReturnValue({
      data: { spec: originalSpec },
      isLoading: false,
      error: undefined,
    });
    mocks.post
      .mockResolvedValueOnce({ data: previewResponse })
      .mockResolvedValueOnce({ data: { dagRunId: 'run-2' } });

    renderSpec();

    const editor = screen.getByLabelText('DAG spec');
    await waitFor(() => expect(editor).toHaveValue(originalSpec));
    fireEvent.change(editor, { target: { value: editedSpec } });

    await userEvent.click(
      await screen.findByRole('button', { name: /retry as a new run/i })
    );
    await userEvent.click(
      await screen.findByLabelText('Reuse previous output for extract')
    );
    await userEvent.click(
      screen.getByRole('button', { name: /create new run/i })
    );

    await waitFor(() => expect(mocks.post).toHaveBeenCalledTimes(2));
    expect(mocks.post).toHaveBeenLastCalledWith(
      '/dag-runs/{name}/{dagRunId}/edit-retry',
      expect.objectContaining({
        body: {
          spec: editedSpec,
          dagName: 'example',
          persistSpec: false,
          skipSteps: [],
        },
      })
    );
  });

  it('does not submit when the preview contains errors', async () => {
    mocks.useQuery.mockReturnValue({
      data: { spec: originalSpec },
      isLoading: false,
      error: undefined,
    });
    mocks.post.mockResolvedValueOnce({
      data: {
        ...previewResponse,
        errors: ['invalid edited DAG'],
      },
    });

    renderSpec();

    const editor = screen.getByLabelText('DAG spec');
    await waitFor(() => expect(editor).toHaveValue(originalSpec));
    fireEvent.change(editor, { target: { value: editedSpec } });

    await userEvent.click(
      await screen.findByRole('button', { name: /retry as a new run/i })
    );

    const submitButton = await screen.findByRole('button', {
      name: /create new run/i,
    });
    expect(submitButton).toBeDisabled();

    await userEvent.click(submitButton);
    expect(mocks.post).toHaveBeenCalledTimes(1);
    expect(mocks.navigate).not.toHaveBeenCalled();
  });
});
