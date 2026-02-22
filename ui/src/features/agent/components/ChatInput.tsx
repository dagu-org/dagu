import { useState, useCallback, useEffect, useRef, KeyboardEvent, ChangeEvent } from 'react';
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
import { DAGContext } from '../types';
import { DAGPicker } from './DAGPicker';
import { SkillPicker, type SkillRef, type SkillPickerHandle } from './SkillPicker';
import { useDagPageContext } from '../hooks/useDagPageContext';
import { useAvailableModels } from '../hooks/useAvailableModels';
import { useAvailableSouls } from '../hooks/useAvailableSouls';

interface ChatInputProps {
  onSend: (message: string, dagContexts?: DAGContext[], model?: string, soulId?: string) => void;
  onCancel?: () => void;
  isWorking: boolean;
  disabled?: boolean;
  placeholder?: string;
  initialValue?: string | null;
  hasActiveSession?: boolean;
}

export function ChatInput({
  onSend,
  onCancel,
  isWorking,
  disabled,
  placeholder = 'Type a message...',
  initialValue,
  hasActiveSession,
}: ChatInputProps) {
  const [message, setMessage] = useState('');
  const [isPending, setIsPending] = useState(false);
  const [selectedDags, setSelectedDags] = useState<DAGContext[]>([]);
  const { models, selectedModel, setSelectedModel } = useAvailableModels();
  const { souls, selectedSoul, setSelectedSoul } = useAvailableSouls();
  const currentPageDag = useDagPageContext();
  // Track IME composition state manually for reliable Japanese/Chinese input handling
  const isComposingRef = useRef(false);

  // Skill picker state
  const [selectedSkills, setSelectedSkills] = useState<SkillRef[]>([]);
  const [slashMenuOpen, setSlashMenuOpen] = useState(false);
  const [slashQuery, setSlashQuery] = useState('');
  const [slashStart, setSlashStart] = useState(-1);
  const skillPickerRef = useRef<SkillPickerHandle>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  const showPauseButton = isPending || isWorking;

  // Pre-fill textarea with initial value (e.g., from setup wizard)
  useEffect(() => {
    if (initialValue) {
      setMessage(initialValue);
      requestAnimationFrame(() => {
        const el = textareaRef.current;
        if (el) {
          el.style.height = 'auto';
          el.style.height = `${Math.min(el.scrollHeight, 120)}px`;
        }
      });
    }
  }, [initialValue]);

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

    // Prepend skill instructions if skills are selected.
    let finalMessage = trimmed;
    if (selectedSkills.length > 0) {
      const prefix = selectedSkills.map((s) => `[Skill: ${s.id}]`).join(' ');
      finalMessage = `${prefix}\n${trimmed}`;
    }

    const soulValue = selectedSoul && selectedSoul !== '__default__' ? selectedSoul : undefined;
    onSend(
      finalMessage,
      allContexts.length > 0 ? allContexts : undefined,
      selectedModel || undefined,
      soulValue
    );
    setMessage('');
    setSelectedSkills([]);
  }, [message, isPending, disabled, onSend, selectedDags, currentPageDag, selectedModel, selectedSkills, selectedSoul]);

  const handleChange = useCallback(
    (e: ChangeEvent<HTMLTextAreaElement>) => {
      const val = e.target.value;
      const pos = e.target.selectionStart ?? val.length;
      setMessage(val);

      // Detect '/' at start or after whitespace to open skill menu.
      if (pos > 0 && val[pos - 1] === '/') {
        const charBefore = pos > 1 ? val[pos - 2] : undefined;
        if (charBefore === undefined || charBefore === ' ' || charBefore === '\n') {
          setSlashStart(pos - 1);
          setSlashMenuOpen(true);
          setSlashQuery('');
          return;
        }
      }

      // Update filter query while slash menu is open.
      if (slashMenuOpen && slashStart >= 0) {
        const query = val.substring(slashStart + 1, pos);
        // Close menu if user typed a space (done filtering).
        if (query.includes(' ')) {
          setSlashMenuOpen(false);
          setSlashStart(-1);
        } else {
          setSlashQuery(query);
        }
      }
    },
    [slashMenuOpen, slashStart]
  );

  const handleSkillSelect = useCallback(
    (skill: SkillRef) => {
      // Add to selected skills (no duplicates).
      if (!selectedSkills.find((s) => s.id === skill.id)) {
        setSelectedSkills((prev) => [...prev, skill]);
      }
      // Remove /text from textarea.
      const before = message.substring(0, slashStart);
      const after = message.substring(slashStart + 1 + slashQuery.length);
      setMessage(before + after);
      // Close dropdown.
      setSlashMenuOpen(false);
      setSlashStart(-1);
      setSlashQuery('');
      // Re-focus textarea.
      textareaRef.current?.focus();
    },
    [selectedSkills, message, slashStart, slashQuery]
  );

  const handleSkillRemove = useCallback((id: string) => {
    setSelectedSkills((prev) => prev.filter((s) => s.id !== id));
  }, []);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      // Let skill picker handle keys when open.
      if (slashMenuOpen && skillPickerRef.current) {
        // Backspace past the '/' closes the menu.
        if (e.key === 'Backspace' && slashQuery === '') {
          setSlashMenuOpen(false);
          setSlashStart(-1);
          return;
        }
        const consumed = skillPickerRef.current.handleKeyDown(e);
        if (consumed) return;
      }

      // Ignore Enter during IME composition (e.g., Japanese input conversion)
      // Check both isComposing and our manual ref for cross-browser compatibility
      if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing && !isComposingRef.current) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend, slashMenuOpen, slashQuery]
  );

  const handleCompositionStart = useCallback(() => {
    isComposingRef.current = true;
  }, []);

  const handleCompositionEnd = useCallback(() => {
    isComposingRef.current = false;
  }, []);

  return (
    <div className="p-2 border-t border-border bg-background relative">
      {/* DAG Picker with chips */}
      <DAGPicker
        selectedDags={selectedDags}
        onChange={setSelectedDags}
        currentPageDag={currentPageDag}
        disabled={disabled || showPauseButton}
      />

      {/* Skill chips and dropdown */}
      <SkillPicker
        ref={skillPickerRef}
        selectedSkills={selectedSkills}
        onSelect={handleSkillSelect}
        onRemove={handleSkillRemove}
        isOpen={slashMenuOpen}
        onClose={() => {
          setSlashMenuOpen(false);
          setSlashStart(-1);
          setSlashQuery('');
        }}
        filterQuery={slashQuery}
        disabled={disabled || showPauseButton}
      />

      {/* Model & soul selector row */}
      {(models.length > 0 || (souls.length > 0 && !hasActiveSession)) && (
        <div className="mb-1.5 flex items-center gap-2">
          {models.length > 0 && (
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
          )}
          {souls.length > 0 && !hasActiveSession && (
            <Select value={selectedSoul} onValueChange={setSelectedSoul}>
              <SelectTrigger className="h-7 text-xs w-auto min-w-[140px] max-w-[200px]">
                <SelectValue placeholder="default" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__default__" className="text-xs">default</SelectItem>
                {souls.map((s) => (
                  <SelectItem key={s.id} value={s.id} className="text-xs">
                    {s.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          )}
        </div>
      )}

      {/* Input row */}
      <div className="flex items-end gap-2">
        <textarea
          ref={textareaRef}
          autoFocus
          value={message}
          onChange={handleChange}
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
