import { useState, useCallback, useEffect, useRef, useContext, KeyboardEvent } from 'react';
import { Send, Square } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import { cn } from '@/lib/utils';
import { useConfig } from '@/contexts/ConfigContext';
import { AppBarContext } from '@/contexts/AppBarContext';
import { getAuthHeaders } from '@/lib/authHeaders';
import { DAGContext } from '../types';
import { DAGPicker } from './DAGPicker';
import { useDagPageContext } from '../hooks/useDagPageContext';

interface ModelOption {
  id: string;
  name: string;
}

interface ChatInputProps {
  onSend: (message: string, dagContexts?: DAGContext[], model?: string) => void;
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
  const config = useConfig();
  const appBarContext = useContext(AppBarContext);
  const [message, setMessage] = useState('');
  const [isPending, setIsPending] = useState(false);
  const [selectedDags, setSelectedDags] = useState<DAGContext[]>([]);
  const [models, setModels] = useState<ModelOption[]>([]);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const currentPageDag = useDagPageContext();
  // Track IME composition state manually for reliable Japanese/Chinese input handling
  const isComposingRef = useRef(false);

  const showPauseButton = isPending || isWorking;

  // Fetch available models
  useEffect(() => {
    async function fetchModels() {
      try {
        const remoteNode = encodeURIComponent(appBarContext.selectedRemoteNode || 'local');
        const response = await fetch(
          `${config.apiURL}/settings/agent/models?remoteNode=${remoteNode}`,
          { headers: getAuthHeaders() }
        );
        if (!response.ok) return;
        const data = await response.json();
        const modelList: ModelOption[] = (data.models || []).map((m: { id: string; name: string }) => ({
          id: m.id,
          name: m.name,
        }));
        setModels(modelList);
        if (data.defaultModelId) {
          setSelectedModel(data.defaultModelId);
        } else if (modelList.length > 0) {
          setSelectedModel(modelList[0]!.id);
        }
      } catch {
        // Models fetch is best-effort
      }
    }
    fetchModels();
  }, [config.apiURL, appBarContext.selectedRemoteNode]);

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
    // Allow sending while working (isWorking=true) - message will be queued
    // Only block during brief isPending state or when disabled
    if (!trimmed || isPending || disabled) {
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

    onSend(
      trimmed,
      allContexts.length > 0 ? allContexts : undefined,
      selectedModel || undefined
    );
    setMessage('');
  }, [message, isPending, disabled, onSend, selectedDags, currentPageDag, selectedModel]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      // Ignore Enter during IME composition (e.g., Japanese input conversion)
      // Check both isComposing and our manual ref for cross-browser compatibility
      if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing && !isComposingRef.current) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend]
  );

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
  }, []);

  const handleCompositionEnd = useCallback(() => {
    isComposingRef.current = false;
  }, []);

  return (
    <div className="p-2 border-t border-border bg-background">
      {/* DAG Picker with chips */}
      <DAGPicker
        selectedDags={selectedDags}
        onChange={setSelectedDags}
        currentPageDag={currentPageDag}
        disabled={disabled || showPauseButton}
      />

      {/* Model selector row */}
      {models.length > 0 && (
        <div className="mb-1.5">
          <Select value={selectedModel} onValueChange={setSelectedModel}>
            <SelectTrigger className="h-7 text-xs w-auto min-w-[140px] max-w-[200px]">
              <SelectValue placeholder="Select model" />
            </SelectTrigger>
            <SelectContent>
              {models.map((m) => (
                <SelectItem key={m.id} value={m.id} className="text-xs">
                  {m.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      )}

      {/* Input row */}
      <div className="flex items-end gap-2">
        <textarea
          value={message}
          onChange={(e) => setMessage(e.target.value)}
          onKeyDown={handleKeyDown}
          onCompositionStart={handleCompositionStart}
          onCompositionEnd={handleCompositionEnd}
          placeholder={placeholder}
          disabled={disabled}
          rows={1}
          className={cn(
            'flex-1 resize-none rounded-md border border-input bg-background px-3 py-2 text-sm',
            'placeholder:text-muted-foreground focus-visible:outline-none focus-visible:border-ring',
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
            variant="primary"
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
