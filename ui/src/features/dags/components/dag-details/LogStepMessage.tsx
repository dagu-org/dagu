// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import { MessageSquare } from 'lucide-react';

type LogStepMessageProps = {
  message: string;
  className?: string;
  compact?: boolean;
};

const singleLineLimit = 72;

function formatSingleLine(message: string): string {
  return message.length === 0 ? '(empty message)' : message;
}

function truncateSingleLine(message: string): {
  text: string;
  isTruncated: boolean;
} {
  const display = formatSingleLine(message);
  if (display.length <= singleLineLimit) {
    return { text: display, isTruncated: false };
  }
  return {
    text: `${display.slice(0, singleLineLimit - 3)}...`,
    isTruncated: true,
  };
}

export function LogStepMessage({
  message,
  className,
  compact = false,
}: LogStepMessageProps) {
  const isMultiline = message.includes('\n');
  const singleLine = truncateSingleLine(message);

  const messageBody = isMultiline ? (
    <pre className="max-h-28 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-5 text-foreground">
      {message || '(empty message)'}
    </pre>
  ) : (
    <span
      className="block truncate font-mono text-xs leading-5 text-foreground"
      title={singleLine.isTruncated ? message : undefined}
    >
      {singleLine.text}
    </span>
  );

  const content = (
    <div
      className={cn(
        'inline-flex min-w-0 max-w-full items-start gap-1.5 rounded-md border border-border bg-muted/40 px-2 py-1.5',
        compact ? 'max-w-[260px]' : 'w-full',
        className
      )}
      aria-label={`Log message: ${formatSingleLine(message)}`}
    >
      <MessageSquare className="mt-0.5 h-3.5 w-3.5 flex-shrink-0 text-primary" />
      <div className="min-w-0 flex-1">{messageBody}</div>
    </div>
  );

  if (!isMultiline && !singleLine.isTruncated) {
    return content;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent className="max-w-[520px]">
        <pre className="whitespace-pre-wrap break-words font-mono text-xs">
          {message || '(empty message)'}
        </pre>
      </TooltipContent>
    </Tooltip>
  );
}
