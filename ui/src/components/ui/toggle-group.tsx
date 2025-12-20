import { cn } from '@/lib/utils';
import React from 'react';

type ToggleGroupProps = {
  value: string;
  onChange: (value: string) => void;
  children: React.ReactNode;
  className?: string;
  'aria-label'?: string;
};

export const ToggleGroup = ({
  children,
  className,
  'aria-label': ariaLabel,
}: Omit<ToggleGroupProps, 'value' | 'onChange'>) => {
  return (
    <div
      className={cn(
        'inline-flex rounded border border-[rgb(var(--btn-secondary-border))] overflow-hidden',
        'bg-gradient-to-b from-[rgb(var(--btn-secondary-top))] to-[rgb(var(--btn-secondary-bottom))]',
        'border-b-[3px] border-b-[rgb(var(--btn-secondary-platform))]',
        className
      )}
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
    'rounded-l rounded-r-none': position === 'first',
    'rounded-r rounded-l-none': position === 'last',
    'rounded-none': position === 'middle',
    'rounded': position === 'single',
  });

  return (
    <button
      type="button"
      className={cn(
        'inline-flex items-center justify-center h-8 px-4 text-sm font-semibold transition-all focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring cursor-pointer',
        borderRadiusClasses,
        isSelected
          ? 'bg-gradient-to-b from-[rgb(var(--btn-primary-top))] to-[rgb(var(--btn-primary-bottom))] text-white [&_svg]:text-white'
          : 'text-[#3d3833] hover:bg-white/50 [&_svg]:text-[#3d3833]',
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
