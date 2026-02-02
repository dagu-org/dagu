import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import React, { ReactElement, forwardRef } from 'react';

interface ActionButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  children: React.ReactNode;
  label?: boolean;
  icon: ReactElement;
  // disabled and onClick are covered by ButtonHTMLAttributes
}

const ActionButton = forwardRef<HTMLButtonElement, ActionButtonProps>(
  ({ label, children, icon, className, disabled, onClick, ...props }, ref) => {
    // Common base styles for all action buttons
    const baseStyles =
      'inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded text-sm font-semibold transition-all disabled:pointer-events-none disabled:opacity-50 outline-none cursor-pointer focus-visible:ring-1 focus-visible:ring-ring';

    return label ? (
      // Labeled button (with text)
      <Button
        ref={ref}
        size="sm"
        disabled={disabled}
        onClick={onClick}
        className={cn(baseStyles, 'btn-3d-secondary', className)}
        {...props}
      >
        <span className="h-4 w-4">{icon}</span>
        {children}
      </Button>
    ) : (
      // Icon-only button
      <Button
        ref={ref}
        size="icon"
        disabled={disabled}
        onClick={onClick}
        className={cn(baseStyles, 'h-8 w-8 btn-3d-secondary', className)}
        {...props}
      >
        <span className="h-4 w-4">{icon}</span>
        <span className="sr-only">{children}</span>
      </Button>
    );
  }
);

ActionButton.displayName = 'ActionButton';

export default ActionButton;
