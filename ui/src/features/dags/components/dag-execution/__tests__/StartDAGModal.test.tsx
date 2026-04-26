// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import React from 'react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import StartDAGModal from '../StartDAGModal';

const renderedFormProps = vi.fn();

vi.mock('@rjsf/shadcn', async () => {
  const React = await import('react');

  return {
    default: React.forwardRef(function MockSchemaForm(
      props: {
        formData?: Record<string, unknown>;
        uiSchema?: Record<string, unknown>;
        onChange?: (event: { formData: Record<string, unknown> }) => void;
      },
      ref: any
    ) {
      renderedFormProps(props);
      React.useImperativeHandle(ref, () => ({
        validateForm: () => true,
      }));

      return (
        <div data-testid="schema-form">
          <button
            type="button"
            onClick={() =>
              props.onChange?.({
                formData: { region: 'us-west-2', count: 5 },
              })
            }
          >
            Update schema form
          </button>
        </div>
      );
    }),
  };
});

vi.mock('@rjsf/validator-ajv8', () => ({
  default: {},
}));

beforeEach(() => {
  vi.clearAllMocks();
});

describe('StartDAGModal', () => {
  it('renders the schema-backed form path and submits a JSON object payload', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <StartDAGModal
        visible={true}
        dismissModal={vi.fn()}
        onSubmit={onSubmit}
        dag={
          {
            name: 'schema-dag',
            paramSchema: {
              type: 'object',
              properties: {
                region: {
                  type: 'string',
                  enum: ['us-east-1', 'us-west-2'],
                },
                count: {
                  type: 'integer',
                },
              },
            },
            defaultParams: 'region="us-east-1" count="3"',
          } as never
        }
      />
    );

    expect(screen.getByTestId('schema-form')).toBeInTheDocument();
    expect(renderedFormProps).toHaveBeenCalledWith(
      expect.objectContaining({
        formData: { region: 'us-east-1', count: 3 },
        uiSchema: expect.objectContaining({
          region: expect.objectContaining({ 'ui:widget': 'radio' }),
        }),
        templates: expect.objectContaining({
          BaseInputTemplate: expect.any(Function),
        }),
        widgets: expect.objectContaining({
          RadioWidget: expect.any(Function),
          CheckboxWidget: expect.any(Function),
          SelectWidget: expect.any(Function),
          TextareaWidget: expect.any(Function),
        }),
      })
    );

    fireEvent.click(screen.getByRole('button', { name: 'Update schema form' }));
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        '{"region":"us-west-2","count":5}',
        undefined,
        true
      )
    );
  });

  it('falls back to typed param fields when paramSchema is absent', () => {
    render(
      <StartDAGModal
        visible={true}
        dismissModal={vi.fn()}
        onSubmit={vi.fn()}
        dag={
          {
            name: 'typed-dag',
            paramDefs: [
              {
                name: 'region',
                type: 'string',
                required: true,
              },
            ],
          } as never
        }
      />
    );

    expect(screen.queryByTestId('schema-form')).not.toBeInTheDocument();
    expect(screen.getByLabelText(/region/i)).toBeInTheDocument();
  });

  it('submits typed param fields as a JSON array payload', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <StartDAGModal
        visible={true}
        dismissModal={vi.fn()}
        onSubmit={onSubmit}
        dag={
          {
            name: 'typed-dag',
            paramDefs: [
              {
                name: 'region',
                type: 'string',
                required: true,
              },
              {
                name: 'count',
                type: 'integer',
                required: true,
              },
            ],
          } as never
        }
      />
    );

    fireEvent.change(screen.getByLabelText(/region/i), {
      target: { value: 'us-west-2' },
    });
    fireEvent.change(screen.getByLabelText(/count/i), {
      target: { value: '5' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Start' }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        '[{"region":"us-west-2"},{"count":"5"}]',
        undefined,
        true
      )
    );
  });
});
