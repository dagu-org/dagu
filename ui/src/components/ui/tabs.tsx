import { cn } from '@/lib/utils';
import * as React from 'react';

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string;
  children: React.ReactNode;
}

function Tabs({ className, children, ...props }: Omit<TabsProps, 'value'>) {
  return (
    <div
      className={cn(
        'inline-flex items-center gap-0.5 rounded-md bg-surface p-0.5 text-muted-foreground border border-border',
        className
      )}
      {...props}
    >
      {children}
    </div>
  );
}

interface TabProps extends React.HTMLAttributes<HTMLButtonElement> {
  value: string;
  isActive?: boolean;
}

function Tab({
  className,
  isActive,
  children,
  ...props
}: Omit<TabProps, 'value'>) {
  return (
    <button
      className={cn(
        'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-1.5 text-sm font-medium',
        'transition-all duration-200 ease-in-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
        'disabled:pointer-events-none disabled:opacity-50',
        isActive
          ? 'bg-accent-surface text-foreground font-medium'
          : 'text-muted-foreground hover:text-foreground hover:bg-accent-surface bg-transparent',
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export { Tab, Tabs };
