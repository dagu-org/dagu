import { cn } from '@/lib/utils';
import * as React from 'react';

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string;
  children: React.ReactNode;
}

function Tabs({ className, children, ...props }: Omit<TabsProps, 'value'>) {
  return (
    <div
      className={cn('inline-flex items-center gap-1', className)}
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
        'inline-flex items-center justify-center whitespace-nowrap px-3 py-1.5 text-sm font-medium',
        'transition-all duration-200 ease-in-out focus-visible:outline-none',
        'disabled:pointer-events-none disabled:opacity-50',
        isActive
          ? 'text-primary font-semibold border-b-2 border-primary [&_svg]:text-primary'
          : 'text-foreground hover:text-foreground/80 border-b-2 border-transparent',
        className
      )}
      {...props}
    >
      {children}
    </button>
  );
}

export { Tab, Tabs };
