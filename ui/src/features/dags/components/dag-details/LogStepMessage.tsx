// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';

type LogStepMessageProps = {
  message: string;
  className?: string;
  compact?: boolean;
};

const singleLineLimit = 72;
const ariaLabelLimit = 120;

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

function truncateAriaLabel(message: string): string {
  const display = formatSingleLine(message).replace(/\s+/g, ' ');
  if (display.length <= ariaLabelLimit) {
    return display;
  }
  return `${display.slice(0, ariaLabelLimit - 3)}...`;
}

export function LogStepMessage({
  message,
  className,
  compact = false,
}: LogStepMessageProps) {
  const isMultiline = message.includes('\n');
  const singleLine = truncateSingleLine(message);
  const ariaLabel = truncateAriaLabel(message);

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
        'min-w-0 rounded-md border border-border bg-muted/40 px-2 py-1.5',
        compact ? 'w-[320px] max-w-full' : 'w-full',
        className
      )}
      aria-label={`Log message: ${ariaLabel}`}
    >
      {messageBody}
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
