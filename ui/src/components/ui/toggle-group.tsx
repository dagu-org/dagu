import { cn } from '@/lib/utils';
import React, { useCallback, useRef } from 'react';

type ToggleGroupProps = {
  value: string;
  onChange: (value: string) => void;
  children: React.ReactNode;
  className?: string;
  'aria-label'?: string;
};

export function ToggleGroup({
  children,
  className,
  'aria-label': ariaLabel,
}: Omit<ToggleGroupProps, 'value' | 'onChange'>): React.ReactElement {
  const groupRef = useRef<HTMLDivElement>(null);

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (!groupRef.current) return;

    const buttons = Array.from(
      groupRef.current.querySelectorAll('button:not([disabled])')
    ) as HTMLButtonElement[];

    if (buttons.length === 0) return;

    const currentIndex = buttons.findIndex(
      (btn) => btn === document.activeElement
    );

    if (currentIndex === -1) return;

    let nextIndex: number | null = null;

    switch (event.key) {
      case 'ArrowRight':
      case 'ArrowDown':
        nextIndex = (currentIndex + 1) % buttons.length;
        break;
      case 'ArrowLeft':
      case 'ArrowUp':
        nextIndex = (currentIndex - 1 + buttons.length) % buttons.length;
        break;
      case 'Home':
        nextIndex = 0;
        break;
      case 'End':
        nextIndex = buttons.length - 1;
        break;
    }

    if (nextIndex !== null) {
      const nextButton = buttons[nextIndex];
      if (nextButton) {
        event.preventDefault();
        nextButton.focus();
      }
    }
  }, []);

  return (
    <div
      ref={groupRef}
      className={cn(
        'inline-flex rounded-lg border border-border overflow-hidden bg-muted/50 p-1 gap-1',
        className
      )}
      role="group"
      aria-label={ariaLabel}
      onKeyDown={handleKeyDown}
    >
      {children}
    </div>
  );
}

type ToggleButtonProps = {
  value: string;
  groupValue?: string;
  onClick?: () => void;
  children: React.ReactNode;
  className?: string;
  'aria-label'?: string;
  position?: 'first' | 'middle' | 'last' | 'single';
};

export function ToggleButton({
  value,
  groupValue,
  onClick,
  children,
  className,
  'aria-label': ariaLabel,
}: ToggleButtonProps): React.ReactElement {
  const isSelected = groupValue === value;

  return (
    <button
      type="button"
      className={cn(
        'inline-flex items-center justify-center h-8 px-4 text-xs font-bold transition-all rounded-md cursor-pointer',
        isSelected
          ? 'bg-primary text-primary-foreground shadow-sm scale-100 dark:bg-transparent dark:border dark:border-primary dark:text-primary'
          : 'text-muted-foreground hover:bg-muted hover:text-foreground active:scale-95',
        className
      )}
      onClick={onClick}
      aria-pressed={isSelected}
      aria-label={ariaLabel}
    >
      {children}
    </button>
  );
}
