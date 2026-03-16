// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import { useState } from 'react';
import { AlertTriangle, Check, FolderOpen, Loader2, X } from 'lucide-react';
import { cn } from '@/lib/utils';
import { UserPrompt, UserPromptResponse } from '../types';

interface CommandApprovalMessageProps {
  prompt: UserPrompt;
  onRespond: (response: UserPromptResponse, displayValue: string) => Promise<void>;
  isAnswered: boolean;
  answeredValue?: string;
}

export function CommandApprovalMessage({
  prompt,
  onRespond,
  isAnswered,
  answeredValue,
}: CommandApprovalMessageProps): React.ReactNode {
  const [pendingAction, setPendingAction] = useState<'approve' | 'reject' | null>(null);

  const handleApprove = async () => {
    if (pendingAction || isAnswered) return;
    setPendingAction('approve');
    try {
      await onRespond(
        {
          prompt_id: prompt.prompt_id,
          selected_option_ids: ['approve'],
        },
        'Approved'
      );
    } finally {
      setPendingAction(null);
    }
  };

  const handleReject = async () => {
    if (pendingAction || isAnswered) return;
    setPendingAction('reject');
    try {
      await onRespond(
        {
          prompt_id: prompt.prompt_id,
          selected_option_ids: ['reject'],
        },
        'Rejected'
      );
    } finally {
      setPendingAction(null);
    }
  };

  const wasApproved = answeredValue === 'Approved';

  if (isAnswered) {
    return (
      <div className="pl-1">
        <div
          className={cn(
            'rounded border p-2',
            wasApproved
              ? 'border-green-600/30 bg-green-50 dark:bg-green-500/5'
              : 'border-red-600/30 bg-red-50 dark:bg-red-500/5'
          )}
        >
          <div className="flex items-start gap-1.5">
            {wasApproved ? (
              <Check className="h-3 w-3 mt-0.5 flex-shrink-0 text-green-600" />
            ) : (
              <X className="h-3 w-3 mt-0.5 flex-shrink-0 text-red-600" />
            )}
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-foreground">
                Command {wasApproved ? 'approved' : 'rejected'}
              </p>
              <code className="text-xs text-muted-foreground mt-0.5 font-mono block truncate">
                {prompt.command}
              </code>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="pl-1">
      <div className="rounded border border-amber-500 bg-background p-2">
        <div className="flex items-start gap-1.5 mb-2">
          <AlertTriangle className="h-3.5 w-3.5 mt-0.5 flex-shrink-0 text-amber-500" />
          <p className="text-xs font-medium text-foreground">
            Command requires approval
          </p>
        </div>

        <div className="bg-muted rounded p-2 mb-2">
          <code className="text-xs font-mono break-all whitespace-pre-wrap">
            {prompt.command}
          </code>
        </div>

        {prompt.working_dir && (
          <div className="flex items-center gap-1 text-xs text-muted-foreground mb-2">
            <FolderOpen className="h-3 w-3" />
            <span className="font-mono truncate">{prompt.working_dir}</span>
          </div>
        )}

        <div className="flex gap-1.5">
          <button
            onClick={handleApprove}
            disabled={pendingAction !== null}
            className={cn(
              'px-2 py-1 text-xs rounded font-medium flex items-center gap-1',
              pendingAction ? 'bg-green-600/50 text-white cursor-not-allowed' : 'bg-green-600 text-white hover:bg-green-700'
            )}
          >
            {pendingAction === 'approve' && <Loader2 className="h-3 w-3 animate-spin" />}
            {pendingAction === 'approve' ? 'Sending...' : 'Approve'}
          </button>
          <button
            onClick={handleReject}
            disabled={pendingAction !== null}
            className={cn(
              'px-2 py-1 text-xs rounded font-medium border border-border flex items-center gap-1',
              pendingAction ? 'opacity-50 cursor-not-allowed' : 'hover:bg-muted'
            )}
          >
            {pendingAction === 'reject' && <Loader2 className="h-3 w-3 animate-spin" />}
            {pendingAction === 'reject' ? 'Sending...' : 'Reject'}
          </button>
        </div>
      </div>
    </div>
  );
}
