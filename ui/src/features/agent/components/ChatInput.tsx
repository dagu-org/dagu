import * as React from 'react';
import { useState, useCallback, KeyboardEvent } from 'react';
import { Send, Square } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';

interface ChatInputProps {
  onSend: (message: string) => void;
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

  const handleSend = useCallback(() => {
    const trimmed = message.trim();
    if (trimmed && !isWorking && !disabled) {
      onSend(trimmed);
      setMessage('');
    }
  }, [message, isWorking, disabled, onSend]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend]
  );

  const handleCancel = useCallback(() => {
    if (onCancel) {
      onCancel();
    }
  }, [onCancel]);

  return (
    <div className="flex items-end gap-2 p-2 border-t border-border/40 bg-background">
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
      {isWorking ? (
        <Button
          size="sm"
          variant="destructive"
          onClick={handleCancel}
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
  );
}
