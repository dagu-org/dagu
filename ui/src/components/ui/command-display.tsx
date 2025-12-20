import React from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from './tooltip';
import { Code, Terminal } from 'lucide-react';

interface CommandDisplayProps {
  command: string | undefined;
  args?: string | string[];
  icon?: 'code' | 'terminal';
  maxLength?: number;
  className?: string;
  showFullInTooltip?: boolean;
}

const truncateCommand = (
  text: string,
  maxLength: number
): { truncated: string; isTruncated: boolean } => {
  if (text.length <= maxLength) {
    return { truncated: text, isTruncated: false };
  }

  // Smart truncation: try to break at a space or special character
  const cutoff = maxLength - 3; // Leave room for "..."
  let breakPoint = cutoff;

  // Look for a good break point (space, slash, dash)
  for (let i = cutoff; i > cutoff - 10 && i > 0; i--) {
    if (' /-_=&|'.includes(text[i] || '')) {
      breakPoint = i;
      break;
    }
  }

  return {
    truncated: text.substring(0, breakPoint) + '...',
    isTruncated: true,
  };
};

export const CommandDisplay: React.FC<CommandDisplayProps> = ({
  command,
  args,
  icon = 'code',
  maxLength = 60,
  className = '',
  showFullInTooltip = true,
}) => {
  if (!command) {
    return null;
  }

  const Icon = icon === 'code' ? Code : Terminal;
  const { truncated: truncatedCommand, isTruncated: isCommandTruncated } =
    truncateCommand(command, maxLength);

  // Process args
  const argsString = Array.isArray(args) ? args.join(' ') : args || '';
  const { truncated: truncatedArgs, isTruncated: isArgsTruncated } =
    truncateCommand(argsString, maxLength);

  const needsTooltip =
    showFullInTooltip &&
    (isCommandTruncated || (argsString && isArgsTruncated));

  const commandElement = (
    <div className={`space-y-1 ${className}`}>
      <div className="flex items-center gap-1.5 text-xs font-medium">
        <Icon className="h-4 w-4 text-blue-500 flex-shrink-0" />
        <span className="bg-slate-100 rounded-md px-1.5 py-0.5 text-slate-700 font-mono">
          {truncatedCommand}
        </span>
      </div>

      {argsString && (
        <div className="pl-5 text-xs text-slate-500 font-mono">
          <span className="opacity-60"></span> {truncatedArgs}
        </div>
      )}
    </div>
  );

  if (!needsTooltip) {
    return commandElement;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className="cursor-pointer">{commandElement}</div>
      </TooltipTrigger>
      <TooltipContent className="max-w-[600px]">
        <div className="space-y-2">
          <div className="text-xs font-semibold text-slate-600">
            Full Command:
          </div>
          <pre className="whitespace-pre-wrap break-all text-xs font-mono bg-slate-50 p-2 rounded">
            {command}
            {argsString && (
              <>
                {'\n'}
                <span className="text-slate-500">
                  Arguments:{' '}
                </span>
                {argsString}
              </>
            )}
          </pre>
        </div>
      </TooltipContent>
    </Tooltip>
  );
};
