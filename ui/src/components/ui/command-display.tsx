import React from 'react';
import { Tooltip, TooltipContent, TooltipTrigger } from './tooltip';
import { Code, Terminal } from 'lucide-react';

interface CommandDisplayProps {
  command: string | string[] | undefined;
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
  if (!command || (Array.isArray(command) && command.length === 0)) {
    return null;
  }

  const Icon = icon === 'code' ? Code : Terminal;

  // Normalize command to array
  const commands = Array.isArray(command) ? command : [command];
  const fullCommandString = commands.join(' && ');

  const { truncated: truncatedCommand, isTruncated: isCommandTruncated } =
    truncateCommand(fullCommandString, maxLength);

  // Process args
  const argsString = Array.isArray(args) ? args.join(' ') : args || '';
  const { truncated: truncatedArgs, isTruncated: isArgsTruncated } =
    truncateCommand(argsString, maxLength);

  const needsTooltip =
    showFullInTooltip &&
    (isCommandTruncated || (argsString && isArgsTruncated) || commands.length > 1);

  const commandElement = (
    <div className={`space-y-1 ${className}`}>
      <div className="flex items-center gap-1.5 text-xs font-medium">
        <Icon className="h-4 w-4 text-primary flex-shrink-0" />
        <span className="bg-muted rounded-md px-1.5 py-0.5 text-foreground/90 font-mono">
          {truncatedCommand}
        </span>
        {commands.length > 1 && (
          <span className="text-muted-foreground text-[10px]">
            ({commands.length} commands)
          </span>
        )}
      </div>

      {argsString && (
        <div className="pl-5 text-xs text-muted-foreground font-mono">
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
          <div className="text-xs font-semibold text-muted-foreground">
            {commands.length > 1 ? 'Commands:' : 'Full Command:'}
          </div>
          <pre className="whitespace-pre-wrap break-all text-xs font-mono bg-muted p-2 rounded">
            {commands.map((cmd, i) => (
              <React.Fragment key={i}>
                {i > 0 && '\n'}
                {commands.length > 1 && <span className="text-muted-foreground">{i + 1}. </span>}
                {cmd}
              </React.Fragment>
            ))}
            {argsString && (
              <>
                {'\n'}
                <span className="text-muted-foreground">
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
