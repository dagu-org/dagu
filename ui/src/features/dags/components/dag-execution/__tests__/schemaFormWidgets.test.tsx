// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { fireEvent, render, screen } from '@testing-library/react';
import type { ComponentType } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { schemaFormWidgets } from '../schemaFormWidgets';

const TextareaWidget = schemaFormWidgets.TextareaWidget as ComponentType<any>;

describe('schemaFormWidgets', () => {
  it('auto-grows schema textarea widgets on input', () => {
    render(
      <TextareaWidget
        id="root_message"
        htmlName="message"
        schema={{ type: 'string' }}
        value=""
        required={false}
        disabled={false}
        readonly={false}
        autofocus={false}
        options={{ emptyValue: undefined, rows: 1 }}
        onChange={vi.fn()}
        onBlur={vi.fn()}
        onFocus={vi.fn()}
      />
    );

    const textarea = screen.getByRole('textbox');
    Object.defineProperty(textarea, 'scrollHeight', {
      configurable: true,
      value: 180,
    });

    fireEvent.input(textarea, { target: { value: 'line 1\nline 2' } });

    expect(textarea.style.height).toBe('150px');
    expect(textarea.style.overflowY).toBe('auto');
  });
});
