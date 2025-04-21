import React from 'react';
import { cn } from '@/lib/utils';

type ToggleGroupProps = {
  value: string;
  onChange: (value: string) => void;
  children: React.ReactNode;
  className?: string;
  'aria-label'?: string;
};

export const ToggleGroup = ({
  value,
  onChange,
  children,
  className,
  'aria-label': ariaLabel,
}: ToggleGroupProps) => {
  return (
    <div
      className={cn('inline-flex rounded-md border bg-background', className)}
      role="group"
      aria-label={ariaLabel}
    >
      {children}
    </div>
  );
};

type ToggleButtonProps = {
  value: string;
  groupValue?: string;
  onClick?: () => void;
  children: React.ReactNode;
  className?: string;
  'aria-label'?: string;
  position?: 'first' | 'middle' | 'last' | 'single';
};

export const ToggleButton = ({
  value,
  groupValue,
  onClick,
  children,
  className,
  'aria-label': ariaLabel,
  position = 'middle',
}: ToggleButtonProps) => {
  const isSelected = groupValue === value;

  // Apply different border radius based on position
  const borderRadiusClasses = cn({
    'rounded-l-md rounded-r-none': position === 'first',
    'rounded-r-md rounded-l-none': position === 'last',
    'rounded-none': position === 'middle',
    'rounded-md': position === 'single',
  });

  return (
    <button
      type="button"
      className={cn(
        'inline-flex items-center justify-center px-3 py-2 text-sm font-medium ring-offset-background transition-colors hover:bg-muted hover:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 cursor-pointer',
        borderRadiusClasses,
        isSelected
          ? 'bg-primary text-primary-foreground'
          : 'text-muted-foreground',
        className
      )}
      onClick={onClick}
      aria-pressed={isSelected}
      aria-label={ariaLabel}
    >
      {children}
    </button>
  );
};
