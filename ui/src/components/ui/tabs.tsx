import { cn } from '@/lib/utils';
import * as React from 'react';

interface TabsProps extends React.HTMLAttributes<HTMLDivElement> {
  value: string;
  children: React.ReactNode;
}

function Tabs({ className, children, ...props }: Omit<TabsProps, 'value'>) {
  return (
    <div className={cn('inline-flex items-center gap-1', className)} {...props}>
      {children}
    </div>
  );
}

interface TabProps extends React.HTMLAttributes<HTMLElement> {
  value: string;
  isActive?: boolean;
  asChild?: boolean;
}

function Tab({
  className,
  isActive,
  children,
  asChild = false,
  ...props
}: Omit<TabProps, 'value'>) {
  const classes = cn(
    'inline-flex items-center justify-center whitespace-nowrap px-3 py-1.5 text-sm font-semibold',
    'transition-colors duration-200 ease-in-out focus-visible:outline-none',
    'disabled:pointer-events-none disabled:opacity-50',
    isActive
      ? 'text-primary border-b-2 border-primary [&_svg]:text-primary'
      : 'text-muted-foreground hover:text-foreground border-b-2 border-transparent',
    className
  );

  if (asChild) {
    return (
      <span className={classes} {...props}>
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
