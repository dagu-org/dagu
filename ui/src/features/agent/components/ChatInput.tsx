import { useState, useCallback, useEffect, KeyboardEvent } from 'react';
import { Send, Square } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { DAGContext } from '../types';
import { DAGPicker } from './DAGPicker';
import { useDagPageContext } from '../hooks/useDagPageContext';

interface ChatInputProps {
  onSend: (message: string, dagContexts?: DAGContext[]) => void;
  onCancel?: () => void;
  isWorking: boolean;
  disabled?: boolean;
  placeholder?: string;
}

export function ChatInput({
  onSend,
  onCancel,
  isWorking,
  disabled,
  placeholder = 'Type a message...',
}: ChatInputProps) {
  const [message, setMessage] = useState('');
  const [isPending, setIsPending] = useState(false);
  const [selectedDags, setSelectedDags] = useState<DAGContext[]>([]);
  const currentPageDag = useDagPageContext();

  const showPauseButton = isPending || isWorking;

  // Reset pending state when server confirms processing or after timeout fallback
  useEffect(() => {
    if (isWorking) {
      setIsPending(false);
      return;
    }
    if (isPending) {
      // Use longer timeout to ensure SSE has time to confirm working state
      const timer = setTimeout(() => setIsPending(false), 3000);
      return () => clearTimeout(timer);
    }
  }, [isWorking, isPending]);

  const handleSend = useCallback(() => {
    const trimmed = message.trim();
    if (!trimmed || showPauseButton || disabled) {
      return;
    }

    setIsPending(true);

    // Build contexts: current page DAG first, then additional selected DAGs (excluding duplicates)
    const additionalDags = selectedDags.filter(
      (dag) => dag.dag_file !== currentPageDag?.dag_file
    );
    const allContexts = currentPageDag
      ? [currentPageDag, ...additionalDags]
      : additionalDags;

    onSend(trimmed, allContexts.length > 0 ? allContexts : undefined);
    setMessage('');
  }, [message, showPauseButton, disabled, onSend, selectedDags, currentPageDag]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend]
  );

  return (
    <div className="p-2 border-t border-border/40 bg-background">
      {/* DAG Picker with chips */}
      <DAGPicker
        selectedDags={selectedDags}
        onChange={setSelectedDags}
        currentPageDag={currentPageDag}
        disabled={disabled || showPauseButton}
      />

      {/* Input row */}
      <div className="flex items-end gap-2">
        <textarea
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
          className={cn(
            'flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm',
            'placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring',
            'min-h-[36px] max-h-[120px]',
            disabled && 'opacity-50 cursor-not-allowed'
          )}
          style={{
            height: 'auto',
            minHeight: '36px',
          }}
          onInput={(e) => {
            const target = e.target as HTMLTextAreaElement;
            target.style.height = 'auto';
            target.style.height = `${Math.min(target.scrollHeight, 120)}px`;
          }}
        />
        {showPauseButton ? (
          <Button
            size="sm"
            variant="destructive"
            onClick={onCancel}
            className="h-9 w-9 p-0"
            title="Stop"
          >
            <Square className="h-4 w-4" />
          </Button>
        ) : (
          <Button
            size="sm"
            onClick={handleSend}
            disabled={!message.trim() || disabled}
            className="h-9 w-9 p-0"
            title="Send"
          >
            <Send className="h-4 w-4" />
          </Button>
        )}
      </div>
    </div>
  );
}
