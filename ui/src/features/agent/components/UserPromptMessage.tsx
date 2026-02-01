import { useState } from 'react';
import { Check, MessageCircleQuestion, SkipForward } from 'lucide-react';
import { cn } from '@/lib/utils';
import { UserPrompt, UserPromptResponse } from '../types';

interface UserPromptMessageProps {
  prompt: UserPrompt;
  onRespond: (response: UserPromptResponse, displayValue: string) => void;
  isAnswered: boolean;
  answeredValue?: string;
}

export function UserPromptMessage({
  prompt,
  onRespond,
  isAnswered,
  answeredValue,
}: UserPromptMessageProps): React.ReactNode {
  const [selectedOptions, setSelectedOptions] = useState<Set<string>>(new Set());
  const [freeText, setFreeText] = useState('');

  const handleOptionClick = (optionId: string) => {
    if (isAnswered) return;

    const newSelected = new Set(selectedOptions);
    if (prompt.multi_select) {
      if (newSelected.has(optionId)) {
        newSelected.delete(optionId);
      } else {
        newSelected.add(optionId);
      }
    } else {
      if (newSelected.has(optionId)) {
        newSelected.clear();
      } else {
        newSelected.clear();
        newSelected.add(optionId);
      }
    }
    setSelectedOptions(newSelected);
    if (newSelected.size > 0) {
      setFreeText('');
    }
  };

  const handleFreeTextChange = (value: string) => {
    setFreeText(value);
    if (value) {
      setSelectedOptions(new Set());
    }
  };

  const handleSubmit = () => {
    const response: UserPromptResponse = {
      prompt_id: prompt.prompt_id,
    };

    let displayValue = '';
    if (selectedOptions.size > 0) {
      response.selected_option_ids = Array.from(selectedOptions);
      const selectedLabels = prompt.options
        ?.filter(opt => selectedOptions.has(opt.id))
        .map(opt => opt.label) ?? [];
      displayValue = selectedLabels.join(', ');
    }
    if (freeText) {
      response.free_text_response = freeText;
      displayValue = displayValue ? `${displayValue}; ${freeText}` : freeText;
    }

    onRespond(response, displayValue || 'Submitted');
  };

  const handleSkip = () => {
    onRespond({
      prompt_id: prompt.prompt_id,
      cancelled: true,
    }, 'Skipped');
  };

  const canSubmit = selectedOptions.size > 0 || freeText.trim().length > 0;

  if (isAnswered) {
    return (
      <div className="pl-1">
        <div className="rounded-lg border border-green-300 dark:border-green-500/30 bg-green-50 dark:bg-green-500/5 p-2">
          <div className="flex items-start gap-1.5">
            <Check className="h-3 w-3 mt-0.5 flex-shrink-0 text-green-600 dark:text-green-400" />
            <div className="flex-1 min-w-0">
              <p className="text-xs font-medium text-foreground">{prompt.question}</p>
              <p className="text-xs text-muted-foreground mt-0.5">
                Answered: {answeredValue || 'Skipped'}
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="pl-1">
      <div className="rounded-lg border border-orange-300 dark:border-amber-500/40 bg-orange-50 dark:bg-amber-500/10 p-2">
        <div className="flex items-start gap-1.5 mb-2">
          <MessageCircleQuestion className="h-3.5 w-3.5 mt-0.5 flex-shrink-0 text-orange-600 dark:text-orange-400" />
          <p className="text-xs font-medium text-foreground">{prompt.question}</p>
        </div>

        {prompt.options && prompt.options.length > 0 && (
          <div className="flex flex-wrap gap-1 mb-2">
            {prompt.options.map((option) => (
              <button
                key={option.id}
                onClick={() => handleOptionClick(option.id)}
                className={cn(
                  'px-2 py-1 text-xs rounded border transition-colors',
                  selectedOptions.has(option.id)
                    ? 'bg-amber-500 text-white border-amber-500'
                    : 'bg-background border-border hover:border-amber-500/50'
                )}
                title={option.description}
              >
                {option.label}
              </button>
            ))}
          </div>
        )}

        {prompt.allow_free_text && (
          <input
            type="text"
            value={freeText}
            onChange={(e) => handleFreeTextChange(e.target.value)}
            placeholder={prompt.free_text_placeholder || 'Or type your answer...'}
            className="w-full px-2 py-1 text-xs rounded border border-border bg-background focus:outline-none focus:border-amber-500/50 mb-2"
          />
        )}

        <div className="flex gap-1">
          <button
            onClick={handleSubmit}
            disabled={!canSubmit}
            className={cn(
              'px-2 py-1 text-xs rounded transition-colors font-medium',
              canSubmit
                ? 'bg-amber-600 text-white hover:bg-amber-700 dark:bg-amber-500 dark:text-black dark:hover:bg-amber-400'
                : 'bg-muted text-muted-foreground cursor-not-allowed'
            )}
          >
            Submit
          </button>
          <button
            onClick={handleSkip}
            className="px-2 py-1 text-xs rounded border border-border hover:bg-muted transition-colors flex items-center gap-1"
          >
            <SkipForward className="h-3 w-3" />
            Skip
          </button>
        </div>
      </div>
    </div>
  );
}
