import * as React from 'react';

import { cn } from '@/lib/utils';

// GCP-Style Input - Clean Focus States
const Input = React.forwardRef<HTMLInputElement, React.ComponentProps<'input'>>(
  ({ className, type, ...props }, ref) => {
    return (
      <input
        ref={ref}
        type={type}
        data-slot="input"
        className={cn(
          'flex h-7 w-full min-w-0 rounded-md border border-border bg-background px-3 py-1 text-sm transition-colors',
          'placeholder:text-muted-foreground',
          'file:border-0 file:bg-transparent file:text-sm file:font-medium file:text-foreground',
          'focus-visible:outline-none focus-visible:border-ring',
          'disabled:cursor-not-allowed disabled:opacity-50 disabled:bg-muted',
          'aria-invalid:border-destructive aria-invalid:ring-2 aria-invalid:ring-destructive/20',
          className
        )}
        {...props}
      />
    );
  }
);

Input.displayName = 'Input';

export { Input };
