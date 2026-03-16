// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { Status } from '@/api/v1/schema';
import AutoRetryBadge from '../AutoRetryBadge';

describe('AutoRetryBadge', () => {
  it('hides when no retry limit is configured', () => {
    const { container } = render(
      <AutoRetryBadge status={Status.Failed} count={0} limit={0} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('hides when status is not failed', () => {
    const { container } = render(
      <AutoRetryBadge status={Status.Success} count={1} limit={3} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('hides when status is running', () => {
    const { container } = render(
      <AutoRetryBadge status={Status.Running} count={1} limit={3} />
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the configured retry ratio when failed', () => {
    render(
      <AutoRetryBadge status={Status.Failed} count={1} limit={3} />
    );
    expect(screen.getByText('1/3 auto retries')).toBeInTheDocument();
  });

  it('shows exhaustion once the retry limit is reached', () => {
    render(
      <AutoRetryBadge status={Status.Failed} count={3} limit={3} />
    );
    expect(screen.getByText('auto retries exhausted')).toBeInTheDocument();
  });
});
