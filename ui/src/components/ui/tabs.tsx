import { cn } from '@/lib/utils';
import * as React from 'react';

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string;
  children: React.ReactNode;
}

// GCP-Style Tabs Container
function Tabs({ className, children, ...props }: Omit<TabsProps, 'value'>) {
  return (
    <div className={cn('inline-flex items-center border-b border-border', className)} {...props}>
      {children}
    </div>
  );
}

interface TabProps extends React.HTMLAttributes<HTMLElement> {
  value: string;
  isActive?: boolean;
  asChild?: boolean;
}

// GCP-Style Tab - Clean with Bottom Border Indicator
function Tab({
  className,
  isActive,
  children,
  asChild = false,
  ...props
}: Omit<TabProps, 'value'>) {
  const classes = cn(
    'inline-flex items-center justify-center whitespace-nowrap h-12 px-4 text-sm font-medium relative',
    'transition-all duration-150 ease-in-out focus-visible:outline-none',
    'disabled:pointer-events-none disabled:opacity-50',
    'border-b-2',
    isActive
      ? 'text-foreground border-primary [&_svg]:text-primary'
      : 'text-text-secondary hover:text-foreground hover:bg-muted border-transparent',
    className
  );

  if (asChild) {
    return (
      <span className={classes} role="button" tabIndex={0} {...props}>
        {children}
      </span>
    );
  }

  return (
    <button className={classes} {...props}>
      {children}
    </button>
  );
}

export { Tab, Tabs };
