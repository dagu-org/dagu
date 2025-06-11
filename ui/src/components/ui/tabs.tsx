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
        'inline-flex items-center gap-1 rounded-lg bg-white dark:bg-zinc-900 p-1 text-muted-foreground border border-border',
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
        'inline-flex items-center justify-center whitespace-nowrap rounded-md px-3 py-2 text-sm font-medium',
        'transition-all duration-200 ease-in-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2',
        'disabled:pointer-events-none disabled:opacity-50 border',
        isActive
          ? 'bg-blue-100 dark:bg-blue-900/50 text-blue-700 dark:text-blue-300 border-blue-200 dark:border-blue-600 font-semibold'
          : 'text-muted-foreground hover:text-foreground hover:bg-muted/50 border-transparent',
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export { Tab, Tabs };
