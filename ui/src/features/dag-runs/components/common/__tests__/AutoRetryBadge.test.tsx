import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import AutoRetryBadge from '../AutoRetryBadge';

describe('AutoRetryBadge', () => {
  it('hides when no retry limit is configured', () => {
    const { container } = render(<AutoRetryBadge count={0} limit={0} />);
    expect(container).toBeEmptyDOMElement();
  });

  it('shows the configured retry ratio before exhaustion', () => {
    render(<AutoRetryBadge count={1} limit={3} />);
    expect(screen.getByText('1/3 auto retries')).toBeInTheDocument();
  });

  it('shows exhaustion once the retry limit is reached', () => {
    render(<AutoRetryBadge count={3} limit={3} />);
    expect(screen.getByText('auto retries exhausted')).toBeInTheDocument();
  });
});
