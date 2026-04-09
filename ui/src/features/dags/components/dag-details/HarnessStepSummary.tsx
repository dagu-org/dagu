import { Badge } from '@/components/ui/badge';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { getHarnessStepSummary, type HarnessAttemptSummary } from '@/lib/harness-step';
import { cn } from '@/lib/utils';
import { Bot } from 'lucide-react';
import { components } from '../../../../api/v1/schema';

type Props = {
  step: components['schemas']['Step'];
  className?: string;
};

const visibleOptionCount = 3;
const promptTooltipThreshold = 120;

function HarnessStepSummary({ step, className }: Props) {
  const summary = getHarnessStepSummary(step);

  if (!summary) {
    return null;
  }

  const promptContent = summary.prompt ? (
    <div className="rounded-md border border-border/70 bg-background/70 px-2 py-1.5">
      <div className="mb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
        Prompt
      </div>
      <div className="line-clamp-3 whitespace-pre-wrap break-words text-xs leading-relaxed text-foreground/90">
        {summary.prompt}
      </div>
    </div>
  ) : null;

  return (
    <div
      className={cn(
        'rounded-lg border border-info/20 bg-info/5 p-2 space-y-2',
        className
      )}
    >
      <div className="flex flex-wrap items-center gap-1.5">
        <Badge
          variant="info"
          className="h-6 gap-1.5 px-2.5 normal-case tracking-normal"
        >
          <Bot className="h-3.5 w-3.5" />
          Harness
        </Badge>
      </div>

      {summary.attempts.map((attempt, index) => (
        <HarnessAttemptRow
          key={`${attempt.label}-${attempt.provider || index}`}
          attempt={attempt}
          isPrimary={index === 0}
        />
      ))}

      {summary.prompt &&
      summary.prompt.length > promptTooltipThreshold ? (
        <Tooltip>
          <TooltipTrigger asChild>
            <div className="cursor-help">{promptContent}</div>
          </TooltipTrigger>
          <TooltipContent className="max-w-[640px]">
            <div className="whitespace-pre-wrap break-words text-xs leading-relaxed">
              {summary.prompt}
            </div>
          </TooltipContent>
        </Tooltip>
      ) : (
        promptContent
      )}
    </div>
  );
}

function HarnessAttemptRow({
  attempt,
  isPrimary,
}: {
  attempt: HarnessAttemptSummary;
  isPrimary: boolean;
}) {
  const visibleOptions = attempt.options.slice(0, visibleOptionCount);
  const hiddenOptions = attempt.options.slice(visibleOptionCount);

  return (
    <div className="flex flex-wrap items-center gap-1.5">
      <Badge
        variant={isPrimary ? 'primary' : 'outline'}
        className="h-5 px-2 normal-case tracking-normal"
      >
        {attempt.label}
      </Badge>
      {attempt.provider && (
        <Badge
          variant={isPrimary ? 'info' : 'secondary'}
          className="h-5 px-2 font-mono normal-case tracking-normal"
        >
          {attempt.provider}
        </Badge>
      )}
      {visibleOptions.map((option) => (
        <Badge
          key={`${attempt.label}-${option}`}
          variant="outline"
          className="h-5 px-2 font-mono normal-case tracking-normal"
        >
          {option}
        </Badge>
      ))}
      {hiddenOptions.length > 0 && (
        <Tooltip>
          <TooltipTrigger asChild>
            <Badge
              variant="outline"
              className="h-5 cursor-help px-2 normal-case tracking-normal"
            >
              +{hiddenOptions.length} more
            </Badge>
          </TooltipTrigger>
          <TooltipContent className="max-w-[480px]">
            <div className="flex flex-wrap gap-1.5">
              {hiddenOptions.map((option) => (
                <Badge
                  key={`${attempt.label}-hidden-${option}`}
                  variant="outline"
                  className="h-5 px-2 font-mono normal-case tracking-normal"
                >
                  {option}
                </Badge>
              ))}
            </div>
          </TooltipContent>
        </Tooltip>
      )}
    </div>
  );
}

export default HarnessStepSummary;
