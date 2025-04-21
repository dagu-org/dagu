import * as React from 'react';
import { cn } from '@/lib/utils';

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string;
  children: React.ReactNode;
}

function Tabs({ className, value, children, ...props }: TabsProps) {
  return (
    <div
      className={cn(
        'inline-flex items-center justify-center gap-1 rounded-lg bg-white p-1 text-muted-foreground',
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

function Tab({ className, value, isActive, children, ...props }: TabProps) {
  return (
    <button
      className={cn(
        'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium',
        'transition-all duration-200 ease-in-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        'disabled:pointer-events-none disabled:opacity-50',
        isActive
          ? 'text-primary font-medium'
          : 'text-muted-foreground hover:text-primary hover:bg-primary/5',
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export { Tabs, Tab };
