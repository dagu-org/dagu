// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { render, screen } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import { AgentChatModalHeader } from '../AgentChatModalHeader';
import { ChatMessages } from '../ChatMessages';

const updatePreferenceMock = vi.fn();

vi.mock('@/contexts/UserPreference', () => ({
  useUserPreferences: () => ({
    preferences: { safeMode: true },
    updatePreference: updatePreferenceMock,
  }),
}));

describe('Steward UI copy', () => {
  beforeEach(() => {
    updatePreferenceMock.mockReset();
  });

  it('renders the steward title in the floating chat header', () => {
    render(
      <AgentChatModalHeader
        sessionId={null}
        totalCost={0}
        isSidebarOpen={true}
        onToggleSidebar={vi.fn()}
        onClearSession={vi.fn()}
      />
    );

    expect(screen.getByText('Steward')).toBeInTheDocument();
  });

  it('renders the steward-focused empty state guidance', () => {
    render(
      <ChatMessages
        messages={[]}
        pendingUserMessage={null}
        isWorking={false}
      />
    );

    expect(
      screen.getByText('Ask Steward to improve, repair, or explain workflows')
    ).toBeInTheDocument();
  });
});
